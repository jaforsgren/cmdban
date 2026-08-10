package ui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cmdban/internal/azuredevops"
	"cmdban/internal/config"
)

type adoSetupStep int

const (
	adoSetupStepSelectPAT adoSetupStep = iota
	adoSetupStepSelectOrg
	adoSetupStepSelectProject
	adoSetupStepSelectTeam
	adoSetupStepSelectIteration
	adoSetupStepEnterBoardName
)

type adoSetupState struct {
	step          adoSetupStep
	selectorIndex int
	patAlias      string
	org           string
	orgs          []string
	project       string
	projects      []string
	team          string
	teams         []string
	boardLevel    string
	isBacklog     bool
	boardColumns  []azuredevops.BoardColumn
	iterations    []azuredevops.Iteration
	iteration     string
	boardName     string
	loading       bool
	loadError     string
	fromURL       bool
}

type adoOrgsDiscoveredMsg struct{ orgs []string }
type adoOrgsDiscoveryFailedMsg struct{ err error }
type adoProjectsDiscoveredMsg struct{ projects []string }
type adoTeamsDiscoveredMsg struct{ teams []string }
type adoBoardColumnsDiscoveredMsg struct{ columns []azuredevops.BoardColumn }
type adoIterationsDiscoveredMsg struct{ iterations []azuredevops.Iteration }

func (m Model) startADOSetup() (tea.Model, tea.Cmd) {
	if len(m.patStore.PATs) == 0 {
		m.message = "No PATs stored. Run :pats to add one first."
		return m, nil
	}
	m.adoSetup = adoSetupState{step: adoSetupStepSelectPAT}
	m.mode = ModeInput
	m.inputAction = "ado-url"
	m.textInput.Reset()
	m.textInput.Placeholder = "Paste ADO board/backlog URL, or Enter to pick manually..."
	m.textInput.EchoMode = textinput.EchoNormal
	m.textInput.Width = 70
	m.textInput.Focus()
	return m, textinput.Blink
}

// parseADOBoardURL extracts org, project, team, and board level from either
// a Kanban board URL or a backlog URL:
//
//	https://dev.azure.com/{org}/{project}/_boards/board/t/{team}/{level}
//	https://dev.azure.com/{org}/{project}/_backlogs/backlog/{team}/{level}
//
// isBacklog reports whether the URL was a backlog URL — backlog levels list
// every item at that level regardless of sprint iteration, unlike Kanban
// boards, so callers use it to skip iteration-based setup entirely.
func parseADOBoardURL(raw string) (org, project, team, boardLevel string, isBacklog bool, err error) {
	u, parseErr := url.Parse(raw)
	if parseErr != nil {
		return "", "", "", "", false, fmt.Errorf("invalid URL: %w", parseErr)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")

	switch {
	case len(parts) >= 6 && parts[2] == "_boards" && parts[4] == "t":
		level := ""
		if len(parts) > 6 {
			level = parts[6]
		}
		return parts[0], parts[1], parts[5], level, false, nil
	case len(parts) >= 5 && parts[2] == "_backlogs" && parts[3] == "backlog":
		level := ""
		if len(parts) > 5 {
			level = parts[5]
		}
		return parts[0], parts[1], parts[4], level, true, nil
	default:
		return "", "", "", "", false, fmt.Errorf("not a recognised ADO board or backlog URL")
	}
}

func (m Model) handleADOSetupMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" || msg.String() == "q" {
		m.mode = ModeNormal
		return m, nil
	}

	if m.adoSetup.loading {
		return m, nil
	}

	switch m.adoSetup.step {
	case adoSetupStepSelectPAT:
		return m.handleADOSetupSelectPAT(msg)
	case adoSetupStepSelectOrg:
		return m.handleADOSetupSelectList(msg, m.adoSetup.orgs, func(m Model, selected string) (tea.Model, tea.Cmd) {
			m.adoSetup.org = selected
			m.adoSetup.loading = true
			m.adoSetup.step = adoSetupStepSelectProject
			m.adoSetup.selectorIndex = 0
			return m, m.discoverProjects
		})
	case adoSetupStepSelectProject:
		return m.handleADOSetupSelectList(msg, m.adoSetup.projects, func(m Model, selected string) (tea.Model, tea.Cmd) {
			m.adoSetup.project = selected
			m.adoSetup.boardName = strings.ToLower(strings.ReplaceAll(selected, " ", "-")) + "-sprint"
			m.adoSetup.loading = true
			m.adoSetup.step = adoSetupStepSelectTeam
			m.adoSetup.selectorIndex = 0
			return m, m.discoverTeams
		})
	case adoSetupStepSelectTeam:
		return m.handleADOSetupSelectList(msg, m.adoSetup.teams, func(m Model, selected string) (tea.Model, tea.Cmd) {
			m.adoSetup.team = selected
			m.adoSetup.boardName = strings.ToLower(strings.ReplaceAll(m.adoSetup.project, " ", "-")) + "-sprint"
			m.adoSetup.loading = true
			return m, m.discoverBoardColumns
		})
	case adoSetupStepSelectIteration:
		return m.handleADOSetupSelectIteration(msg)
	}
	return m, nil
}

