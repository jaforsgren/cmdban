package ui

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/azuredevops"
	"cmdban/internal/config"
)

const (
	inputActionAddWiki         = "add-wiki"
	inputActionRemoveWiki      = "remove-wiki"
	inputActionPublishWikiPage = "publish-wiki-page"

	wikiURLPlaceholder = "Wiki page URL, e.g. https://dev.azure.com/org/project/_wiki/wikis/Wiki/12/Page"
	wikiTempFilePrefix = "cmdban-wiki-*.md"
	wikiSelectionJump  = 5
	wikiScrollJump     = 10
)

var errNoPAT = errors.New("no personal access token stored — run :pats and add one first")

// wikiPageKey identifies a page across the configured wikis. wiki is the
// wiki's identity rather than its position, so removing a wiki doesn't
// mix up the state of the ones after it.
type wikiPageKey struct {
	wiki string
	path string
}

func wikiID(w config.Wiki) string {
	return strings.ToLower(fmt.Sprintf("%s/%s/%s/%d", w.Org, w.Project, w.Wiki, w.PageID))
}

type wikiTree struct {
	root    *azuredevops.WikiPage
	loading bool
	err     error
}

// wikiRow is one visible line of the wiki tree. guides holds, for every
// level from the top-level pages down to the row itself, whether that
// level's page is the last of its siblings; it decides the tree lines drawn
// in front of the title.
type wikiRow struct {
	wikiIndex int
	key       wikiPageKey
	page      *azuredevops.WikiPage
	guides    []bool
}

func (r wikiRow) depth() int {
	return len(r.guides)
}

func (r wikiRow) hasSubPages() bool {
	return r.page != nil && len(r.page.SubPages) > 0
}

// wikiEdit is a page opened in $EDITOR: its original content and ETag, and
// once the editor closes, what was written.
type wikiEdit struct {
	wiki     config.Wiki
	key      wikiPageKey
	tempPath string
	original azuredevops.WikiPageContent
	edited   string
}

type wikiState struct {
	trees        map[string]wikiTree
	expanded     map[wikiPageKey]bool
	pages        map[wikiPageKey]azuredevops.WikiPageContent
	pageErrors   map[wikiPageKey]error
	loadingPages map[wikiPageKey]bool
	selected     int
	preview      bool
	scroll       int
	lastSync     time.Time
	// returnMode is the wiki screen a prompt or confirmation was opened
	// from.
	returnMode  Mode
	pendingEdit wikiEdit
	// leader is the space-led key sequence typed so far.
	leader          string
	picker          wikiPicker
	watch           bool
	watchGeneration int
}

func newWikiState() wikiState {
	return wikiState{
		trees:        make(map[string]wikiTree),
		expanded:     make(map[wikiPageKey]bool),
		pages:        make(map[wikiPageKey]azuredevops.WikiPageContent),
		pageErrors:   make(map[wikiPageKey]error),
		loadingPages: make(map[wikiPageKey]bool),
		returnMode:   ModeWiki,
	}
}

type wikiTreeLoadedMsg struct {
	wiki string
	root azuredevops.WikiPage
	err  error
	// background marks a watch refresh, which leaves the footer message
	// alone.
	background bool
}

type wikiPageLoadedMsg struct {
	key     wikiPageKey
	content azuredevops.WikiPageContent
	err     error
}

type wikiAddedMsg struct {
	wiki config.Wiki
	root azuredevops.WikiPage
}

type wikiEditReadyMsg struct {
	edit wikiEdit
}

type wikiEditorFinishedMsg struct {
	edit    wikiEdit
	execErr error
}

type wikiPagePublishedMsg struct {
	key wikiPageKey
}

func isWikiMode(mode Mode) bool {
	return mode == ModeWiki || mode == ModeWikiView || mode == ModeWikiPicker
}

func isWikiInputAction(action string) bool {
	return action == inputActionAddWiki || action == inputActionRemoveWiki || action == inputActionPublishWikiPage
}

// inputReturnMode is where cancelling a prompt or confirmation lands: the
// wiki or task screen it came from, or the board.
func (m Model) inputReturnMode() Mode {
	if isWikiInputAction(m.inputAction) {
		return m.wiki.returnMode
	}
	if m.inputAction == inputActionDelete {
		return m.deleteReturnMode
	}
	if isBulkAction(m.inputAction) {
		return ModeVisual
	}
	return ModeNormal
}

