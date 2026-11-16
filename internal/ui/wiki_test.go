package ui

import (
	"os"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"cmdban/internal/azuredevops"
	"cmdban/internal/config"
	"cmdban/internal/pat"
)

func wikiTestConfig() *config.Config {
	cfg := adoTestConfig()
	cfg.Wikis = []config.Wiki{
		{Name: "Krav", Org: "org", Project: "proj", Wiki: "Docs", PageID: 93, PAT: "work"},
		{Name: "Guides", Org: "org", Project: "proj", Wiki: "Docs", PageID: 7, PAT: "work"},
	}
	return cfg
}

func wikiTestTree() azuredevops.WikiPage {
	return azuredevops.WikiPage{ID: 93, Path: "/Krav", SubPages: []azuredevops.WikiPage{
		{Path: "/Krav/Insurify", SubPages: []azuredevops.WikiPage{
			{Path: "/Krav/Insurify/SIE"},
			{Path: "/Krav/Insurify/Regler"},
		}},
		{Path: "/Krav/Pay later"},
	}}
}

// wikiTestModel is in wiki mode with the first wiki's tree loaded and the
// second still loading.
func wikiTestModel(t *testing.T, cfg *config.Config) Model {
	t.Helper()
	m := adoTestModel(cfg)
	m.patStore = &pat.Store{PATs: []pat.Entry{{Name: "work", Token: "secret"}}}
	m.mode = ModeWiki
	m = update(t, m, wikiTreeLoadedMsg{wiki: wikiID(cfg.Wikis[0]), root: wikiTestTree()})
	m.wiki.trees[wikiID(cfg.Wikis[1])] = wikiTree{loading: true}
	return m
}

func update(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	got, _ := m.Update(msg)
	return got.(Model)
}

func rowTitles(m Model) []string {
	var titles []string
	for _, row := range m.wikiRows() {
		title, _ := m.wikiRowText(row)
		titles = append(titles, title)
	}
	return titles
}

func selectedTitle(t *testing.T, m Model) string {
	t.Helper()
	row, ok := m.selectedWikiRow()
	if !ok {
		t.Fatal("no wiki row selected")
	}
	title, _ := m.wikiRowText(row)
	return title
}

func runCommand(t *testing.T, m Model, command string) (Model, tea.Cmd) {
	t.Helper()
	got, cmd := m.handleCommand(command)
	return got.(Model), cmd
}

func TestWikiCommandAndShortAliasOpenWikiMode(t *testing.T) {
	for _, command := range []string{"wiki", "w"} {
		m, _ := runCommand(t, adoTestModel(wikiTestConfig()), command)
		if m.mode != ModeWiki {
			t.Errorf(":%s mode = %v, want ModeWiki", command, m.mode)
		}
	}
}

func TestWikiCommandSyncsEveryWikiTheFirstTime(t *testing.T) {
	m, cmd := runCommand(t, adoTestModel(wikiTestConfig()), "wiki")
	if cmd == nil {
		t.Fatal("cmd = nil, want the wiki trees fetched")
	}
	for _, w := range m.config.Wikis {
		if !m.wiki.trees[wikiID(w)].loading {
			t.Errorf("wiki %q not loading", w.Name)
		}
	}
}

func TestWikiCommandWithoutWikisExplainsHowToAdd(t *testing.T) {
	m, _ := runCommand(t, adoTestModel(adoTestConfig()), "wiki")
	if !strings.Contains(m.message, "press a") {
		t.Fatalf("message = %q, want a hint to add a wiki", m.message)
	}
}