func (m Model) handleADOSetupSelectIteration(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// index 0 is the synthetic "@CurrentIteration" option; real iterations start at 1
	total := len(m.adoSetup.iterations) + 1
	switch msg.String() {
	case "j", "down":
		m.adoSetup.selectorIndex = (m.adoSetup.selectorIndex + 1) % total
	case "k", "up":
		m.adoSetup.selectorIndex = (m.adoSetup.selectorIndex - 1 + total) % total
	case "enter":
		if m.adoSetup.selectorIndex == 0 {
			m.adoSetup.iteration = "@CurrentIteration"
		} else {
			m.adoSetup.iteration = m.adoSetup.iterations[m.adoSetup.selectorIndex-1].Path
		}
		return m.enterBoardNameStep()
	}
	return m, nil
}

func (m Model) handleADOSetupSelectPAT(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	pats := m.patStore.PATs
	switch msg.String() {
	case "j", "down":
		m.adoSetup.selectorIndex = (m.adoSetup.selectorIndex + 1) % len(pats)
	case "k", "up":
		m.adoSetup.selectorIndex = (m.adoSetup.selectorIndex - 1 + len(pats)) % len(pats)
	case "enter":
		m.adoSetup.patAlias = pats[m.adoSetup.selectorIndex].Name
		m.adoSetup.loading = true
		m.adoSetup.loadError = ""
		if m.adoSetup.fromURL {
			// org and project already known from URL — go straight to team picker
			m.adoSetup.step = adoSetupStepSelectTeam
			return m, m.discoverTeams
		}
		return m, m.discoverOrgs
	}
	return m, nil
}

// handleADOSetupSelectList handles j/k/enter for any list-selection step.
type adoSelectCallback func(Model, string) (tea.Model, tea.Cmd)

func (m Model) handleADOSetupSelectList(msg tea.KeyMsg, items []string, onSelect adoSelectCallback) (tea.Model, tea.Cmd) {
	if len(items) == 0 {
		return m, nil
	}
	switch msg.String() {
	case "j", "down":
		m.adoSetup.selectorIndex = (m.adoSetup.selectorIndex + 1) % len(items)
	case "k", "up":
		m.adoSetup.selectorIndex = (m.adoSetup.selectorIndex - 1 + len(items)) % len(items)
	case "enter":
		return onSelect(m, items[m.adoSetup.selectorIndex])
	}
	return m, nil
}

func (m Model) discoverOrgs() tea.Msg {
	token, ok := m.patStore.Get(m.adoSetup.patAlias)
	if !ok {
		return adoOrgsDiscoveryFailedMsg{fmt.Errorf("PAT %q not found", m.adoSetup.patAlias)}
	}
	orgs, err := azuredevops.DiscoverOrganizations(token)
	if err != nil {
		return adoOrgsDiscoveryFailedMsg{err}
	}
	return adoOrgsDiscoveredMsg{orgs}
}