// lookupPAT resolves a stored PAT name to its token.
func (m Model) lookupPAT(name string) (string, error) {
	token, ok := m.patStore.Get(name)
	if !ok {
		return "", fmt.Errorf("PAT %q not found — run :pats to add it", name)
	}
	return token, nil
}

// openWiki switches to wiki mode, syncing the wikis the first time.
func (m Model) openWiki() (tea.Model, tea.Cmd) {
	m.mode = ModeWiki
	m.message = ""
	if m.wiki.lastSync.IsZero() {
		return m.syncWikis()
	}
	return m, nil
}

// syncWikis refetches every wiki's page tree and drops cached page content,
// keeping which pages are expanded.
func (m Model) syncWikis() (tea.Model, tea.Cmd) {
	if len(m.config.Wikis) == 0 {
		m.message = "No wikis yet — press a to add one"
		return m, nil
	}
	clear(m.wiki.pages)
	clear(m.wiki.pageErrors)
	clear(m.wiki.loadingPages)
	cmds := make([]tea.Cmd, len(m.config.Wikis))
	for i, w := range m.config.Wikis {
		tree := m.wiki.trees[wikiID(w)]
		tree.loading = true
		m.wiki.trees[wikiID(w)] = tree
		cmds[i] = m.fetchWikiTree(w, false)
	}
	m.message = "Syncing wikis..."
	return m, tea.Batch(cmds...)
}

func (m Model) fetchWikiTree(w config.Wiki, background bool) tea.Cmd {
	return func() tea.Msg {
		token, err := m.lookupPAT(w.PAT)
		if err != nil {
			return wikiTreeLoadedMsg{wiki: wikiID(w), err: err, background: background}
		}
		root, err := azuredevops.FetchWikiTree(&w, token)
		return wikiTreeLoadedMsg{wiki: wikiID(w), root: root, err: err, background: background}
	}
}

func (m Model) handleWikiTreeLoaded(msg wikiTreeLoadedMsg) (tea.Model, tea.Cmd) {
	tree := wikiTree{err: msg.err}
	if msg.err == nil {
		tree.root = &msg.root
	} else {
		m.logs.Error(msg.err.Error())
	}
	m.wiki.trees[msg.wiki] = tree
	m.wiki.lastSync = time.Now()
	if !msg.background && !m.anyWikiTreeLoading() {
		m.message = ""
	}
	m.clampWikiSelection()
	return m.loadSelectedWikiPage()
}

func (m Model) anyWikiTreeLoading() bool {
	for _, tree := range m.wiki.trees {
		if tree.loading {
			return true
		}
	}
	return false
}

// wikiRows flattens the wikis into the lines shown: each wiki's page, and
// beneath every expanded page its sub pages.
func (m Model) wikiRows() []wikiRow {
	var rows []wikiRow
	for i, w := range m.config.Wikis {
		id := wikiID(w)
		tree := m.wiki.trees[id]
		row := wikiRow{wikiIndex: i, key: wikiPageKey{wiki: id}, page: tree.root}
		if tree.root != nil {
			row.key.path = tree.root.Path
		}
		rows = append(rows, row)
		if row.hasSubPages() && m.wiki.expanded[row.key] {
			rows = m.appendWikiSubRows(rows, row, nil)
		}
	}
	return rows
}

func (m Model) appendWikiSubRows(rows []wikiRow, parent wikiRow, guides []bool) []wikiRow {
	subPages := parent.page.SubPages
	for i := range subPages {
		row := wikiRow{
			wikiIndex: parent.wikiIndex,
			key:       wikiPageKey{wiki: parent.key.wiki, path: subPages[i].Path},
			page:      &subPages[i],
			guides:    append(slices.Clone(guides), i == len(subPages)-1),
		}
		rows = append(rows, row)
		if row.hasSubPages() && m.wiki.expanded[row.key] {
			rows = m.appendWikiSubRows(rows, row, row.guides)
		}
	}
	return rows
}

func (m Model) selectedWikiRow() (wikiRow, bool) {
	rows := m.wikiRows()
	if m.wiki.selected < 0 || m.wiki.selected >= len(rows) {
		return wikiRow{}, false
	}
	return rows[m.wiki.selected], true
}

