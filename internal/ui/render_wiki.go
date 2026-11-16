package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"cmdban/internal/azuredevops"
)

const (
	wikiListTitle = "WIKIS"
	// wikiPreviewListShare is the part of the width the tree keeps while
	// the preview is open, as numerator over denominator.
	wikiPreviewListShareNum = 2
	wikiPreviewListShareDen = 5
	wikiListMinWidth        = 30
	wikiPageMinWidth        = 30
	wikiListChromeCols      = 4 // lane border and padding
	wikiScrollIndicatorRows = 2

	wikiExpandedMarker  = "▾ "
	wikiCollapsedMarker = "▸ "
	wikiLeafMarker      = "· "
	wikiGuideBranch     = "├─ "
	wikiGuideLastBranch = "└─ "
	wikiGuideLine       = "│  "
	wikiGuideBlank      = "   "
	wikiBreadcrumbSep   = " › "

	wikiPreviewFooterHint = "[tab: close preview  enter: read  ctrl+u/ctrl+f: scroll  ctrl+g: edit in nvim  ctrl+o: open in browser]"
	wikiViewFooterHint    = "[q: back  j/k: scroll  ctrl+g: edit in nvim  M: markdown  ctrl+o: open in browser  r: refresh]"
	wikiEmptyHint         = "No wikis yet — press a and paste a wiki page URL"
	wikiEmptyPage         = "*(empty page)*"
	wikiLoadingText       = "Loading…"
)

func (m Model) renderWiki() string {
	if !m.wiki.preview {
		return m.renderWikiList(m.width)
	}
	listWidth := max(m.width*wikiPreviewListShareNum/wikiPreviewListShareDen, wikiListMinWidth)
	listWidth = min(listWidth, max(m.width-wikiPageMinWidth, 1))
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderWikiList(listWidth),
		m.renderWikiPagePane(m.width-listWidth, wikiPreviewFooterHint),
	)
}

func (m Model) renderWikiView() string {
	return m.renderWikiPagePane(m.width, wikiViewFooterHint)
}

// renderWikiList draws the wiki tree as a single lane, one line per page.
func (m Model) renderWikiList(width int) string {
	availableHeight := max(m.bodyHeight()-laneBorderRows, minBodyHeight)
	laneWidth := GetLaneWidth(width, 1)
	textWidth := max(laneWidth-wikiListChromeCols, 1)

	header := ActiveLaneHeaderStyle.Width(textWidth).Render(
		wikiListTitle + lipgloss.NewStyle().Foreground(SubtleColor).Render(fmt.Sprintf(" (%d)", len(m.config.Wikis))),
	)
	lines := []string{header}

	rows := m.wikiRows()
	if len(rows) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(SubtleColor).Italic(true).Width(textWidth).Render(wikiEmptyHint))
	} else {
		rendered := make([]string, len(rows))
		for i, row := range rows {
			rendered[i] = m.renderWikiRow(row, i == m.wiki.selected, textWidth)
		}
		contentHeight := max(availableHeight-lipgloss.Height(header)-wikiScrollIndicatorRows, 1)
		visible, above, below := laneScrollWindow(rendered, m.wiki.selected, contentHeight)
		indicator := lipgloss.NewStyle().Foreground(SubtleColor).Width(textWidth)
		if above > 0 {
			lines = append(lines, indicator.Render(fmt.Sprintf("  ▲ %d more", above)))
		}
		lines = append(lines, visible...)
		if below > 0 {
			lines = append(lines, indicator.Render(fmt.Sprintf("  ▼ %d more", below)))
		}
	}

	return ActiveLaneStyle.Width(laneWidth).Height(availableHeight).Render(strings.Join(lines, "\n"))
}

// renderWikiRow draws one page as a single line: tree guides showing where
// it sits, an expand marker, its title, and how many pages it holds.
func (m Model) renderWikiRow(row wikiRow, selected bool, width int) string {
	style := TaskStyle
	metaColor := SubtleColor
	if selected {
		style = SelectedTaskStyle
		metaColor = SelectedMetaColor
	}
	if row.depth() == 0 {
		style = style.Bold(true)
	}
	metaStyle := lipgloss.NewStyle().Foreground(metaColor).Background(style.GetBackground())

	title, meta := m.wikiRowText(row)
	guides := metaStyle.Render(wikiGuides(row.guides))
	line := guides + style.Inline(true).Render(m.wikiRowMarker(row)+title) + metaStyle.Render(meta)
	return style.Width(width).Render(ansi.Truncate(line, max(width-TaskStyle.GetHorizontalPadding(), 1), "…"))
}

