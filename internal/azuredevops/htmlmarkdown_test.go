package azuredevops

import "testing"

func TestHTMLToMarkdown(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "line breaks and divs become lines",
			in:   "<div><span>First line.<br></span><br><span>- item one<br></span><div>- item two<br> </div></div>",
			want: "First line.\n\n- item one\n- item two",
		},
		{
			name: "entities and non-breaking spaces",
			in:   "<div>Looks good &amp; ready&nbsp;&mdash; <b>ship it</b></div>",
			want: "Looks good & ready — **ship it**",
		},
		{
			name: "labelled and bare links",
			in:   `<div>See <a href="https://example.com/a">the docs</a> and <a href="https://example.com/b">https://example.com/b</a></div>`,
			want: "See [the docs](https://example.com/a) and https://example.com/b",
		},
		{
			name: "lists and headings",
			in:   "<h2>Steps</h2><ul><li>One</li><li><i>Two</i></li></ul><p>Done</p>",
			want: "## Steps\n\n- One\n- *Two*\n\nDone",
		},
		{
			name: "inline code and pre blocks",
			in:   "<div>Call <code>POST /auth</code></div><pre>line 1</pre>",
			want: "Call `POST /auth`\n\n```\nline 1\n```",
		},
		{
			name: "markdown passes through with entities unescaped",
			in:   "**Källa**\n\nSee [Slack](https://slack.com) &quot;quoted&quot;\n\n\n\n1. Step",
			want: "**Källa**\n\nSee [Slack](https://slack.com) \"quoted\"\n\n1. Step",
		},
		{
			name: "long loose lists are tightened",
			in:   "<div>- a<br></div><div>- b<br></div><div>- c<br></div><div>- d</div>",
			want: "- a\n- b\n- c\n- d",
		},
		{
			name: "empty",
			in:   "",
			want: "",
		},
	}
	for _, tt := range tests {
		if got := htmlToMarkdown(tt.in); got != tt.want {
			t.Errorf("%s: htmlToMarkdown() = %q, want %q", tt.name, got, tt.want)
		}
	}
}
