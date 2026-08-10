package azuredevops

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
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

func TestBaseURLEscapesSpecialCharacters(t *testing.T) {
	c := NewClient("R&D Org", "Proj#1", "Team?A", "pat")

	got := c.baseURL()
	want := "https://dev.azure.com/R&D%20Org/Proj%231"
	if got != want {
		t.Fatalf("baseURL() = %q, want %q", got, want)
	}
}

func TestTeamURLEscapesSpecialCharacters(t *testing.T) {
	c := NewClient("R&D Org", "Proj#1", "Team?A", "pat")

	got := c.teamURL()
	want := "https://dev.azure.com/R&D%20Org/Proj%231/Team%3FA"
	if got != want {
		t.Fatalf("teamURL() = %q, want %q", got, want)
	}
}

// TestBaseURLEscapesStayValidWhenParsed guards against the actual bug: a #
// or ? in an org/project name must not be interpreted as a fragment/query
// delimiter once escaped and parsed by net/http.
func TestBaseURLEscapesStayValidWhenParsed(t *testing.T) {
	c := NewClient("org", "Proj#1?A", "team", "pat")

	parsed, err := url.Parse(c.baseURL() + "/_apis/wit/wiql?api-version=7.0")
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if parsed.Path != "/org/Proj#1?A/_apis/wit/wiql" {
		t.Fatalf("parsed.Path = %q, want the full escaped path preserved", parsed.Path)
	}
	if parsed.RawQuery != "api-version=7.0" {
		t.Fatalf("parsed.RawQuery = %q, want only the real query string", parsed.RawQuery)
	}
}

func TestClientGetDecodesSuccessResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept header = %q, want application/json", got)
		}
		w.Write([]byte(`{"count":1,"value":[{"id":"1","name":"Team A"}]}`))
	}))
	defer srv.Close()

	c := NewClient("org", "proj", "team", "pat")
	var resp TeamListResponse
	if err := c.get(srv.URL, &resp); err != nil {
		t.Fatalf("get() error = %v", err)
	}
	if len(resp.Value) != 1 || resp.Value[0].Name != "Team A" {
		t.Fatalf("resp = %+v, want one team named Team A", resp)
	}
}

func TestClientGetReturnsErrorOnHTTPFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	c := NewClient("org", "proj", "team", "pat")
	var resp TeamListResponse
	err := c.get(srv.URL, &resp)
	if err == nil {
		t.Fatal("get() error = nil, want an error for a 404 response")
	}

	msg := err.Error()
	if !strings.Contains(msg, "GET") {
		t.Errorf("error %q does not contain the HTTP method", msg)
	}
	if !strings.Contains(msg, srv.URL) {
		t.Errorf("error %q does not contain the request URL", msg)
	}
	if !strings.Contains(msg, "404") {
		t.Errorf("error %q does not contain the status code", msg)
	}
}

func TestClientPatchJSONSkipsDecodeWhenOutIsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json-patch+json" {
			t.Errorf("Content-Type header = %q, want application/json-patch+json", got)
		}
		w.Write([]byte("not valid json, but out is nil so this must not be parsed"))
	}))
	defer srv.Close()

	c := NewClient("org", "proj", "team", "pat")
	ops := []PatchOperation{{Op: "add", Path: "/fields/System.Title", Value: "x"}}
	if err := c.patchJSON(srv.URL, "application/json-patch+json", ops, nil); err != nil {
		t.Fatalf("patchJSON() error = %v", err)
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
