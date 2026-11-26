package ui

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"

	"cmdban/internal/azuredevops"
)

type wikiPickerKind int

const (
	wikiPickerFindPages wikiPickerKind = iota
	wikiPickerGrep
	wikiPickerFavorites
)

const (
	wikiLeaderKey = " "
	// wikiGrepFetchLimit caps how many pages grep fetches at once while it
	// reads every page.
	wikiGrepFetchLimit = 6
	// wikiGrepMaxResults keeps a very common query from listing every line
	// of the wiki.
	wikiGrepMaxResults = 500
	// wikiRevealContextRows is how many lines stay visible above a grep hit
	// when it is opened in the page view.
	wikiRevealContextRows = 2
	wikiLeaderHint        = "space: <space> find pages  <s g> grep page contents"
	wikiLeaderGrepHint    = "space s: <g> grep page contents"
)

// wikiLeaderSequences are the space-led key sequences of wiki mode, like
// telescope's leader mappings in nvim.
var wikiLeaderSequences = map[string]wikiPickerKind{
	wikiLeaderKey + " ":  wikiPickerFindPages,
	wikiLeaderKey + "sg": wikiPickerGrep,
}

// wikiPickerResult is one line of a picker: a page, or for grep, one
// matching line of a page. matches are byte offsets into the highlighted
// text: the label for pages, the line text for grep hits.
type wikiPickerResult struct {
	wikiIndex int
	key       wikiPageKey
	label     string
	line      int
	text      string
	matches   []int
}

type wikiPicker struct {
	kind          wikiPickerKind
	results       []wikiPickerResult
	selected      int
	previewScroll int
	returnMode    Mode
}

// handleWikiLeader follows space-led sequences, reporting whether msg was
// part of one. A key that continues no sequence cancels the sequence.
func (m Model) handleWikiLeader(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if m.wiki.leader == "" {
		if msg.String() != wikiLeaderKey {
			return m, nil, false
		}
		m.wiki.leader = wikiLeaderKey
		m.message = wikiLeaderHint
		return m, nil, true
	}

	sequence := m.wiki.leader + msg.String()
	m.wiki.leader = ""
	m.message = ""
	if kind, ok := wikiLeaderSequences[sequence]; ok {
		model, cmd := m.openWikiPicker(kind)
		return model, cmd, true
	}
	for known := range wikiLeaderSequences {
		if strings.HasPrefix(known, sequence) {
			m.wiki.leader = sequence
			m.message = wikiLeaderGrepHint
			break
		}
	}
	return m, nil, true
}

func (m Model) openWikiPicker(kind wikiPickerKind) (tea.Model, tea.Cmd) {
	m.wiki.picker = wikiPicker{kind: kind, returnMode: m.mode}
	m.mode = ModeWikiPicker
	m.textInput.Reset()
	m.textInput.Placeholder = m.wikiPickerPlaceholder()
	m.textInput.EchoMode = textinput.EchoNormal
	m.textInput.Focus()
	m.refreshWikiPicker()

	cmds := []tea.Cmd{textinput.Blink}
	if kind == wikiPickerGrep {
		limit := make(chan struct{}, wikiGrepFetchLimit)
		for _, page := range m.wikiPageCandidates() {
			cmds = append(cmds, m.ensureWikiPage(page.wikiIndex, page.key, limit))
		}
	} else {
		cmds = append(cmds, m.loadPickerPreview())
	}
	return m, tea.Batch(cmds...)
}

func (m Model) wikiPickerPlaceholder() string {
	switch m.wiki.picker.kind {
	case wikiPickerGrep:
		return "grep page contents..."
	case wikiPickerFavorites:
		return "find favorite pages..."
	}
	return "find pages..."
}

func (m Model) closeWikiPicker() (tea.Model, tea.Cmd) {
	m.mode = m.wiki.picker.returnMode
	m.textInput.Reset()
	m.wiki.picker = wikiPicker{}
	return m, nil
}

// wikiPageCandidates is every loaded page of every wiki, in tree order,
// labelled with its breadcrumb.
func (m Model) wikiPageCandidates() []wikiPickerResult {
	var candidates []wikiPickerResult
	for i, w := range m.config.Wikis {
		if root := m.wiki.trees[wikiID(w)].root; root != nil {
			candidates = m.appendWikiCandidates(candidates, i, wikiID(w), *root)
		}
	}
	return candidates
}

func (m Model) appendWikiCandidates(candidates []wikiPickerResult, wikiIndex int, wiki string, page azuredevops.WikiPage) []wikiPickerResult {
	key := wikiPageKey{wiki: wiki, path: page.Path}
	candidates = append(candidates, wikiPickerResult{
		wikiIndex: wikiIndex,
		key:       key,
		label:     m.wikiBreadcrumb(wikiRow{wikiIndex: wikiIndex, key: key}),
	})
	for _, sub := range page.SubPages {
		candidates = m.appendWikiCandidates(candidates, wikiIndex, wiki, sub)
	}
	return candidates
}

