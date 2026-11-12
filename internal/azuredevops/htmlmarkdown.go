package azuredevops

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

// ADO stores System.Description and older comments as HTML, while fields such
// as repro steps and newer comments are markdown. htmlToMarkdown turns the
// HTML into markdown the card view can render, and leaves markdown as is.
var (
	htmlTagRegex          = regexp.MustCompile(`<[^>]+>`)
	htmlLooksLikeRegex    = regexp.MustCompile(`(?i)<(?:/?[a-z][a-z0-9]*)(?:\s[^>]*)?/?>`)
	htmlLinkRegex         = regexp.MustCompile(`(?is)<a\s[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	htmlBreakRegex        = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlListItemRegex     = regexp.MustCompile(`(?i)<li(?:\s[^>]*)?>`)
	htmlOrderedList       = regexp.MustCompile(`(?is)<ol(\s[^>]*)?>(.*?)</ol>`)
	htmlListStartRegex    = regexp.MustCompile(`(?i)start="(\d+)"`)
	htmlListEndRegex      = regexp.MustCompile(`(?i)</li>`)
	htmlListRegex         = regexp.MustCompile(`(?i)</?(?:ul|ol)(?:\s[^>]*)?>`)
	htmlHeadingRegex      = regexp.MustCompile(`(?i)<h([1-6])(?:\s[^>]*)?>`)
	htmlHeadingEnd        = regexp.MustCompile(`(?i)</h[1-6]>`)
	htmlBoldRegex         = regexp.MustCompile(`(?i)</?(?:strong|b)(?:\s[^>]*)?>`)
	htmlItalicRegex       = regexp.MustCompile(`(?i)</?(?:em|i)(?:\s[^>]*)?>`)
	htmlCodeRegex         = regexp.MustCompile(`(?i)</?code(?:\s[^>]*)?>`)
	htmlPreBlockRegex     = regexp.MustCompile(`(?is)<pre(?:\s[^>]*)?>(.*?)</pre>`)
	htmlCodeLanguageRegex = regexp.MustCompile(`(?i)<code[^>]*class="[^"]*language-([\w+-]+)`)
	numberedItemLine      = regexp.MustCompile(`^(\d+)\. (.*)$`)
	htmlBlockRegex        = regexp.MustCompile(`(?i)</?(?:div|p|tr|table|tbody|blockquote)(?:\s[^>]*)?>`)
	extraBlankLines       = regexp.MustCompile(`\n{3,}`)
	listMarkerSpacing     = regexp.MustCompile(`^(-|\d+\.)\s+`)
	listItemLine          = regexp.MustCompile(`^(?:-|\d+\.) `)
	looseListItems        = regexp.MustCompile(`(?m)(^(?:-|\d+\.) .*)\n\n((?:-|\d+\.) )`)
)

const (
	markdownBold       = "**"
	markdownItalic     = "*"
	markdownCode       = "`"
	codeFenceMarker    = "```"
	markdownBullet     = "\n- "
	nonBreakingSpace   = " "
	markdownLinkFormat = "[%s](%s)"
)

func htmlToMarkdown(s string) string {
	if !htmlLooksLikeRegex.MatchString(s) {
		return tidyMarkdown(html.UnescapeString(s))
	}

	// <pre> blocks keep their line breaks, so they're converted separately
	// from the surrounding HTML, where raw newlines are just whitespace.
	var b strings.Builder
	last := 0
	for _, loc := range htmlPreBlockRegex.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(htmlSegmentToMarkdown(s[last:loc[0]]))
		b.WriteString(preToCodeFence(s[loc[2]:loc[3]]))
		last = loc[1]
	}
	b.WriteString(htmlSegmentToMarkdown(s[last:]))
	return tidyMarkdown(b.String())
}

func htmlSegmentToMarkdown(s string) string {
	s = strings.NewReplacer("\r", "", "\n", " ").Replace(s)
	s = htmlLinkRegex.ReplaceAllStringFunc(s, markdownLink)
	s = htmlBreakRegex.ReplaceAllString(s, "\n")
	s = htmlOrderedList.ReplaceAllStringFunc(s, numberListItems)
	s = htmlListItemRegex.ReplaceAllString(s, markdownBullet)
	s = htmlListEndRegex.ReplaceAllString(s, "")
	s = htmlListRegex.ReplaceAllString(s, "\n")
	s = htmlHeadingRegex.ReplaceAllStringFunc(s, func(tag string) string {
		level := htmlHeadingRegex.FindStringSubmatch(tag)[1][0] - '0'
		return "\n" + strings.Repeat("#", int(level)) + " "
	})
	s = htmlHeadingEnd.ReplaceAllString(s, "\n")
	s = htmlBoldRegex.ReplaceAllString(s, markdownBold)
	s = htmlItalicRegex.ReplaceAllString(s, markdownItalic)
	s = htmlCodeRegex.ReplaceAllString(s, markdownCode)
	s = htmlBlockRegex.ReplaceAllString(s, "\n")
	s = htmlTagRegex.ReplaceAllString(s, "")
	return html.UnescapeString(s)
}

