package azuredevops

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// markdownToHTML converts the markdown subset htmlToMarkdown produces back
// to HTML for System.Description, so editing a work item in $EDITOR keeps
// its formatting: paragraphs (single newlines become <br>), headings,
// bullet and numbered lists, fenced code, bold, italic, inline code, links
// and bare URLs.
var (
	markdownHeadingRegex  = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	markdownBulletRegex   = regexp.MustCompile(`^[-*+]\s+(.*)$`)
	markdownNumberedRegex = regexp.MustCompile(`^(\d+)\.\s+(.*)$`)
	markdownCodeRegex     = regexp.MustCompile("`([^`]+)`")
	markdownLinkRegex     = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	markdownBareURLRegex  = regexp.MustCompile(`(^|[\s(])(https?://[^\s<)]+)`)
	markdownBoldRegex     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	markdownItalicRegex   = regexp.MustCompile(`\*([^*]+)\*`)
)

const (
	listBullet   = "ul"
	listNumbered = "ol"

	firstListNumber = "1"

	codeLanguageClassPrefix = "language-"
)

type markdownHTMLWriter struct {
	out       strings.Builder
	paragraph []string
	list      string
	code      []string
	codeLang  string
	inCode    bool
}

func markdownToHTML(markdown string) string {
	w := &markdownHTMLWriter{}
	for _, line := range strings.Split(strings.ReplaceAll(markdown, "\r", ""), "\n") {
		w.writeLine(line)
	}
	if w.inCode {
		w.flushCode()
	}
	w.flushParagraph()
	w.closeList()
	return w.out.String()
}

func (w *markdownHTMLWriter) writeLine(line string) {
	trimmed := strings.TrimSpace(line)
	if w.inCode {
		if strings.HasPrefix(trimmed, codeFenceMarker) {
			w.flushCode()
			return
		}
		w.code = append(w.code, line)
		return
	}

	switch {
	case strings.HasPrefix(trimmed, codeFenceMarker):
		w.flushParagraph()
		w.closeList()
		w.inCode = true
		w.codeLang = strings.TrimSpace(strings.TrimPrefix(trimmed, codeFenceMarker))
	case trimmed == "":
		w.flushParagraph()
		w.closeList()
	case markdownHeadingRegex.MatchString(trimmed):
		w.flushParagraph()
		w.closeList()
		parts := markdownHeadingRegex.FindStringSubmatch(trimmed)
		level := len(parts[1])
		fmt.Fprintf(&w.out, "<h%d>%s</h%d>", level, markdownInlineToHTML(parts[2]), level)
	case markdownBulletRegex.MatchString(trimmed):
		w.writeListItem(listBullet, "", markdownBulletRegex.FindStringSubmatch(trimmed)[1])
	case markdownNumberedRegex.MatchString(trimmed):
		parts := markdownNumberedRegex.FindStringSubmatch(trimmed)
		w.writeListItem(listNumbered, parts[1], parts[2])
	default:
		w.closeList()
		w.paragraph = append(w.paragraph, markdownInlineToHTML(trimmed))
	}
}

// writeListItem opens a list when the kind changes. A numbered list that
// resumes after nested bullets carries its first number as start, so its
// numbering doesn't restart at 1.
func (w *markdownHTMLWriter) writeListItem(list, number, content string) {
	w.flushParagraph()
	if w.list != list {
		w.closeList()
		w.list = list
		if number != "" && number != firstListNumber {
			fmt.Fprintf(&w.out, `<%s start="%s">`, list, number)
		} else {
			fmt.Fprintf(&w.out, "<%s>", list)
		}
	}
	fmt.Fprintf(&w.out, "<li>%s</li>", markdownInlineToHTML(content))
}

func (w *markdownHTMLWriter) flushParagraph() {
	if len(w.paragraph) == 0 {
		return
	}
	fmt.Fprintf(&w.out, "<div>%s</div>", strings.Join(w.paragraph, "<br>"))
	w.paragraph = nil
}

func (w *markdownHTMLWriter) closeList() {
	if w.list == "" {
		return
	}
	fmt.Fprintf(&w.out, "</%s>", w.list)
	w.list = ""
}

// flushCode writes a fenced block as <pre>, keeping a fence language such as
// ```ts as a language- class so it survives the round trip.
func (w *markdownHTMLWriter) flushCode() {
	code := html.EscapeString(strings.Join(w.code, "\n"))
	if w.codeLang != "" {
		fmt.Fprintf(&w.out, `<pre><code class="%s%s">%s</code></pre>`, codeLanguageClassPrefix, html.EscapeString(w.codeLang), code)
	} else {
		fmt.Fprintf(&w.out, "<pre>%s</pre>", code)
	}
	w.code, w.codeLang, w.inCode = nil, "", false
}

// markdownInlineToHTML escapes the text, then applies inline formatting.
// Bare URLs are linked after labelled links, and only when preceded by
// whitespace or "(", so URLs already inside an <a> tag are left alone.
func markdownInlineToHTML(text string) string {
	s := html.EscapeString(text)
	s = markdownCodeRegex.ReplaceAllString(s, "<code>$1</code>")
	s = markdownLinkRegex.ReplaceAllString(s, `<a href="$2">$1</a>`)
	s = markdownBareURLRegex.ReplaceAllString(s, `$1<a href="$2">$2</a>`)
	s = markdownBoldRegex.ReplaceAllString(s, "<strong>$1</strong>")
	return markdownItalicRegex.ReplaceAllString(s, "<em>$1</em>")
}