// wikiPickerCandidates is the pages the picker searches: the favorite ones
// for the favorites picker, every loaded page otherwise.
func (m Model) wikiPickerCandidates() []wikiPickerResult {
	candidates := m.wikiPageCandidates()
	if m.wiki.picker.kind != wikiPickerFavorites {
		return candidates
	}
	return slices.DeleteFunc(candidates, func(c wikiPickerResult) bool {
		return !m.config.Wikis[c.wikiIndex].IsFavorite(c.key.path)
	})
}

// refreshWikiPicker reruns the picker's search for the typed query.
func (m *Model) refreshWikiPicker() {
	query := m.textInput.Value()
	candidates := m.wikiPickerCandidates()
	if m.wiki.picker.kind == wikiPickerGrep {
		m.wiki.picker.results = m.grepWikiPages(candidates, query)
	} else {
		m.wiki.picker.results = findWikiPages(candidates, query)
	}
	m.wiki.picker.selected = max(min(m.wiki.picker.selected, len(m.wiki.picker.results)-1), 0)
}

// findWikiPages fuzzy matches query against the breadcrumbs. Pages whose
// own title contains query come first, then pages with query anywhere in
// their breadcrumb, then the remaining fuzzy matches, each best first. An
// empty query lists every page in tree order.
func findWikiPages(candidates []wikiPickerResult, query string) []wikiPickerResult {
	if query == "" {
		return candidates
	}
	labels := make([]string, len(candidates))
	for i, c := range candidates {
		labels[i] = c.label
	}
	var inTitle, inBreadcrumb, fuzzyOnly []wikiPickerResult
	for _, match := range fuzzy.Find(query, labels) {
		result := candidates[match.Index]
		result.matches = match.MatchedIndexes
		titleStart := len(result.label) - len(azuredevops.WikiPageTitle(result.key.path))
		if start, length := smartCaseIndex(result.label[titleStart:], query); start >= 0 {
			result.matches = byteRange(titleStart+start, length)
			inTitle = append(inTitle, result)
		} else if start, length := smartCaseIndex(result.label, query); start >= 0 {
			result.matches = byteRange(start, length)
			inBreadcrumb = append(inBreadcrumb, result)
		} else {
			fuzzyOnly = append(fuzzyOnly, result)
		}
	}
	return slices.Concat(inTitle, inBreadcrumb, fuzzyOnly)
}

// grepWikiPages lists every line of the fetched pages containing query,
// ignoring case unless query has an upper case letter (like rg's
// --smart-case).
func (m Model) grepWikiPages(candidates []wikiPickerResult, query string) []wikiPickerResult {
	if query == "" {
		return nil
	}
	var results []wikiPickerResult
	for _, page := range candidates {
		content, ok := m.wiki.pages[page.key]
		if !ok {
			continue
		}
		for i, line := range strings.Split(content.Content, "\n") {
			line = strings.ReplaceAll(line, "\t", " ")
			start, length := smartCaseIndex(line, query)
			if start < 0 {
				continue
			}
			hit := page
			hit.line = i
			hit.text = line
			hit.matches = byteRange(start, length)
			results = append(results, hit)
			if len(results) == wikiGrepMaxResults {
				return results
			}
		}
	}
	return results
}

// smartCaseIndex finds query in s, ignoring case unless query has an upper
// case letter, and returns the byte offset and length of the match in s,
// or -1 when there is none.
func smartCaseIndex(s, query string) (start, length int) {
	if strings.IndexFunc(query, unicode.IsUpper) >= 0 {
		return strings.Index(s, query), len(query)
	}
	for i := range s {
		if length, ok := foldedPrefixLength(s[i:], query); ok {
			return i, length
		}
	}
	return -1, 0
}

// foldedPrefixLength reports whether s starts with query ignoring case, and
// how many bytes of s that prefix takes.
func foldedPrefixLength(s, query string) (int, bool) {
	length := 0
	for _, want := range query {
		got, size := utf8.DecodeRuneInString(s[length:])
		if size == 0 || !strings.EqualFold(string(got), string(want)) {
			return 0, false
		}
		length += size
	}
	return length, true
}

func byteRange(start, length int) []int {
	indexes := make([]int, length)
	for i := range indexes {
		indexes[i] = start + i
	}
	return indexes
}