// selectedWikiPage is the selected row once its wiki has loaded.
func (m Model) selectedWikiPage() (wikiRow, bool) {
	row, ok := m.selectedWikiRow()
	return row, ok && row.page != nil
}

func (m *Model) clampWikiSelection() {
	m.wiki.selected = max(min(m.wiki.selected, len(m.wikiRows())-1), 0)
}

func (m Model) moveWikiSelection(delta int) (tea.Model, tea.Cmd) {
	m.wiki.selected += delta
	m.clampWikiSelection()
	m.wiki.scroll = 0
	return m.loadSelectedWikiPage()
}

// expandWikiPage opens the selected page's sub pages, or steps into the
// first one when it is already open.
func (m Model) expandWikiPage() (tea.Model, tea.Cmd) {
	row, ok := m.selectedWikiPage()
	if !ok || !row.hasSubPages() {
		return m, nil
	}
	if !m.wiki.expanded[row.key] {
		m.wiki.expanded[row.key] = true
		return m, nil
	}
	return m.moveWikiSelection(1)
}

// collapseWikiPage closes the selected page's sub pages, or steps out to
// its parent when there is nothing to close.
func (m Model) collapseWikiPage() (tea.Model, tea.Cmd) {
	row, ok := m.selectedWikiRow()
	if !ok {
		return m, nil
	}
	if row.hasSubPages() && m.wiki.expanded[row.key] {
		delete(m.wiki.expanded, row.key)
		return m, nil
	}
	rows := m.wikiRows()
	for i := m.wiki.selected - 1; i >= 0; i-- {
		if rows[i].wikiIndex == row.wikiIndex && rows[i].depth() < row.depth() {
			return m.moveWikiSelection(i - m.wiki.selected)
		}
	}
	return m, nil
}

// toggleWikiFold opens or folds the selected page's sub pages, keeping the
// selection where it is.
func (m Model) toggleWikiFold() (tea.Model, tea.Cmd) {
	row, ok := m.selectedWikiPage()
	if !ok || !row.hasSubPages() {
		return m, nil
	}
	if m.wiki.expanded[row.key] {
		delete(m.wiki.expanded, row.key)
	} else {
		m.wiki.expanded[row.key] = true
	}
	return m, nil
}

// toggleWikiFoldAll opens the selected page and every page beneath it, or
// folds them all when they are already all open.
func (m Model) toggleWikiFoldAll() (tea.Model, tea.Cmd) {
	row, ok := m.selectedWikiPage()
	if !ok || !row.hasSubPages() {
		return m, nil
	}
	keys := wikiParentKeys(row.key.wiki, *row.page)
	allExpanded := !slices.ContainsFunc(keys, func(k wikiPageKey) bool { return !m.wiki.expanded[k] })
	for _, k := range keys {
		if allExpanded {
			delete(m.wiki.expanded, k)
		} else {
			m.wiki.expanded[k] = true
		}
	}
	return m, nil
}

// wikiParentKeys lists page and every page beneath it that has sub pages.
func wikiParentKeys(wiki string, page azuredevops.WikiPage) []wikiPageKey {
	if len(page.SubPages) == 0 {
		return nil
	}
	keys := []wikiPageKey{{wiki: wiki, path: page.Path}}
	for _, sub := range page.SubPages {
		keys = append(keys, wikiParentKeys(wiki, sub)...)
	}
	return keys
}

// loadSelectedWikiPage fetches the selected page's content when a preview
// or the page view is showing it and it isn't cached yet.
func (m Model) loadSelectedWikiPage() (tea.Model, tea.Cmd) {
	if !m.wiki.preview && m.mode != ModeWikiView {
		return m, nil
	}
	row, ok := m.selectedWikiPage()
	if !ok {
		return m, nil
	}
	return m, m.ensureWikiPage(row.wikiIndex, row.key, nil)
}

// ensureWikiPage fetches a page's content unless it is cached or already
// on its way.
func (m Model) ensureWikiPage(wikiIndex int, key wikiPageKey, limit chan struct{}) tea.Cmd {
	if _, cached := m.wiki.pages[key]; cached || m.wiki.loadingPages[key] {
		return nil
	}
	return m.fetchWikiPage(wikiIndex, key, limit)
}

