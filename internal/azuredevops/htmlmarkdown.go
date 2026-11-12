package azuredevops

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// ADO stores System.Description and older comments as HTML, while fields such
// as repro steps and newer comments are markdown. htmlToMarkdown turns the
// HTML into markdown the card view can render, and leaves markdown as is.
var (
	htmlTagRegex       = regexp.MustCompile(`<[^>]+>`)
	htmlLooksLikeRegex = regexp.MustCompile(`(?i)<(?:/?[a-z][a-z0-9]*)(?:\s[^>]*)?/?>`)
	htmlLinkRegex      = regexp.MustCompile(`(?is)<a\s[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	htmlBreakRegex     = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlListItemRegex  = regexp.MustCompile(`(?i)<li(?:\s[^>]*)?>`)
	htmlListEndRegex   = regexp.MustCompile(`(?i)</li>`)
	htmlListRegex      = regexp.MustCompile(`(?i)</?(?:ul|ol)(?:\s[^>]*)?>`)
	htmlHeadingRegex   = regexp.MustCompile(`(?i)<h([1-6])(?:\s[^>]*)?>`)
	htmlHeadingEnd     = regexp.MustCompile(`(?i)</h[1-6]>`)
	htmlBoldRegex      = regexp.MustCompile(`(?i)</?(?:strong|b)(?:\s[^>]*)?>`)
	htmlItalicRegex    = regexp.MustCompile(`(?i)</?(?:em|i)(?:\s[^>]*)?>`)
	htmlCodeRegex      = regexp.MustCompile(`(?i)</?code(?:\s[^>]*)?>`)
	htmlPreRegex       = regexp.MustCompile(`(?i)</?pre(?:\s[^>]*)?>`)
	htmlBlockRegex     = regexp.MustCompile(`(?i)</?(?:div|p|tr|table|tbody|blockquote)(?:\s[^>]*)?>`)
	extraBlankLines    = regexp.MustCompile(`\n{3,}`)
	looseListItems     = regexp.MustCompile(`(?m)(^- .*)\n\n(- )`)
)

const (
	markdownBold       = "**"
	markdownItalic     = "*"
	markdownCode       = "`"
	markdownCodeFence  = "\n```\n"
	markdownBullet     = "\n- "
	nonBreakingSpace   = " "
	markdownLinkFormat = "[%s](%s)"
)

func htmlToMarkdown(s string) string {
	if !htmlLooksLikeRegex.MatchString(s) {
		return tidyMarkdown(html.UnescapeString(s))
	}

	// Raw newlines in HTML are just whitespace; tags carry the structure.
	s = strings.NewReplacer("\r", "", "\n", " ").Replace(s)
	s = htmlLinkRegex.ReplaceAllStringFunc(s, markdownLink)
	s = htmlBreakRegex.ReplaceAllString(s, "\n")
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
	s = htmlPreRegex.ReplaceAllString(s, markdownCodeFence)
	s = htmlBlockRegex.ReplaceAllString(s, "\n")
	s = htmlTagRegex.ReplaceAllString(s, "")
	return tidyMarkdown(html.UnescapeString(s))
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

// tidyMarkdown trims each line and collapses runs of blank lines left behind
// by nested block tags, keeping consecutive list items together.
func tidyMarkdown(s string) string {
	s = strings.ReplaceAll(s, nonBreakingSpace, " ")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	s = extraBlankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n")
	// Matches can't overlap, so repeat until every gap in a list is closed.
	for tightened := looseListItems.ReplaceAllString(s, "$1\n$2"); tightened != s; tightened = looseListItems.ReplaceAllString(s, "$1\n$2") {
		s = tightened
	}
	return strings.TrimSpace(s)
}