// wikiGrepProgress reports how many of the pages grep searches have been
// fetched.
func (m Model) wikiGrepProgress() (fetched, total int) {
	for _, page := range m.wikiPageCandidates() {
		total++
		if _, ok := m.wiki.pages[page.key]; ok || m.wiki.pageErrors[page.key] != nil {
			fetched++
		}
	}
	return fetched, total
}

func (m Model) selectedPickerResult() (wikiPickerResult, bool) {
	results := m.wiki.picker.results
	if m.wiki.picker.selected < 0 || m.wiki.picker.selected >= len(results) {
		return wikiPickerResult{}, false
	}
	return results[m.wiki.picker.selected], true
}

func (m Model) loadPickerPreview() tea.Cmd {
	result, ok := m.selectedPickerResult()
	if !ok {
		return nil
	}
	return m.ensureWikiPage(result.wikiIndex, result.key, nil)
}

func (m Model) movePickerSelection(delta int) (tea.Model, tea.Cmd) {
	m.wiki.picker.selected = max(min(m.wiki.picker.selected+delta, len(m.wiki.picker.results)-1), 0)
	m.wiki.picker.previewScroll = 0
	return m, m.loadPickerPreview()
}

// openPickerResult reveals the chosen page in the tree and reads it; a grep
// hit opens scrolled to the matching line.
func (m Model) openPickerResult() (tea.Model, tea.Cmd) {
	result, ok := m.selectedPickerResult()
	if !ok {
		return m, nil
	}
	query := m.textInput.Value()
	grep := m.wiki.picker.kind == wikiPickerGrep
	m.textInput.Reset()
	m.wiki.picker = wikiPicker{}
	m.revealWikiPage(result.key)
	m.mode = ModeWikiView
	m.wiki.scroll = 0
	if grep {
		m.wiki.scroll = m.renderedLineOf(result.key, query)
	}
	return m.loadSelectedWikiPage()
}

// revealWikiPage opens every page above key in the tree and selects it.
func (m *Model) revealWikiPage(key wikiPageKey) {
	for _, w := range m.config.Wikis {
		root := m.wiki.trees[wikiID(w)].root
		if wikiID(w) != key.wiki || root == nil {
			continue
		}
		for _, parent := range wikiParentKeys(key.wiki, *root) {
			if parent.path != key.path && strings.HasPrefix(key.path, strings.TrimRight(parent.path, azuredevops.WikiPathSeparator)+azuredevops.WikiPathSeparator) {
				m.wiki.expanded[parent] = true
			}
		}
	}
	for i, row := range m.wikiRows() {
		if row.key == key {
			m.wiki.selected = i
			return
		}
	}
}

// renderedLineOf finds the first line of the page as the page view renders
// it that contains query, so a grep hit opens at the hit; markdown
// rendering changes line breaks, so the raw line number can't be used.
func (m Model) renderedLineOf(key wikiPageKey, query string) int {
	content, ok := m.wiki.pages[key]
	if !ok {
		return 0
	}
	for i, line := range m.markdownLines(content.Content, m.width-detailPaneContentInsetCols) {
		if start, _ := smartCaseIndex(ansi.Strip(line), query); start >= 0 {
			return max(i-wikiRevealContextRows, 0)
		}
	}
	return 0
}

// handleWikiPickerMode leaves letters to the query; results are walked with
// the arrows or ctrl+n/p (ctrl+j/k), and ctrl+u/d scroll the preview, as in
// telescope.
func (m Model) handleWikiPickerMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		return m.closeWikiPicker()
	case "enter":
		return m.openPickerResult()
	case "down", "ctrl+n", "ctrl+j":
		return m.movePickerSelection(1)
	case "up", "ctrl+p", "ctrl+k":
		return m.movePickerSelection(-1)
	case "ctrl+d":
		m.wiki.picker.previewScroll += wikiScrollJump
		return m, nil
	case "ctrl+u":
		m.wiki.picker.previewScroll = max(m.wiki.picker.previewScroll-wikiScrollJump, 0)
		return m, nil
	}

	previous := m.textInput.Value()
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	if m.textInput.Value() == previous {
		return m, cmd
	}
	m.wiki.picker.selected = 0
	m.wiki.picker.previewScroll = 0
	m.refreshWikiPicker()
	return m, tea.Batch(cmd, m.loadPickerPreview())
}

func (m Model) wikiPickerTitle() string {
	if m.wiki.picker.kind == wikiPickerGrep {
		fetched, total := m.wikiGrepProgress()
		if fetched < total {
			return fmt.Sprintf("Grep Wiki (reading pages %d/%d)", fetched, total)
		}
		return "Grep Wiki"
	}
	if m.wiki.picker.kind == wikiPickerFavorites {
		return "Favorite Pages"
	}
	return "Find Pages"
}