// fetchWikiPage fetches a page's content, keeping any cached copy on
// screen until the new one arrives. A non-nil limit caps how many fetches
// run at once when many pages are fetched together.
func (m Model) fetchWikiPage(wikiIndex int, key wikiPageKey, limit chan struct{}) tea.Cmd {
	m.wiki.loadingPages[key] = true
	delete(m.wiki.pageErrors, key)
	w := m.config.Wikis[wikiIndex]
	return func() tea.Msg {
		if limit != nil {
			limit <- struct{}{}
			defer func() { <-limit }()
		}
		token, err := m.lookupPAT(w.PAT)
		if err != nil {
			return wikiPageLoadedMsg{key: key, err: err}
		}
		content, err := azuredevops.FetchWikiPage(&w, token, key.path)
		return wikiPageLoadedMsg{key: key, content: content, err: err}
	}
}

func (m Model) handleWikiPageLoaded(msg wikiPageLoadedMsg) (tea.Model, tea.Cmd) {
	delete(m.wiki.loadingPages, msg.key)
	if msg.err != nil {
		m.wiki.pageErrors[msg.key] = msg.err
		m.logs.Error(msg.err.Error())
		if m.mode == ModeWikiPicker {
			m.refreshWikiPicker()
		}
		return m, nil
	}
	m.wiki.pages[msg.key] = msg.content
	if m.mode == ModeWikiPicker {
		m.refreshWikiPicker()
	}
	return m, nil
}

func (m Model) toggleWikiPreview() (tea.Model, tea.Cmd) {
	m.wiki.preview = !m.wiki.preview
	m.wiki.scroll = 0
	return m.loadSelectedWikiPage()
}

func (m Model) openWikiPageView() (tea.Model, tea.Cmd) {
	if _, ok := m.selectedWikiPage(); !ok {
		return m, nil
	}
	m.mode = ModeWikiView
	m.wiki.scroll = 0
	return m.loadSelectedWikiPage()
}

func (m Model) scrollWikiPage(delta int) (tea.Model, tea.Cmd) {
	m.wiki.scroll = max(m.wiki.scroll+delta, 0)
	return m, nil
}

func (m Model) openWikiPageInBrowser() (tea.Model, tea.Cmd) {
	row, ok := m.selectedWikiPage()
	if !ok {
		return m, nil
	}
	return m.openURL(azuredevops.WikiPageWebURL(&m.config.Wikis[row.wikiIndex], row.key.path))
}

// toggleWikiFavorite marks or unmarks the selected page as a favorite and
// saves the change, leaving the favorites as they were when saving fails.
func (m Model) toggleWikiFavorite() (tea.Model, tea.Cmd) {
	row, ok := m.selectedWikiPage()
	if !ok {
		return m, nil
	}
	favorite, err := m.config.ToggleWikiFavoriteAt(row.wikiIndex, row.key.path)
	if err != nil {
		m.message = "Error updating favorites: " + err.Error()
		return m, nil
	}
	if err := m.config.Save(); err != nil {
		m.config.ToggleWikiFavoriteAt(row.wikiIndex, row.key.path)
		m.message = "Error saving config: " + err.Error()
		m.logs.Error(m.message)
		return m, nil
	}
	if favorite {
		m.message = fmt.Sprintf("Added %q to favorites", row.page.Title())
	} else {
		m.message = fmt.Sprintf("Removed %q from favorites", row.page.Title())
	}
	return m, nil
}

func (m Model) startAddWiki() (tea.Model, tea.Cmd) {
	m.wiki.returnMode = ModeWiki
	return m.startNewTaskInput(inputActionAddWiki, wikiURLPlaceholder)
}

