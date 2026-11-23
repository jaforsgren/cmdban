package ui

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/azuredevops"
	"cmdban/internal/checkboxdraft"
	"cmdban/internal/task"
)

// Checkbox chords reuse the delete prompt opened by x: a digit toggles that
// checkbox instead of answering it, so x alone can never change a card and
// deleting still needs an explicit y.
const (
	inputActionDelete = "delete"

	checkboxPrefixKey   = "x"
	checkboxSaveKey     = "ctrl+s"
	checkboxDiscardKey  = "u"
	checkboxToggleChord = checkboxPrefixKey + " 1-9"
	checkboxUndoChord   = checkboxPrefixKey + " " + checkboxDiscardKey

	firstCheckboxNumber = 1
	lastCheckboxNumber  = 9

	unsavedCheckboxMarker = "*"
)

type checkboxesSavedMsg struct {
	org        string
	workItemID int
	changes    []task.CheckboxChange
}

func loadCheckboxDrafts() (*checkboxdraft.Store, error) {
	path := checkboxdraft.DefaultStorePath()
	store, err := checkboxdraft.Load(path)
	if err != nil {
		return checkboxdraft.New(path), err
	}
	return store, nil
}

func checkboxNumberForKey(keyName string) (int, bool) {
	number, err := strconv.Atoi(keyName)
	if err != nil || number < firstCheckboxNumber || number > lastCheckboxNumber {
		return 0, false
	}
	return number, true
}

// handleCheckboxChord completes an x chord from the delete prompt. It reports
// false for keys that should answer the prompt instead.
func (m Model) handleCheckboxChord(keyName string) (tea.Model, tea.Cmd, bool) {
	if number, ok := checkboxNumberForKey(keyName); ok {
		m.mode = m.inputReturnMode()
		updated, cmd := m.toggleCheckbox(number)
		return updated, cmd, true
	}
	if !m.config.IsADOBoard() {
		return m, nil, false
	}
	if keyName == checkboxDiscardKey {
		m.mode = m.inputReturnMode()
		updated, cmd := m.discardCheckboxDraft()
		return updated, cmd, true
	}
	return m, nil, false
}

// handlesCheckboxShortcuts reports whether the current screen acts on a
// selected ADO card, where the checkbox save and discard keys apply.
func (m Model) handlesCheckboxShortcuts() bool {
	if !m.config.IsADOBoard() {
		return false
	}
	return m.mode == ModeNormal || m.mode == ModeView || m.mode == ModeFocus
}

func (m Model) toggleCheckbox(number int) (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}
	if t.ADOItemID != 0 {
		return m.toggleADOCheckbox(t, number)
	}
	path := t.FilePath
	return m, func() tea.Msg {
		if err := task.ToggleCheckboxInFile(path, number); err != nil {
			return errMsg{err}
		}
		return taskSavedMsg{}
	}
}

// toggleADOCheckbox only updates the local draft; nothing reaches Azure
// DevOps until the draft is saved.
func (m Model) toggleADOCheckbox(t *task.Task, number int) (tea.Model, tea.Cmd) {
	boxes := t.Checkboxes()
	if number > len(boxes) {
		m.message = fmt.Sprintf("Card has no checkbox %d (it has %d)", number, len(boxes))
		return m, nil
	}
	box := boxes[number-1]
	checked := !box.Checked
	t.SetCheckbox(number, checked)
	org := m.adoOrg()
	m.checkboxDrafts.Toggle(org, t.ADOItemID, task.CheckboxChange{Number: number, Label: box.Label, Checked: checked})
	if err := m.checkboxDrafts.Save(); err != nil {
		m.message = "Checkbox changed but not stored: " + err.Error()
		m.logs.Error(m.message)
		return m, nil
	}
	m.message = fmt.Sprintf("%d unsaved checkbox change(s) on #%d (%s saves to ADO, %s discards)",
		m.unsavedCheckboxCount(t), t.ADOItemID, checkboxSaveKey, checkboxUndoChord)
	return m, nil
}