func (m Model) discoverProjects() tea.Msg {
	token, ok := m.patStore.Get(m.adoSetup.patAlias)
	if !ok {
		return errMsg{fmt.Errorf("PAT %q not found", m.adoSetup.patAlias)}
	}
	projects, err := azuredevops.DiscoverProjects(m.adoSetup.org, token)
	if err != nil {
		return errMsg{err}
	}
	return adoProjectsDiscoveredMsg{projects}
}

func (m Model) discoverTeams() tea.Msg {
	token, ok := m.patStore.Get(m.adoSetup.patAlias)
	if !ok {
		return errMsg{fmt.Errorf("PAT %q not found", m.adoSetup.patAlias)}
	}
	teams, err := azuredevops.DiscoverTeams(m.adoSetup.org, m.adoSetup.project, token)
	if err != nil {
		return errMsg{err}
	}
	return adoTeamsDiscoveredMsg{teams}
}

func (m Model) discoverBoardColumns() tea.Msg {
	token, ok := m.patStore.Get(m.adoSetup.patAlias)
	if !ok {
		return errMsg{fmt.Errorf("PAT %q not found", m.adoSetup.patAlias)}
	}
	columns, err := azuredevops.DiscoverBoardColumns(
		m.adoSetup.org, m.adoSetup.project, m.adoSetup.team, m.adoSetup.boardLevel, token,
	)
	if err != nil {
		return errMsg{err}
	}
	return adoBoardColumnsDiscoveredMsg{columns}
}

func (m Model) handleADOBoardColumnsDiscovered(columns []azuredevops.BoardColumn) (tea.Model, tea.Cmd) {
	m.adoSetup.boardColumns = columns
	if m.adoSetup.isBacklog {
		// Backlog levels aren't scoped to a sprint iteration, so there's
		// nothing to pick — skip straight to naming the board.
		return m.enterBoardNameStep()
	}
	m.adoSetup.loading = true
	m.adoSetup.step = adoSetupStepSelectIteration
	return m, m.discoverIterations
}

func (m Model) discoverIterations() tea.Msg {
	token, ok := m.patStore.Get(m.adoSetup.patAlias)
	if !ok {
		return errMsg{fmt.Errorf("PAT %q not found", m.adoSetup.patAlias)}
	}
	iterations, err := azuredevops.DiscoverIterations(
		m.adoSetup.org, m.adoSetup.project, m.adoSetup.team, token,
	)
	if err != nil {
		return errMsg{err}
	}
	return adoIterationsDiscoveredMsg{iterations}
}

func (m Model) handleADOIterationsDiscovered(iterations []azuredevops.Iteration) (tea.Model, tea.Cmd) {
	m.adoSetup.loading = false
	m.adoSetup.iterations = iterations
	m.adoSetup.selectorIndex = 0
	m.mode = ModeADOSetup
	return m, nil
}

func (m Model) enterBoardNameStep() (tea.Model, tea.Cmd) {
	m.adoSetup.step = adoSetupStepEnterBoardName
	m.mode = ModeInput
	m.inputAction = "ado-board-name"
	m.textInput.Reset()
	m.textInput.SetValue(m.adoSetup.boardName)
	m.textInput.Placeholder = "Board name..."
	m.textInput.EchoMode = textinput.EchoNormal
	m.textInput.Focus()
	return m, textinput.Blink
}

func generateColumnConfig(boardColumns []azuredevops.BoardColumn) ([]config.Column, map[string]string) {
	slugCounts := map[string]int{}
	type pair struct {
		slug    string
		adoName string
	}
	pairs := make([]pair, 0, len(boardColumns))
	for _, bc := range boardColumns {
		s := slugify(bc.Name)
		slugCounts[s]++
		pairs = append(pairs, pair{s, bc.Name})
	}

	seen := map[string]int{}
	columns := make([]config.Column, 0, len(pairs))
	columnMap := make(map[string]string, len(pairs))
	for _, p := range pairs {
		slug := p.slug
		if slugCounts[slug] > 1 {
			seen[slug]++
			slug = fmt.Sprintf("%s-%d", slug, seen[slug])
		}
		columns = append(columns, config.Column{Name: slug})
		columnMap[slug] = p.adoName
	}
	return columns, columnMap
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			last := rune(b.String()[b.Len()-1])
			if last != '-' {
				b.WriteRune('-')
			}
		}
	}
	result := strings.TrimRight(b.String(), "-")
	return result
}