// addWiki checks the URL by fetching the page tree before saving the wiki,
// named after the page it points at.
func (m Model) addWiki(rawURL string) (tea.Model, tea.Cmd) {
	m.mode = ModeWiki
	location, err := azuredevops.ParseWikiURL(rawURL)
	if err != nil {
		m.message = "Invalid wiki URL: " + err.Error()
		return m, nil
	}
	patName := m.config.PATForOrg(location.Org, m.patStore.Names())
	if patName == "" {
		m.message = errNoPAT.Error()
		return m, nil
	}
	w := config.Wiki{
		Name:    location.Wiki,
		Org:     location.Org,
		Project: location.Project,
		Wiki:    location.Wiki,
		PageID:  location.PageID,
		PAT:     patName,
	}
	m.message = "Adding wiki..."
	return m, func() tea.Msg {
		token, err := m.lookupPAT(w.PAT)
		if err != nil {
			return errMsg{err}
		}
		root, err := azuredevops.FetchWikiTree(&w, token)
		if err != nil {
			return errMsg{err}
		}
		if root.Path != azuredevops.WikiRootPath {
			w.Name = root.Title()
		}
		return wikiAddedMsg{wiki: w, root: root}
	}
}

func (m Model) handleWikiAdded(msg wikiAddedMsg) (tea.Model, tea.Cmd) {
	if err := m.config.AddWiki(msg.wiki); err != nil {
		m.message = err.Error()
		return m, nil
	}
	if err := m.config.Save(); err != nil {
		m.message = "Error saving config: " + err.Error()
		m.logs.Error(m.message)
		return m, nil
	}
	m.wiki.trees[wikiID(msg.wiki)] = wikiTree{root: &msg.root}
	m.wiki.lastSync = time.Now()
	m.wiki.selected = slices.IndexFunc(m.wikiRows(), func(r wikiRow) bool { return r.wikiIndex == len(m.config.Wikis)-1 })
	m.message = fmt.Sprintf("Added wiki %q", msg.wiki.Name)
	return m.loadSelectedWikiPage()
}

// confirmRemoveWiki asks before dropping the selected wiki from the config;
// only a wiki's own line can be removed, not the pages beneath it.
func (m Model) confirmRemoveWiki() (tea.Model, tea.Cmd) {
	row, ok := m.selectedWikiRow()
	if !ok {
		return m, nil
	}
	if row.depth() > 0 {
		m.message = "Only a wiki's top line can be removed — select it with h"
		return m, nil
	}
	m.wiki.returnMode = m.mode
	m.mode = ModeConfirm
	m.inputAction = inputActionRemoveWiki
	return m, nil
}

func (m Model) removeSelectedWiki() (tea.Model, tea.Cmd) {
	m.mode = m.wiki.returnMode
	row, ok := m.selectedWikiRow()
	if !ok {
		return m, nil
	}
	removed := m.config.Wikis[row.wikiIndex]
	if err := m.config.RemoveWikiAt(row.wikiIndex); err != nil {
		m.message = "Error removing wiki: " + err.Error()
		return m, nil
	}
	if err := m.config.Save(); err != nil {
		m.message = "Error saving config: " + err.Error()
		m.logs.Error(m.message)
		return m, nil
	}
	delete(m.wiki.trees, wikiID(removed))
	m.clampWikiSelection()
	m.message = fmt.Sprintf("Removed wiki %q (nothing was changed in Azure DevOps)", removed.Name)
	return m.loadSelectedWikiPage()
}

// editWikiPage fetches the page fresh, so the edit starts from the current
// content and ETag, and opens it in $EDITOR.
func (m Model) editWikiPage() (tea.Model, tea.Cmd) {
	row, ok := m.selectedWikiPage()
	if !ok {
		return m, nil
	}
	w := m.config.Wikis[row.wikiIndex]
	m.message = fmt.Sprintf("Opening %q for editing...", row.page.Title())
	return m, func() tea.Msg {
		token, err := m.lookupPAT(w.PAT)
		if err != nil {
			return errMsg{err}
		}
		content, err := azuredevops.FetchWikiPage(&w, token, row.key.path)
		if err != nil {
			return errMsg{err}
		}
		tempPath, err := writeWikiTempFile(content.Content)
		if err != nil {
			return errMsg{fmt.Errorf("writing wiki page to a temp file: %w", err)}
		}
		return wikiEditReadyMsg{edit: wikiEdit{wiki: w, key: row.key, tempPath: tempPath, original: content}}
	}
}

func (m Model) handleWikiEditReady(msg wikiEditReadyMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	return m, tea.ExecProcess(editorCommand(msg.edit.tempPath), func(err error) tea.Msg {
		return wikiEditorFinishedMsg{edit: msg.edit, execErr: err}
	})
}