func (m Model) wikiRowText(row wikiRow) (title, meta string) {
	if row.depth() > 0 {
		title = row.page.Title()
		if row.hasSubPages() {
			meta = fmt.Sprintf("  (%d)", len(row.page.SubPages))
		}
		return title, meta
	}

	w := m.config.Wikis[row.wikiIndex]
	tree := m.wiki.trees[row.key.wiki]
	source := fmt.Sprintf("  %s · %s/%s", w.Wiki, w.Org, w.Project)
	switch {
	case tree.loading && tree.root == nil:
		return w.Name, "  " + wikiLoadingText + source
	case tree.err != nil:
		return w.Name, "  ⚠ " + tree.err.Error()
	case tree.root == nil:
		return w.Name, source
	}
	return w.Name, fmt.Sprintf("  %d pages", tree.root.DescendantCount()) + source
}

func (m Model) wikiRowMarker(row wikiRow) string {
	switch {
	case !row.hasSubPages():
		return wikiLeafMarker
	case m.wiki.expanded[row.key]:
		return wikiExpandedMarker
	default:
		return wikiCollapsedMarker
	}
}

// wikiGuides draws the tree lines in front of a page: a continuing line for
// every ancestor level that has siblings below, and a branch for the page.
func wikiGuides(guides []bool) string {
	var b strings.Builder
	for i, last := range guides {
		switch {
		case i == len(guides)-1 && last:
			b.WriteString(wikiGuideLastBranch)
		case i == len(guides)-1:
			b.WriteString(wikiGuideBranch)
		case last:
			b.WriteString(wikiGuideBlank)
		default:
			b.WriteString(wikiGuideLine)
		}
	}
	return b.String()
}

// wikiBreadcrumb is the selected page's place in its wiki, from the
// configured wiki's name down to the page.
func (m Model) wikiBreadcrumb(row wikiRow) string {
	w := m.config.Wikis[row.wikiIndex]
	tree := m.wiki.trees[row.key.wiki]
	crumbs := []string{w.Name}
	if tree.root != nil {
		relative := strings.TrimPrefix(row.key.path, strings.TrimRight(tree.root.Path, azuredevops.WikiPathSeparator))
		for _, segment := range strings.Split(strings.Trim(relative, azuredevops.WikiPathSeparator), azuredevops.WikiPathSeparator) {
			if segment != "" {
				crumbs = append(crumbs, segment)
			}
		}
	}
	return strings.Join(crumbs, wikiBreadcrumbSep)
}

func (m Model) renderWikiPagePane(width int, footerHint string) string {
	row, ok := m.selectedWikiPage()
	if !ok {
		return DialogStyle.Render("No wiki page selected")
	}
	textWidth := width - detailPaneContentInsetCols
	return m.renderReadPane(width, m.wiki.scroll,
		row.page.Title(),
		m.wikiBreadcrumb(row),
		m.markdownLines(m.wikiPageBody(row.key), textWidth),
		footerHint,
	)
}

func (m Model) wikiPageBody(key wikiPageKey) string {
	if err := m.wiki.pageErrors[key]; err != nil {
		return "**Error:** " + err.Error()
	}
	content, ok := m.wiki.pages[key]
	switch {
	case !ok:
		return wikiLoadingText
	case strings.TrimSpace(content.Content) == "":
		return wikiEmptyPage
	}
	return content.Content
}

// buildWikiContextInfo is the topbar's CONTEXT section in wiki mode.
func (m Model) buildWikiContextInfo() []string {
	lines := []string{topBarLine("📚", "Wikis", topBarValueStyle.Render(fmt.Sprintf("%d", len(m.config.Wikis))))}
	if row, ok := m.selectedWikiRow(); ok {
		w := m.config.Wikis[row.wikiIndex]
		lines = append(lines, topBarLine("🔗", "Source", topBarValueStyle.Render(truncateName(w.Org+"/"+w.Project+"/"+w.Wiki))))
		if row.page != nil {
			lines = append(lines,
				topBarLine("📄", "Page", topBarValueStyle.Render(truncateName(row.page.Title()))),
				topBarLine("🧭", "Path", topBarValueStyle.Render(truncateName(m.wikiBreadcrumb(row)))),
				topBarLine("🌳", "Beneath", topBarValueStyle.Render(fmt.Sprintf("%d pages", row.page.DescendantCount()))),
			)
		}
	}
	return append(lines, syncedLineFor(m.wiki.lastSync, m.wiki.watch))
}