// backlogLevelFor returns the ADO backlog level to store for a backlog-
// sourced board, or "" for a Kanban board (which is scoped by iteration).
func backlogLevelFor(setup adoSetupState) string {
	if setup.isBacklog {
		return setup.boardLevel
	}
	return ""
}

func (m Model) saveADOBoard(boardName string) (tea.Model, tea.Cmd) {
	setup := m.adoSetup
	columns, columnMap := generateColumnConfig(setup.boardColumns)
	if len(columns) == 0 {
		columns = []config.Column{
			{Name: "today"},
			{Name: "tomorrow"},
			{Name: "backlog"},
			{Name: "done"},
		}
		columnMap = map[string]string{
			"today":    "Active",
			"tomorrow": "Committed",
			"backlog":  "New",
			"done":     "Closed",
		}
	}
	board := config.Board{
		Name: boardName,
		Type: config.BoardTypeAzureDevOps,
		AzureDevOps: &config.AzureDevOpsConfig{
			Org:                 setup.org,
			Project:             setup.project,
			Team:                setup.team,
			Iteration:           setup.iteration,
			BacklogLevel:        backlogLevelFor(setup),
			PAT:                 setup.patAlias,
			ColumnMap:           columnMap,
			DefaultWorkItemType: "User Story",
		},
		Columns: columns,
	}
	m.config.Boards = append(m.config.Boards, board)
	if err := m.config.Save(); err != nil {
		m.message = "Error saving config: " + err.Error()
		m.logs.Error(m.message)
		m.mode = ModeNormal
		return m, nil
	}
	return m.switchBoard(boardName)
}

func (m Model) renderADOSetup() string {
	setup := m.adoSetup

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Bold(true).Foreground(HighlightColor).Render("  New Azure DevOps Board"))
	rows = append(rows, "")

	rows = append(rows, m.renderADOSetupBreadcrumb())
	rows = append(rows, "")

	if setup.loadError != "" {
		rows = append(rows, lipgloss.NewStyle().Foreground(WarningColor).Render("  "+setup.loadError))
		rows = append(rows, "")
	}

	switch setup.step {
	case adoSetupStepSelectPAT:
		if setup.fromURL {
			parsed := lipgloss.NewStyle().Foreground(HighlightColor).Render(
				fmt.Sprintf("  %s  /  %s  /  %s", setup.org, setup.project, setup.team),
			)
			rows = append(rows, parsed)
			rows = append(rows, "")
		}
		rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Render("  Select PAT  ·  j/k navigate  enter select  esc cancel"))
		rows = append(rows, "")
		for i, p := range m.patStore.PATs {
			rows = append(rows, renderADOSetupItem(p.Name, i == setup.selectorIndex))
		}

	case adoSetupStepSelectOrg:
		if setup.loading {
			rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Italic(true).Render("  Discovering organisations..."))
		} else {
			rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Render("  Select Organisation  ·  j/k navigate  enter select  esc cancel"))
			rows = append(rows, "")
			for i, org := range setup.orgs {
				rows = append(rows, renderADOSetupItem(org, i == setup.selectorIndex))
			}
		}

	case adoSetupStepSelectProject:
		if setup.loading {
			rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Italic(true).Render(
				fmt.Sprintf("  Fetching projects from %s...", setup.org),
			))
		} else {
			rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Render("  Select Project  ·  j/k navigate  enter select  esc cancel"))
			rows = append(rows, "")
			for i, p := range setup.projects {
				rows = append(rows, renderADOSetupItem(p, i == setup.selectorIndex))
			}
		}

	case adoSetupStepSelectTeam:
		if setup.loading {
			rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Italic(true).Render(
				fmt.Sprintf("  Fetching teams in %s / %s...", setup.org, setup.project),
			))
		} else {
			label := "Select Team"
			if setup.fromURL {
				label = "Confirm Team"
				rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Render(
					fmt.Sprintf("  %s / %s", setup.org, setup.project),
				))
				rows = append(rows, "")
			}
			rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Render(
				fmt.Sprintf("  %s  ·  j/k navigate  enter select  esc cancel", label),
			))
			rows = append(rows, "")
			for i, t := range setup.teams {
				rows = append(rows, renderADOSetupItem(t, i == setup.selectorIndex))
			}
		}

	case adoSetupStepSelectIteration:
		if setup.loading {
			rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Italic(true).Render("  Fetching iterations..."))
		} else {
			rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Render("  Select Iteration  ·  j/k navigate  enter select  esc cancel"))
			rows = append(rows, "")
			rows = append(rows, renderADOSetupItem("Current sprint (auto)", setup.selectorIndex == 0))
			for i, it := range setup.iterations {
				label := it.Name
				if it.Attributes.TimeFrame == "current" {
					label += "  (current)"
				}
				rows = append(rows, renderADOSetupItem(label, setup.selectorIndex == i+1))
			}
		}

	case adoSetupStepEnterBoardName:
		rows = append(rows, lipgloss.NewStyle().Foreground(SubtleColor).Italic(true).Render("  Entering board name..."))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return DialogStyle.Width(60).Render(content)
}