// handleWikiEditorFinished asks before publishing a changed page, since the
// wiki is shared with everyone in the project.
func (m Model) handleWikiEditorFinished(msg wikiEditorFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.execErr != nil {
		os.Remove(msg.edit.tempPath)
		m.message = "Editor error: " + msg.execErr.Error()
		m.logs.Error(m.message)
		return m, nil
	}
	edited, err := os.ReadFile(msg.edit.tempPath)
	if err != nil {
		m.message = "Error reading edited page: " + err.Error()
		m.logs.Error(m.message)
		return m, nil
	}
	if string(edited) == msg.edit.original.Content {
		os.Remove(msg.edit.tempPath)
		m.message = "No changes to publish"
		return m, nil
	}
	msg.edit.edited = string(edited)
	m.wiki.pendingEdit = msg.edit
	m.wiki.returnMode = m.mode
	m.mode = ModeConfirm
	m.inputAction = inputActionPublishWikiPage
	return m, nil
}

// publishWikiEdit saves the edit to ADO. The temp file is only removed once
// the page is saved, so a rejected edit isn't lost.
func (m Model) publishWikiEdit() (tea.Model, tea.Cmd) {
	m.mode = m.wiki.returnMode
	edit := m.wiki.pendingEdit
	m.wiki.pendingEdit = wikiEdit{}
	m.message = fmt.Sprintf("Publishing %q...", azuredevops.WikiPageTitle(edit.key.path))
	return m, func() tea.Msg {
		token, err := m.lookupPAT(edit.wiki.PAT)
		if err != nil {
			return errMsg{err}
		}
		if err := azuredevops.UpdateWikiPage(&edit.wiki, token, edit.key.path, edit.edited, edit.original.ETag); err != nil {
			return errMsg{fmt.Errorf("%w (your edit is kept in %s)", err, edit.tempPath)}
		}
		os.Remove(edit.tempPath)
		return wikiPagePublishedMsg{key: edit.key}
	}
}

func (m *Model) discardWikiEdit() {
	os.Remove(m.wiki.pendingEdit.tempPath)
	m.message = fmt.Sprintf("Discarded edit to %q", azuredevops.WikiPageTitle(m.wiki.pendingEdit.key.path))
	m.wiki.pendingEdit = wikiEdit{}
}

func (m Model) handleWikiPagePublished(msg wikiPagePublishedMsg) (tea.Model, tea.Cmd) {
	delete(m.wiki.pages, msg.key)
	m.message = fmt.Sprintf("Published %q", azuredevops.WikiPageTitle(msg.key.path))
	return m.loadSelectedWikiPage()
}

// openWikiMarkdown opens the selected page read-only in $EDITOR, the same
// way M opens a work item.
func (m Model) openWikiMarkdown() (tea.Model, tea.Cmd) {
	row, ok := m.selectedWikiPage()
	if !ok {
		return m, nil
	}
	w := m.config.Wikis[row.wikiIndex]
	m.message = fmt.Sprintf("Preparing %q as markdown...", row.page.Title())
	return m, func() tea.Msg {
		token, err := m.lookupPAT(w.PAT)
		if err != nil {
			return errMsg{err}
		}
		content, err := azuredevops.FetchWikiPage(&w, token, row.key.path)
		if err != nil {
			return errMsg{err}
		}
		path, err := writeWikiTempFile(content.Content)
		if err != nil {
			return errMsg{fmt.Errorf("writing wiki page to a temp file: %w", err)}
		}
		return adoMarkdownReadyMsg{path: path}
	}
}

