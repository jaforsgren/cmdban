package azuredevops

import "testing"

func TestMarkdownToHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "paragraphs and line breaks",
			in:   "First line\nsecond line\n\nNew paragraph",
			want: "<div>First line<br>second line</div><div>New paragraph</div>",
		},
		{
			name: "escapes html in text",
			in:   "a < b & \"c\"",
			want: "<div>a &lt; b &amp; &#34;c&#34;</div>",
		},
		{
			name: "headings and lists",
			in:   "## Steps\n- one\n- two\n\n1. first\n2. second",
			want: "<h2>Steps</h2><ul><li>one</li><li>two</li></ul><ol><li>first</li><li>second</li></ol>",
		},
		{
			name: "inline formatting and links",
			in:   "**Bold** and *it* with `code`, [docs](https://example.com/a?x=1&y=2) and https://example.com/b",
			want: `<div><strong>Bold</strong> and <em>it</em> with <code>code</code>, <a href="https://example.com/a?x=1&amp;y=2">docs</a> and <a href="https://example.com/b">https://example.com/b</a></div>`,
		},
		{
			name: "code fences keep content verbatim",
			in:   "```\nif a < b {\n  **not bold**\n}\n```",
			want: "<pre>if a &lt; b {\n  **not bold**\n}</pre>",
		},
		{
			name: "empty",
			in:   "",
			want: "",
		},
	}
	for _, tt := range tests {
		if got := markdownToHTML(tt.in); got != tt.want {
			t.Errorf("%s:\n got  %q\n want %q", tt.name, got, tt.want)
		}
	}
}

// TestMarkdownSurvivesRoundTrip guards the edit flow: a description read as
// markdown, saved back as HTML and read again must come back unchanged.
func TestMarkdownSurvivesRoundTrip(t *testing.T) {
	inputs := []string{
		"Intro line\nsecond line\n\n- one\n- two",
		"## Steps\n\n1. first\n2. second\n\nDone",
		"**Bold** and *italic* with `code` and [docs](https://example.com/a)",
		"See https://example.com/b for details",
		"Price < 5 & more",
		"1. one\n2. two\n- nested\n3. three\n4. four",
		"Example:\n\n```ts\nconst a = 1 < 2;\n```\n\nAfter",
	}
	for _, in := range inputs {
		if got := htmlToMarkdown(markdownToHTML(in)); got != in {
			t.Errorf("round trip changed %q into %q", in, got)
		}
	}
}

func TestAutoNumberedListsReadSequentially(t *testing.T) {
	if got, want := htmlToMarkdown("1. a\n1. b\n1. c"), "1. a\n2. b\n3. c"; got != want {
		t.Fatalf("htmlToMarkdown() = %q, want %q", got, want)
	}
}

func TestNumberedListResumesWithStart(t *testing.T) {
	got := markdownToHTML("1. one\n- nested\n2. two")

	if want := `<ol><li>one</li></ol><ul><li>nested</li></ul><ol start="2"><li>two</li></ol>`; got != want {
		t.Fatalf("markdownToHTML() = %q, want %q", got, want)
	}
}

func TestHTMLToMarkdownNumbersOrderedLists(t *testing.T) {
	got := htmlToMarkdown("<ol><li>first</li><li>second</li></ol><ul><li>bullet</li></ul>")

	if want := "1. first\n2. second\n- bullet"; got != want {
		t.Fatalf("htmlToMarkdown() = %q, want %q", got, want)
	}
}