// preToCodeFence turns a <pre> block (often wrapping <code> and styled
// <span>s) into a fenced code block with its line breaks intact.
func preToCodeFence(content string) string {
	language := ""
	if match := htmlCodeLanguageRegex.FindStringSubmatch(content); match != nil {
		language = match[1]
	}
	content = htmlBreakRegex.ReplaceAllString(content, "\n")
	content = html.UnescapeString(htmlTagRegex.ReplaceAllString(content, ""))
	content = strings.Trim(strings.ReplaceAll(content, nonBreakingSpace, " "), "\n")
	return "\n" + codeFenceMarker + language + "\n" + content + "\n" + codeFenceMarker + "\n"
}

// numberListItems turns an <ol>'s items into "1. ", "2. ", ... lines; the
// generic <li> handling would otherwise make them bullets.
func numberListItems(list string) string {
	parts := htmlOrderedList.FindStringSubmatch(list)
	attributes, items := parts[1], parts[2]
	number := 0
	if start := htmlListStartRegex.FindStringSubmatch(attributes); start != nil {
		number, _ = strconv.Atoi(start[1])
		number--
	}
	return "\n" + htmlListItemRegex.ReplaceAllStringFunc(items, func(string) string {
		number++
		return fmt.Sprintf("\n%d. ", number)
	}) + "\n"
}

// markdownLink keeps bare URLs bare and turns labelled links into
// [label](url).
func markdownLink(anchor string) string {
	parts := htmlLinkRegex.FindStringSubmatch(anchor)
	href := html.UnescapeString(parts[1])
	label := strings.TrimSpace(html.UnescapeString(htmlTagRegex.ReplaceAllString(parts[2], "")))
	if label == "" || label == href {
		return href
	}
	return fmt.Sprintf(markdownLinkFormat, label, href)
}

// tidyMarkdown normalises markdown into one layout, so text read from ADO
// and text that went through markdownToHTML and back come out identical:
// trimmed lines (except inside code fences), single spaces after list
// markers, bare URLs for self-labelled links, a blank line around headings,
// code fences and the start of a list, and tight lists.
func tidyMarkdown(s string) string {
	s = strings.ReplaceAll(s, nonBreakingSpace, " ")
	var out []string
	inFence, blankNext := false, false
	addBlank := func() {
		if len(out) > 0 && out[len(out)-1] != "" {
			out = append(out, "")
		}
	}
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		if inFence {
			if strings.HasPrefix(line, codeFenceMarker) {
				out = append(out, codeFenceMarker)
				inFence, blankNext = false, true
				continue
			}
			out = append(out, strings.TrimRight(raw, " \t"))
			continue
		}
		if line == "" {
			out = append(out, "")
			continue
		}
		line = listMarkerSpacing.ReplaceAllString(line, "$1 ")
		line = markdownSelfLinks(line)
		line = continueNumbering(line, out)
		isList := listItemLine.MatchString(line)
		prevIsList := len(out) > 0 && listItemLine.MatchString(out[len(out)-1])
		prevIsHeading := len(out) > 0 && strings.HasPrefix(out[len(out)-1], "#")
		switch {
		case strings.HasPrefix(line, codeFenceMarker):
			addBlank()
			inFence = true
		case blankNext, prevIsHeading, strings.HasPrefix(line, "#"), isList && !prevIsList:
			addBlank()
		}
		blankNext = false
		out = append(out, line)
	}
	s = extraBlankLines.ReplaceAllString(strings.Join(out, "\n"), "\n\n")
	s = tightenLists(s)
	return strings.TrimSpace(s)
}

// continueNumbering renumbers a numbered item that directly follows another
// one, so "1. a / 1. b" auto-numbering reads as "1. a / 2. b", the way it
// renders and the way it reads back after a save.
func continueNumbering(line string, out []string) string {
	item := numberedItemLine.FindStringSubmatch(line)
	if item == nil || len(out) == 0 {
		return line
	}
	previous := numberedItemLine.FindStringSubmatch(out[len(out)-1])
	if previous == nil {
		return line
	}
	number, _ := strconv.Atoi(previous[1])
	return fmt.Sprintf("%d. %s", number+1, item[2])
}

// markdownSelfLinks turns [url](url) into the bare url, which is how
// markdownToHTML's linked bare URLs read back.
func markdownSelfLinks(line string) string {
	return markdownLinkRegex.ReplaceAllStringFunc(line, func(link string) string {
		parts := markdownLinkRegex.FindStringSubmatch(link)
		if parts[1] == parts[2] {
			return parts[2]
		}
		return link
	})
}

// tightenLists removes blank lines between consecutive list items, numbered
// or bulleted, since ADO interleaves them for nested points. Matches can't
// overlap, so it repeats until every gap is closed.
func tightenLists(s string) string {
	for tightened := looseListItems.ReplaceAllString(s, "$1\n$2"); tightened != s; tightened = looseListItems.ReplaceAllString(s, "$1\n$2") {
		s = tightened
	}
	return s
}