func TestWikisShowAsOneCollapsedLineEach(t *testing.T) {
	m := wikiTestModel(t, wikiTestConfig())
	if got, want := rowTitles(m), []string{"Krav", "Guides"}; !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestLExpandsThenStepsIntoTheFirstSubPage(t *testing.T) {
	m := wikiTestModel(t, wikiTestConfig())

	m = pressKey(t, m, "l")
	if got, want := rowTitles(m), []string{"Krav", "Insurify", "Pay later", "Guides"}; !slices.Equal(got, want) {
		t.Fatalf("rows after l = %v, want %v", got, want)
	}
	if got := selectedTitle(t, m); got != "Krav" {
		t.Fatalf("selected = %q, want expanding to keep the selection", got)
	}

	m = pressKey(t, m, "l")
	if got := selectedTitle(t, m); got != "Insurify" {
		t.Fatalf("selected = %q, want l on an open page to step into it", got)
	}
}

func TestHStepsOutToTheParentThenCollapses(t *testing.T) {
	m := wikiTestModel(t, wikiTestConfig())
	m = pressKey(t, pressKey(t, pressKey(t, m, "l"), "l"), "l")
	m = pressKey(t, m, "l")
	if got := selectedTitle(t, m); got != "SIE" {
		t.Fatalf("selected = %q, want SIE", got)
	}

	m = pressKey(t, m, "h")
	if got := selectedTitle(t, m); got != "Insurify" {
		t.Fatalf("selected = %q, want h on a leaf to step out to the parent", got)
	}
	m = pressKey(t, m, "h")
	if got, want := rowTitles(m), []string{"Krav", "Insurify", "Pay later", "Guides"}; !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want h on an open page to collapse it", got)
	}
}

func TestWikiGuidesDrawTheTree(t *testing.T) {
	cases := []struct {
		guides []bool
		want   string
	}{
		{[]bool{false}, "├─ "},
		{[]bool{true}, "└─ "},
		{[]bool{false, true}, "│  └─ "},
		{[]bool{true, false}, "   ├─ "},
	}
	for _, c := range cases {
		if got := wikiGuides(c.guides); got != c.want {
			t.Errorf("wikiGuides(%v) = %q, want %q", c.guides, got, c.want)
		}
	}
}

func TestWikiListShowsPageCountsAndMarkers(t *testing.T) {
	m := pressKey(t, wikiTestModel(t, wikiTestConfig()), "l")
	view := m.renderWikiList(m.width)
	for _, want := range []string{wikiExpandedMarker + "Krav", "4 pages", wikiCollapsedMarker + "Insurify", "(2)", wikiLeafMarker + "Pay later", wikiLoadingText} {
		if !strings.Contains(view, want) {
			t.Errorf("wiki list missing %q:\n%s", want, view)
		}
	}
}

func TestTabPreviewsTheSelectedPage(t *testing.T) {
	m := wikiTestModel(t, wikiTestConfig())
	got, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = got.(Model)
	if !m.wiki.preview || cmd == nil {
		t.Fatalf("preview = %v, cmd = %v; want the preview on and the page fetched", m.wiki.preview, cmd)
	}

	key := wikiPageKey{wiki: wikiID(m.config.Wikis[0]), path: "/Krav"}
	m = update(t, m, wikiPageLoadedMsg{key: key, content: azuredevops.WikiPageContent{Content: "Requirements live here"}})
	if view := m.renderWiki(); !strings.Contains(view, "Requirements live here") {
		t.Fatalf("preview missing the page content:\n%s", view)
	}
}

func TestEnterReadsThePageAndQGoesBackToTheTree(t *testing.T) {
	m := wikiTestModel(t, wikiTestConfig())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != ModeWikiView {
		t.Fatalf("mode = %v, want ModeWikiView", m.mode)
	}
	m = pressKey(t, m, "q")
	if m.mode != ModeWiki {
		t.Fatalf("mode = %v, want q to return to the wiki tree", m.mode)
	}
	m = pressKey(t, m, "q")
	if m.mode != ModeNormal {
		t.Fatalf("mode = %v, want q in the tree to return to the board", m.mode)
	}
}

func TestCtrlOOpensTheSelectedWikiPage(t *testing.T) {
	opened := stubBrowser(t)
	m := pressKey(t, pressKey(t, wikiTestModel(t, wikiTestConfig()), "l"), "j")

	m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})

	if len(*opened) != 1 || !strings.HasSuffix((*opened)[0], "pagePath=%2FKrav%2FInsurify") {
		t.Fatalf("opened %v, want the Insurify page", *opened)
	}
}