func writeWikiTempFile(content string) (string, error) {
	f, err := os.CreateTemp("", wikiTempFilePrefix)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// handleWikiPageKeys handles the keys the wiki tree and the page view
// share, reporting whether msg was one of them.
func (m Model) handleWikiPageKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if model, cmd, handled := m.handleWikiLeader(msg); handled {
		return model, cmd, true
	}
	switch {
	case key.Matches(msg, m.keys.LiveReload):
		model, cmd := m.toggleWikiWatch()
		return model, cmd, true
	case key.Matches(msg, m.keys.Help):
		model, cmd := m.openHelp()
		return model, cmd, true
	case key.Matches(msg, m.keys.WikiFavorite):
		model, cmd := m.toggleWikiFavorite()
		return model, cmd, true
	case key.Matches(msg, m.keys.WikiFavorites):
		model, cmd := m.openWikiPicker(wikiPickerFavorites)
		return model, cmd, true
	case key.Matches(msg, m.keys.OpenEditor):
		model, cmd := m.editWikiPage()
		return model, cmd, true
	case key.Matches(msg, m.keys.MarkdownView):
		model, cmd := m.openWikiMarkdown()
		return model, cmd, true
	case key.Matches(msg, m.keys.OpenURL):
		model, cmd := m.openWikiPageInBrowser()
		return model, cmd, true
	case key.Matches(msg, m.keys.Refresh):
		model, cmd := m.syncWikis()
		return model, cmd, true
	case key.Matches(msg, m.keys.HalfUp):
		model, cmd := m.scrollWikiPage(-wikiScrollJump)
		return model, cmd, true
	case key.Matches(msg, m.keys.HalfDown):
		model, cmd := m.scrollWikiPage(wikiScrollJump)
		return model, cmd, true
	}
	return m, nil, false
}

func (m Model) handleWikiMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model, cmd, handled := m.handleWikiPageKeys(msg); handled {
		return model, cmd
	}
	topPressed := m.gPressed
	m.gPressed = false
	switch {
	case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.Quit):
		m.mode = ModeNormal
		m.message = ""
	case key.Matches(msg, m.keys.Up):
		return m.moveWikiSelection(-1)
	case key.Matches(msg, m.keys.Down):
		return m.moveWikiSelection(1)
	case key.Matches(msg, m.keys.Top):
		if topPressed {
			return m.moveWikiSelection(-m.wiki.selected)
		}
		m.gPressed = true
	case key.Matches(msg, m.keys.Bottom):
		return m.moveWikiSelection(len(m.wikiRows()))
	case key.Matches(msg, m.keys.Right):
		return m.expandWikiPage()
	case key.Matches(msg, m.keys.Left):
		return m.collapseWikiPage()
	case key.Matches(msg, m.keys.Fold):
		return m.toggleWikiFold()
	case key.Matches(msg, m.keys.FoldAll):
		return m.toggleWikiFoldAll()
	case key.Matches(msg, m.keys.Enter):
		return m.openWikiPageView()
	case key.Matches(msg, m.keys.Focus):
		return m.toggleWikiPreview()
	case key.Matches(msg, m.keys.New):
		return m.startAddWiki()
	case key.Matches(msg, m.keys.Delete):
		return m.confirmRemoveWiki()
	}
	return m, nil
}

func (m Model) handleWikiViewMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model, cmd, handled := m.handleWikiPageKeys(msg); handled {
		return model, cmd
	}
	topPressed := m.gPressed
	m.gPressed = false
	switch {
	case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.Quit):
		m.mode = ModeWiki
		m.wiki.scroll = 0
	case key.Matches(msg, m.keys.Up):
		return m.scrollWikiPage(-1)
	case key.Matches(msg, m.keys.Down):
		return m.scrollWikiPage(1)
	case key.Matches(msg, m.keys.Top):
		if topPressed {
			m.wiki.scroll = 0
			return m, nil
		}
		m.gPressed = true
	}
	return m, nil
}

// handleWikiCommand runs :wiki — wiki mode, or with a URL, adding that wiki.
func (m Model) handleWikiCommand(args []string) (tea.Model, tea.Cmd) {
	opened, openCmd := m.openWiki()
	if len(args) == 0 {
		return opened, openCmd
	}
	added, addCmd := opened.(Model).addWiki(strings.Join(args, " "))
	return added, tea.Batch(openCmd, addCmd)
}

func (m Model) wikiInputPrompt() string {
	switch m.inputAction {
	case inputActionRemoveWiki:
		if row, ok := m.selectedWikiRow(); ok {
			return fmt.Sprintf("Remove wiki %q from cmdban? Nothing changes in Azure DevOps. (y/n)", m.config.Wikis[row.wikiIndex].Name)
		}
	case inputActionPublishWikiPage:
		return fmt.Sprintf("Publish your changes to %q in Azure DevOps? (y/n)", azuredevops.WikiPageTitle(m.wiki.pendingEdit.key.path))
	}
	return ""
}
