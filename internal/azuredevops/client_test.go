package azuredevops

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDedupeIntsRemovesDuplicatesPreservingOrder(t *testing.T) {
	got := dedupeInts([]int{1, 2, 2, 3, 1, 4})
	want := []int{1, 2, 3, 4}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dedupeInts() = %v, want %v", got, want)
	}
}

func TestDedupeIntsEmpty(t *testing.T) {
	got := dedupeInts(nil)
	if len(got) != 0 {
		t.Fatalf("dedupeInts(nil) = %v, want empty", got)
	}
}

func TestHtmlizeTextEscapesAndAddsLineBreaks(t *testing.T) {
	got := htmlizeText("Steps:\n1. a < b\n2. done & dusted")
	want := "Steps:<br>\n1. a &lt; b<br>\n2. done &amp; dusted"
	if got != want {
		t.Fatalf("htmlizeText() = %q, want %q", got, want)
	}
}

func TestStripHTMLRemovesTagsAndUnescapesEntities(t *testing.T) {
	got := stripHTML("<div>Looks good &amp; ready &mdash; <b>ship it</b></div>")
	want := "Looks good & ready — ship it"
	if got != want {
		t.Fatalf("stripHTML() = %q, want %q", got, want)
	}
}

func TestCommentListResponseUnmarshal(t *testing.T) {
	raw := `{
		"count": 1,
		"comments": [
			{
				"id": 7,
				"text": "<p>Looks good</p>",
				"createdBy": {"displayName": "Jane Doe", "id": "abc-123"},
				"createdDate": "2026-08-11T10:00:00Z"
			}
		]
	}`

	var resp CommentListResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := CommentListResponse{
		Count: 1,
		Comments: []Comment{
			{
				ID:          7,
				Text:        "<p>Looks good</p>",
				CreatedBy:   IdentityRef{DisplayName: "Jane Doe", ID: "abc-123"},
				CreatedDate: "2026-08-11T10:00:00Z",
			},
		},
	}
	if !reflect.DeepEqual(resp, want) {
		t.Fatalf("resp = %+v, want %+v", resp, want)
	}
}