func TestWikiModeLeavesTheBoardsOut(t *testing.T) {
	m := wikiTestModel(t, wikiTestConfig())
	topBar := m.renderTopBar()
	if strings.Contains(topBar, "RECENT") || strings.Contains(topBar, "FILTERS") {
		t.Fatalf("wiki topbar shows board sections:\n%s", topBar)
	}
	if !strings.Contains(topBar, "Wikis") {
		t.Fatalf("wiki topbar missing the wiki context:\n%s", topBar)
	}
	for _, board := range m.config.Boards {
		if slices.Contains(rowTitles(m), board.Name) {
			t.Errorf("board %q listed among the wikis", board.Name)
		}
	}
}

func TestEscapeFromTheAddWikiPromptReturnsToWikiMode(t *testing.T) {
	m := pressKey(t, wikiTestModel(t, wikiTestConfig()), "a")
	if m.mode != ModeInput || m.displayMode() != ModeWiki {
		t.Fatalf("mode = %v, display = %v; want the prompt over the wiki screen", m.mode, m.displayMode())
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != ModeWiki {
		t.Fatalf("mode = %v, want ModeWiki", m.mode)
	}
}

func TestAddWikiRejectsANonWikiURL(t *testing.T) {
	m := wikiTestModel(t, wikiTestConfig())
	got, cmd := m.addWiki("https://dev.azure.com/org/proj/_boards/board/t/team")
	if cmd != nil || !strings.Contains(got.(Model).message, "Invalid wiki URL") {
		t.Fatalf("message = %q, want the URL rejected", got.(Model).message)
	}
}

func TestWikiAddedIsSavedAndSelected(t *testing.T) {
	cfg := savableConfig(t, wikiTestConfig())
	cfg.Wikis = wikiTestConfig().Wikis
	m := wikiTestModel(t, cfg)
	added := config.Wiki{Name: "Ops", Org: "org", Project: "proj", Wiki: "Docs", PageID: 12, PAT: "work"}

	m = update(t, m, wikiAddedMsg{wiki: added, root: azuredevops.WikiPage{ID: 12, Path: "/Ops"}})

	if got := selectedTitle(t, m); got != "Ops" {
		t.Fatalf("selected = %q, want the added wiki", got)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if len(loaded.Wikis) != 3 || loaded.Wikis[2] != added {
		t.Fatalf("saved wikis = %+v, want the added wiki last", loaded.Wikis)
	}
}

func TestXRemovesAWikiAfterConfirming(t *testing.T) {
	cfg := savableConfig(t, wikiTestConfig())
	cfg.Wikis = wikiTestConfig().Wikis
	m := pressKey(t, wikiTestModel(t, cfg), "x")
	if m.mode != ModeConfirm || !strings.Contains(m.renderFooter(), "Remove wiki \"Krav\"") {
		t.Fatalf("mode = %v, footer = %q; want a removal confirmation", m.mode, m.renderFooter())
	}

	m = pressKey(t, m, "y")

	if m.mode != ModeWiki {
		t.Fatalf("mode = %v, want ModeWiki", m.mode)
	}
	if got := rowTitles(m); !slices.Equal(got, []string{"Guides"}) {
		t.Fatalf("rows = %v, want only Guides left", got)
	}
}

func TestXOnASubPageDoesNotRemoveAnything(t *testing.T) {
	m := pressKey(t, pressKey(t, pressKey(t, wikiTestModel(t, wikiTestConfig()), "l"), "j"), "x")
	if m.mode != ModeWiki || len(m.config.Wikis) != 2 {
		t.Fatalf("mode = %v, wikis = %d; want nothing removed", m.mode, len(m.config.Wikis))
	}
}

func wikiEditWithTempFile(t *testing.T, m Model, content string) wikiEdit {
	t.Helper()
	path, err := writeWikiTempFile(content)
	if err != nil {
		t.Fatalf("writeWikiTempFile() error = %v", err)
	}
	t.Cleanup(func() { os.Remove(path) })
	return wikiEdit{
		wiki:     m.config.Wikis[0],
		key:      wikiPageKey{wiki: wikiID(m.config.Wikis[0]), path: "/Krav"},
		tempPath: path,
		original: azuredevops.WikiPageContent{Content: "original", ETag: `"abc"`},
	}
}

func TestAnUnchangedWikiEditIsNotPublished(t *testing.T) {
	m := wikiTestModel(t, wikiTestConfig())
	edit := wikiEditWithTempFile(t, m, "original")

	m = update(t, m, wikiEditorFinishedMsg{edit: edit})

	if m.mode != ModeWiki || m.message != "No changes to publish" {
		t.Fatalf("mode = %v, message = %q; want nothing to publish", m.mode, m.message)
	}
}

func TestAChangedWikiEditAsksBeforePublishing(t *testing.T) {
	m := wikiTestModel(t, wikiTestConfig())
	edit := wikiEditWithTempFile(t, m, "changed")

	m = update(t, m, wikiEditorFinishedMsg{edit: edit})

	if m.mode != ModeConfirm || m.displayMode() != ModeWiki {
		t.Fatalf("mode = %v, display = %v; want a confirmation over the wiki screen", m.mode, m.displayMode())
	}
	if !strings.Contains(m.renderFooter(), "Publish your changes to \"Krav\"") {
		t.Fatalf("footer = %q, want the publish prompt", m.renderFooter())
	}

	m = pressKey(t, m, "n")
	if m.mode != ModeWiki {
		t.Fatalf("mode = %v, want ModeWiki after declining", m.mode)
	}
	if _, err := os.Stat(edit.tempPath); !os.IsNotExist(err) {
		t.Fatalf("temp file still exists after discarding the edit (err = %v)", err)
	}
}

func TestHelpFromWikiModeReturnsToWikiMode(t *testing.T) {
	m := pressKey(t, wikiTestModel(t, wikiTestConfig()), "?")
	if m.mode != ModeHelp {
		t.Fatalf("mode = %v, want ModeHelp", m.mode)
	}
	if m = pressKey(t, m, "q"); m.mode != ModeWiki {
		t.Fatalf("mode = %v, want ModeWiki", m.mode)
	}
}

func TestAFailedWikiShowsItsErrorOnItsLine(t *testing.T) {
	m := wikiTestModel(t, wikiTestConfig())
	m = update(t, m, wikiTreeLoadedMsg{wiki: wikiID(m.config.Wikis[1]), err: errBoom})
	if view := m.renderWikiList(m.width); !strings.Contains(view, "⚠ boom") {
		t.Fatalf("wiki list missing the error:\n%s", view)
	}
}

func TestZTogglesTheSelectedPageOnly(t *testing.T) {
	m := pressKey(t, wikiTestModel(t, wikiTestConfig()), "z")
	if got, want := rowTitles(m), []string{"Krav", "Insurify", "Pay later", "Guides"}; !slices.Equal(got, want) {
		t.Fatalf("rows after z = %v, want %v", got, want)
	}
	if got := selectedTitle(t, m); got != "Krav" {
		t.Fatalf("selected = %q, want z to keep the selection", got)
	}
	if m = pressKey(t, m, "z"); !slices.Equal(rowTitles(m), []string{"Krav", "Guides"}) {
		t.Fatalf("rows after second z = %v, want Krav folded", rowTitles(m))
	}
}

func TestShiftZExpandsEverythingBeneathThenFoldsItAll(t *testing.T) {
	m := pressKey(t, wikiTestModel(t, wikiTestConfig()), "Z")
	want := []string{"Krav", "Insurify", "SIE", "Regler", "Pay later", "Guides"}
	if got := rowTitles(m); !slices.Equal(got, want) {
		t.Fatalf("rows after Z = %v, want %v", got, want)
	}
	if m = pressKey(t, m, "Z"); !slices.Equal(rowTitles(m), []string{"Krav", "Guides"}) {
		t.Fatalf("rows after second Z = %v, want everything folded", rowTitles(m))
	}
}

func TestShiftZOpensTheRestWhenOnlySomeIsOpen(t *testing.T) {
	m := pressKey(t, pressKey(t, wikiTestModel(t, wikiTestConfig()), "z"), "Z")
	if got := len(rowTitles(m)); got != 6 {
		t.Fatalf("rows after z then Z = %d, want everything beneath Krav open (6)", got)
	}
}

func typeInto(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		m = pressKey(t, m, string(r))
	}
	return m
}

func pickerLabels(m Model) []string {
	var labels []string
	for _, result := range m.wiki.picker.results {
		labels = append(labels, result.label)
	}
	return labels
}

func TestSpaceSpaceOpensThePageFinderListingEveryPage(t *testing.T) {
	m := typeInto(t, wikiTestModel(t, wikiTestConfig()), "  ")
	if m.mode != ModeWikiPicker || m.wiki.picker.kind != wikiPickerFindPages {
		t.Fatalf("mode = %v, kind = %v; want the page finder", m.mode, m.wiki.picker.kind)
	}
	want := []string{"Krav", "Krav › Insurify", "Krav › Insurify › SIE", "Krav › Insurify › Regler", "Krav › Pay later"}
	if got := pickerLabels(m); !slices.Equal(got, want) {
		t.Fatalf("results = %v, want every loaded page %v", got, want)
	}
}

func TestPageFinderFuzzyMatchesBreadcrumbs(t *testing.T) {
	m := typeInto(t, wikiTestModel(t, wikiTestConfig()), "  insie")
	if got := pickerLabels(m); len(got) == 0 || got[0] != "Krav › Insurify › SIE" {
		t.Fatalf("results = %v, want Insurify › SIE first", got)
	}
}

func TestSpaceFollowedByAnotherKeyCancelsTheLeader(t *testing.T) {
	m := typeInto(t, wikiTestModel(t, wikiTestConfig()), " q")
	if m.mode != ModeWiki || m.wiki.leader != "" {
		t.Fatalf("mode = %v, leader = %q; want the sequence dropped without leaving wiki mode", m.mode, m.wiki.leader)
	}
}

func TestEnterOnAFoundPageRevealsItInTheTreeAndReadsIt(t *testing.T) {
	m := typeInto(t, wikiTestModel(t, wikiTestConfig()), "  regler")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.mode != ModeWikiView {
		t.Fatalf("mode = %v, want the page view", m.mode)
	}
	if got := selectedTitle(t, m); got != "Regler" {
		t.Fatalf("selected = %q, want Regler", got)
	}
	if m = pressKey(t, m, "q"); !slices.Contains(rowTitles(m), "Regler") {
		t.Fatalf("rows = %v, want Regler's parents expanded", rowTitles(m))
	}
}

func TestEscClosesThePickerBackToTheTree(t *testing.T) {
	m := typeInto(t, wikiTestModel(t, wikiTestConfig()), "  ")
	if m = update(t, m, tea.KeyMsg{Type: tea.KeyEsc}); m.mode != ModeWiki {
		t.Fatalf("mode = %v, want ModeWiki", m.mode)
	}
}

func TestSpaceSGReadsEveryPageAndGrepsTheirLines(t *testing.T) {
	m := wikiTestModel(t, wikiTestConfig())
	m = pressKey(t, pressKey(t, m, " "), "s")
	got, cmd := m.Update(keyMsgFor("g"))
	m = got.(Model)
	if m.mode != ModeWikiPicker || m.wiki.picker.kind != wikiPickerGrep || cmd == nil {
		t.Fatalf("mode = %v, kind = %v, cmd = %v; want grep fetching the pages", m.mode, m.wiki.picker.kind, cmd)
	}
	if len(m.wiki.loadingPages) != 5 {
		t.Fatalf("loading %d pages, want all 5", len(m.wiki.loadingPages))
	}

	id := wikiID(m.config.Wikis[0])
	m = update(t, m, wikiPageLoadedMsg{key: wikiPageKey{wiki: id, path: "/Krav/Pay later"}, content: azuredevops.WikiPageContent{Content: "intro\n  Pay with Klarna later\nnothing"}})
	m = update(t, m, wikiPageLoadedMsg{key: wikiPageKey{wiki: id, path: "/Krav/Insurify/SIE"}, content: azuredevops.WikiPageContent{Content: "klarna export"}})
	m = typeInto(t, m, "klarna")

	results := m.wiki.picker.results
	if len(results) != 2 {
		t.Fatalf("results = %+v, want both lines mentioning klarna", results)
	}
	if results[1].line != 1 || !strings.Contains(m.renderWikiPicker(), "Pay later:2:") {
		t.Fatalf("results = %+v, want the hit on line 2 of Pay later shown as such", results)
	}
}

func TestGrepIsCaseSensitiveOnlyWithUpperCase(t *testing.T) {
	if start, _ := smartCaseIndex("Pay with Klarna", "klarna"); start != 9 {
		t.Fatalf("lower case query start = %d, want 9", start)
	}
	if start, _ := smartCaseIndex("pay with klarna", "Klarna"); start != -1 {
		t.Fatalf("upper case query start = %d, want no match", start)
	}
	if start, length := smartCaseIndex("ÖPPNA frågor", "öppna"); start != 0 || length != len("ÖPPNA") {
		t.Fatalf("start, length = %d, %d; want ÖPPNA matched ignoring case", start, length)
	}
	if start, length := smartCaseIndex("öppna frågor", "frå"); start != len("öppna ") || length != len("frå") {
		t.Fatalf("start, length = %d, %d; want frå matched past the å", start, length)
	}
}

func TestShiftRTogglesWikiWatch(t *testing.T) {
	m := pressKey(t, wikiTestModel(t, wikiTestConfig()), "R")
	if !m.wiki.watch {
		t.Fatal("watch off after R, want on")
	}
	if m.liveReload {
		t.Fatal("board live reload turned on, want wiki watch kept separate")
	}

	_, cmd := m.Update(wikiWatchTickMsg{generation: m.wiki.watchGeneration})
	if cmd == nil {
		t.Fatal("watch tick did nothing, want a refresh")
	}
	if m = pressKey(t, m, "R"); m.wiki.watch {
		t.Fatal("watch on after second R, want off")
	}
	if _, cmd := m.Update(wikiWatchTickMsg{generation: m.wiki.watchGeneration - 1}); cmd != nil {
		t.Fatal("stale watch tick refreshed, want it dropped")
	}
}

func TestWatchRefreshKeepsTheMessageAndCachedPreview(t *testing.T) {
	m := pressKey(t, wikiTestModel(t, wikiTestConfig()), "R")
	key := wikiPageKey{wiki: wikiID(m.config.Wikis[0]), path: "/Krav"}
	m.wiki.pages[key] = azuredevops.WikiPageContent{Content: "old"}
	m.message = "keep me"

	m.refreshWikisInBackground()
	m = update(t, m, wikiTreeLoadedMsg{wiki: key.wiki, root: wikiTestTree(), background: true})

	if m.message != "keep me" || m.wiki.pages[key].Content != "old" {
		t.Fatalf("message = %q, page = %q; want both kept until new content arrives", m.message, m.wiki.pages[key].Content)
	}
	if !m.wiki.loadingPages[key] {
		t.Fatal("cached page not refetched by the watch refresh")
	}
}

func TestPageFinderRanksTitleMatchesFirst(t *testing.T) {
	candidates := []wikiPickerResult{
		{key: wikiPageKey{path: "/Krav/Sanktion i appen"}, label: "Krav › Sanktion i appen"},
		{key: wikiPageKey{path: "/Krav/Insurify/SIE Filer"}, label: "Krav › Insurify › SIE Filer"},
	}
	got := findWikiPages(candidates, "sie")
	if len(got) != 2 || got[0].label != "Krav › Insurify › SIE Filer" {
		t.Fatalf("results = %+v, want the page titled SIE first", got)
	}
	if want := byteRange(len("Krav › Insurify › "), len("SIE")); !slices.Equal(got[0].matches, want) {
		t.Fatalf("matches = %v, want the title's SIE highlighted %v", got[0].matches, want)
	}
}
