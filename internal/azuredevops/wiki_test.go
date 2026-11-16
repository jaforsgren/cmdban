package azuredevops

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cmdban/internal/config"
)

func TestParseWikiURLReadsPageIDAndSlug(t *testing.T) {
	got, err := ParseWikiURL("https://dev.azure.com/lfantdevelophub/common/_wiki/wikis/ProjectManagement/93/Kravdokumentation")
	if err != nil {
		t.Fatalf("ParseWikiURL() error = %v", err)
	}
	want := WikiLocation{Org: "lfantdevelophub", Project: "common", Wiki: "ProjectManagement", PageID: 93}
	if got != want {
		t.Fatalf("ParseWikiURL() = %+v, want %+v", got, want)
	}
}

func TestParseWikiURLWithoutPageIsTheWikiRoot(t *testing.T) {
	got, err := ParseWikiURL("https://dev.azure.com/org/proj/_wiki/wikis/Docs")
	if err != nil {
		t.Fatalf("ParseWikiURL() error = %v", err)
	}
	if got.PageID != 0 || got.Wiki != "Docs" {
		t.Fatalf("ParseWikiURL() = %+v, want the Docs wiki root", got)
	}
}

func TestParseWikiURLRejectsOtherURLs(t *testing.T) {
	for _, raw := range []string{
		"https://dev.azure.com/org/proj/_boards/board/t/team/Stories",
		"https://dev.azure.com/org/proj/_wiki/wikis/Docs/notanumber/Page",
	} {
		if _, err := ParseWikiURL(raw); !errors.Is(err, ErrNotWikiURL) {
			t.Errorf("ParseWikiURL(%q) error = %v, want ErrNotWikiURL", raw, err)
		}
	}
}

func TestWikiPageTitleIsTheLastPathSegment(t *testing.T) {
	if got := WikiPageTitle("/Krav/Insurify/SIE Filer"); got != "SIE Filer" {
		t.Fatalf("WikiPageTitle() = %q, want %q", got, "SIE Filer")
	}
}

func TestDescendantCountCountsEveryLevel(t *testing.T) {
	page := WikiPage{SubPages: []WikiPage{{SubPages: []WikiPage{{}, {}}}, {}}}
	if got := page.DescendantCount(); got != 4 {
		t.Fatalf("DescendantCount() = %d, want 4", got)
	}
}

func testClient(serverURL string) *Client {
	c := NewClient("org", "proj", "", "pat")
	c.host = serverURL
	return c
}

func TestFetchWikiPageTreeByIDSortsSubPagesInNavigationOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/org/proj/_apis/wiki/wikis/Docs/pages/93" {
			t.Errorf("path = %q, want the page-by-ID endpoint", r.URL.Path)
		}
		if got := r.URL.Query().Get("recursionLevel"); got != wikiRecursion {
			t.Errorf("recursionLevel = %q, want %q", got, wikiRecursion)
		}
		json.NewEncoder(w).Encode(WikiPage{ID: 93, Path: "/Krav", SubPages: []WikiPage{
			{Path: "/Krav/B", Order: 1},
			{Path: "/Krav/A", Order: 2, SubPages: []WikiPage{{Path: "/Krav/A/y", Order: 1}, {Path: "/Krav/A/x", Order: 0}}},
		}})
	}))
	defer srv.Close()

	page, err := testClient(srv.URL).FetchWikiPageTree("Docs", 93)
	if err != nil {
		t.Fatalf("FetchWikiPageTree() error = %v", err)
	}
	if page.SubPages[0].Path != "/Krav/B" || page.SubPages[1].SubPages[0].Path != "/Krav/A/x" {
		t.Fatalf("sub pages = %+v, want them sorted by order at every level", page.SubPages)
	}
}

func TestFetchWikiPageTreeWithoutIDStartsAtTheRoot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("path"); got != WikiRootPath {
			t.Errorf("path query = %q, want the wiki root", got)
		}
		w.Write([]byte(`{"path":"/"}`))
	}))
	defer srv.Close()

	if _, err := testClient(srv.URL).FetchWikiPageTree("Docs", 0); err != nil {
		t.Fatalf("FetchWikiPageTree() error = %v", err)
	}
}

func TestFetchWikiPageContentReturnsTheETag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("path"); got != "/Krav/Pay later" {
			t.Errorf("path query = %q, want the page path unescaped", got)
		}
		w.Header().Set(headerETag, `"abc"`)
		w.Write([]byte(`{"path":"/Krav/Pay later","content":"# Pay later"}`))
	}))
	defer srv.Close()

	got, err := testClient(srv.URL).FetchWikiPageContent("Docs", "/Krav/Pay later")
	if err != nil {
		t.Fatalf("FetchWikiPageContent() error = %v", err)
	}
	if got.Content != "# Pay later" || got.ETag != `"abc"` {
		t.Fatalf("FetchWikiPageContent() = %+v, want the content and ETag", got)
	}
}

// wikiUpdateServer answers the wiki metadata read and records the page PUT.
func wikiUpdateServer(t *testing.T, putStatus int, put func(r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"versions":[{"version":"wiki-main"}]}`))
			return
		}
		put(r)
		w.WriteHeader(putStatus)
		w.Write([]byte(`{}`))
	}))
}

func TestUpdateWikiPageContentSendsETagAndPublishedBranch(t *testing.T) {
	var got *http.Request
	var body wikiPageUpdate
	srv := wikiUpdateServer(t, http.StatusOK, func(r *http.Request) {
		got = r
		json.NewDecoder(r.Body).Decode(&body)
	})
	defer srv.Close()

	if err := testClient(srv.URL).UpdateWikiPageContent("Docs", "/Krav", "new text", `"abc"`); err != nil {
		t.Fatalf("UpdateWikiPageContent() error = %v", err)
	}
	if got == nil || got.Method != http.MethodPut {
		t.Fatalf("request = %v, want a PUT", got)
	}
	if ifMatch := got.Header.Get(headerIfMatch); ifMatch != `"abc"` {
		t.Errorf("If-Match = %q, want the ETag", ifMatch)
	}
	if version := got.URL.Query().Get("versionDescriptor.version"); version != "wiki-main" {
		t.Errorf("versionDescriptor.version = %q, want the published branch", version)
	}
	if body.Content != "new text" {
		t.Errorf("content = %q, want the edited text", body.Content)
	}
}

func TestUpdateWikiPageContentReportsAConcurrentEdit(t *testing.T) {
	srv := wikiUpdateServer(t, http.StatusPreconditionFailed, func(*http.Request) {})
	defer srv.Close()

	err := testClient(srv.URL).UpdateWikiPageContent("Docs", "/Krav", "new text", `"stale"`)
	if !errors.Is(err, ErrWikiPageChanged) {
		t.Fatalf("UpdateWikiPageContent() error = %v, want ErrWikiPageChanged", err)
	}
}

func TestWikiPageWebURLLinksToThePagePath(t *testing.T) {
	w := &config.Wiki{Org: "org", Project: "my proj", Wiki: "Docs"}
	got := WikiPageWebURL(w, "/Krav/Pay later")
	if !strings.HasPrefix(got, "https://dev.azure.com/org/my%20proj/_wiki/wikis/Docs?pagePath=") ||
		!strings.HasSuffix(got, "%2FKrav%2FPay+later") {
		t.Fatalf("WikiPageWebURL() = %q", got)
	}
}