func (m Model) saveCheckboxDraft() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil || t.ADOItemID == 0 {
		return m, nil
	}
	org := m.adoOrg()
	changes := m.checkboxDrafts.Changes(org, t.ADOItemID)
	if len(changes) == 0 {
		m.message = "No unsaved checkbox changes on this card"
		return m, nil
	}
	adoCfg := m.config.ActiveBoard().AzureDevOps
	workItemID := t.ADOItemID
	m.message = fmt.Sprintf("Saving checkboxes on #%d...", workItemID)
	return m, func() tea.Msg {
		token, ok := m.patStore.Get(adoCfg.PAT)
		if !ok {
			return errMsg{fmt.Errorf("PAT %q not found — run :pats to add it", adoCfg.PAT)}
		}
		if err := azuredevops.SaveCheckboxChanges(adoCfg, token, workItemID, changes); err != nil {
			return errMsg{err}
		}
		return checkboxesSavedMsg{org: org, workItemID: workItemID, changes: changes}
	}
}

func (m Model) handleCheckboxesSaved(msg checkboxesSavedMsg) (tea.Model, tea.Cmd) {
	m.checkboxDrafts.Remove(msg.org, msg.workItemID, msg.changes)
	m.saveCheckboxDrafts()
	m.message = fmt.Sprintf("Saved checkboxes on #%d", msg.workItemID)
	return m, m.loadTasks
}

func (m Model) discardCheckboxDraft() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil || t.ADOItemID == 0 {
		return m, nil
	}
	org := m.adoOrg()
	if len(m.checkboxDrafts.Changes(org, t.ADOItemID)) == 0 {
		m.message = "No unsaved checkbox changes on this card"
		return m, nil
	}
	m.checkboxDrafts.Replace(org, t.ADOItemID, nil)
	m.saveCheckboxDrafts()
	m.message = fmt.Sprintf("Discarded checkbox changes on #%d", t.ADOItemID)
	return m, m.loadTasks
}

// applyCheckboxDrafts re-applies unsaved toggles to freshly synced tasks.
// Changes ADO already matches are finished and dropped; a change whose label
// no longer matches stays pending so saving reports the conflict.
func (m Model) applyCheckboxDrafts(tasks []*task.Task) {
	org := m.adoOrg()
	changed := false
	for _, t := range tasks {
		changes := m.checkboxDrafts.Changes(org, t.ADOItemID)
		if len(changes) == 0 {
			continue
		}
		boxes := t.Checkboxes()
		pending := make([]task.CheckboxChange, 0, len(changes))
		for _, change := range changes {
			if change.Number > len(boxes) || boxes[change.Number-1].Label != change.Label {
				pending = append(pending, change)
				continue
			}
			if boxes[change.Number-1].Checked == change.Checked {
				changed = true
				continue
			}
			t.SetCheckbox(change.Number, change.Checked)
			pending = append(pending, change)
		}
		m.checkboxDrafts.Replace(org, t.ADOItemID, pending)
	}
	if changed {
		m.saveCheckboxDrafts()
	}
}

// saveCheckboxDrafts logs failures: the drafts are still held in memory and
// the next successful write stores them.
func (m Model) saveCheckboxDrafts() {
	if err := m.checkboxDrafts.Save(); err != nil {
		m.logs.Error("Error saving checkbox drafts: " + err.Error())
	}
}

func (m Model) unsavedCheckboxCount(t *task.Task) int {
	if t.ADOItemID == 0 || !m.config.IsADOBoard() {
		return 0
	}
	return len(m.checkboxDrafts.Changes(m.adoOrg(), t.ADOItemID))
}

// deleteConfirmPrompt also lists the checkbox chords the prompt completes.
func (m Model) deleteConfirmPrompt() string {
	prompt := "Delete task? (y/n)"
	t := m.selectedTask()
	if t == nil {
		return prompt
	}
	if t.CheckboxTotal > 0 {
		prompt += fmt.Sprintf("  ·  %d-%d: toggle checkbox", firstCheckboxNumber, min(t.CheckboxTotal, lastCheckboxNumber))
	}
	if unsaved := m.unsavedCheckboxCount(t); unsaved > 0 {
		prompt += fmt.Sprintf("  ·  %s: discard %d unsaved", checkboxDiscardKey, unsaved)
	}
	return prompt
}

// numberedADOFields numbers checkboxes across both fields, matching
// Task.Checkboxes.
func numberedADOFields(t *task.Task) (description, reproSteps string) {
	_, descriptionTotal := task.CountCheckboxes(t.Description)
	return task.NumberCheckboxes(t.Description, firstCheckboxNumber),
		task.NumberCheckboxes(t.ReproSteps, firstCheckboxNumber+descriptionTotal)
}