func (m Model) renderADOSetupBreadcrumb() string {
	setup := m.adoSetup
	steps := []string{"PAT", "Org", "Project", "Team", "Iteration", "Name"}
	stepIndex := int(setup.step)

	var parts []string
	for i, s := range steps {
		var style lipgloss.Style
		switch {
		case i < stepIndex:
			style = lipgloss.NewStyle().Foreground(SuccessColor)
		case i == stepIndex:
			style = lipgloss.NewStyle().Foreground(HighlightColor).Bold(true)
		default:
			style = lipgloss.NewStyle().Foreground(SubtleColor)
		}
		label := s
		if i < stepIndex {
			switch adoSetupStep(i) {
			case adoSetupStepSelectPAT:
				label = "PAT: " + setup.patAlias
			case adoSetupStepSelectOrg:
				label = setup.org
			case adoSetupStepSelectProject:
				label = setup.project
			case adoSetupStepSelectTeam:
				label = setup.team
			case adoSetupStepSelectIteration:
				switch {
				case setup.isBacklog:
					label = "backlog: " + setup.boardLevel
				case setup.iteration == "@CurrentIteration":
					label = "auto"
				default:
					label = iterationDisplayName(setup.iteration)
				}
			}
		}
		parts = append(parts, style.Render(label))
	}
	return "  " + strings.Join(parts, lipgloss.NewStyle().Foreground(SubtleColor).Render(" › "))
}

// iterationDisplayName returns just the leaf name from a full ADO iteration path.
func iterationDisplayName(path string) string {
	if i := strings.LastIndex(path, `\`); i >= 0 {
		return path[i+1:]
	}
	return path
}

func renderADOSetupItem(label string, selected bool) string {
	cursor := "  "
	style := lipgloss.NewStyle()
	if selected {
		cursor = "▶ "
		style = style.Foreground(lipgloss.Color("#FFFFFF")).Background(ActiveColor).Bold(true)
	}
	return cursor + style.Render(label)
}

func (m Model) handleADOOrgsDiscovered(orgs []string) (tea.Model, tea.Cmd) {
	m.adoSetup.loading = false
	m.adoSetup.orgs = orgs
	m.adoSetup.selectorIndex = 0
	m.adoSetup.step = adoSetupStepSelectOrg
	return m, nil
}

func (m Model) handleADOOrgsDiscoveryFailed(err error) (tea.Model, tea.Cmd) {
	m.adoSetup.loading = false
	m.adoSetup.loadError = "Could not fetch organisations: " + err.Error()
	m.logs.Error(m.adoSetup.loadError)
	m.adoSetup.step = adoSetupStepSelectOrg
	m.mode = ModeInput
	m.inputAction = "ado-org-name"
	m.textInput.Reset()
	m.textInput.Placeholder = "Organisation name (e.g. myorg)..."
	m.textInput.EchoMode = textinput.EchoNormal
	m.textInput.Focus()
	return m, textinput.Blink
}
