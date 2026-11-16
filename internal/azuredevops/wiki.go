package azuredevops

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"cmdban/internal/config"
)

const (
	// WikiRootPath is the path of a wiki's root, the parent of its top-level
	// pages.
	WikiRootPath      = "/"
	WikiPathSeparator = "/"
	wikiRecursion     = "full"
	wikiURLSegment    = "_wiki"
	wikisURLSegment   = "wikis"
	wikiPagePathKey   = "pagePath"
	headerETag        = "ETag"
	headerIfMatch     = "If-Match"
)

var (
	ErrNotWikiURL = errors.New("not a recognised ADO wiki URL")
	// ErrWikiPageChanged means the page was edited in ADO after it was opened
	// for editing, so saving would overwrite someone else's change.
	ErrWikiPageChanged = errors.New("the page changed in Azure DevOps since it was opened; reopen it and edit again")
)

// WikiLocation is where a wiki URL points: a page by ID, or the wiki root
// when PageID is 0.
type WikiLocation struct {
	Org     string
	Project string
	Wiki    string
	PageID  int
}

// ParseWikiURL reads a wiki page URL as copied from the browser:
//
//	https://dev.azure.com/{org}/{project}/_wiki/wikis/{wiki}
//	https://dev.azure.com/{org}/{project}/_wiki/wikis/{wiki}/{pageID}/{slug}
func ParseWikiURL(raw string) (WikiLocation, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return WikiLocation{}, fmt.Errorf("invalid URL: %w", err)
	}
	parts := strings.Split(strings.Trim(u.Path, WikiPathSeparator), WikiPathSeparator)
	if len(parts) < 5 || parts[2] != wikiURLSegment || parts[3] != wikisURLSegment {
		return WikiLocation{}, ErrNotWikiURL
	}
	location := WikiLocation{Org: parts[0], Project: parts[1], Wiki: parts[4]}
	if len(parts) > 5 {
		location.PageID, err = strconv.Atoi(parts[5])
		if err != nil {
			return WikiLocation{}, fmt.Errorf("%w: page ID %q is not a number", ErrNotWikiURL, parts[5])
		}
	}
	return location, nil
}

// WikiPage is a page with its sub pages; Content is only filled when a
// single page is fetched with its content.
type WikiPage struct {
	ID       int        `json:"id"`
	Path     string     `json:"path"`
	Order    int        `json:"order"`
	Content  string     `json:"content"`
	SubPages []WikiPage `json:"subPages"`
}

// Title is the page name, the last segment of its path.
func (p WikiPage) Title() string {
	return WikiPageTitle(p.Path)
}

// DescendantCount is how many pages sit beneath p at any depth.
func (p WikiPage) DescendantCount() int {
	count := len(p.SubPages)
	for _, sub := range p.SubPages {
		count += sub.DescendantCount()
	}
	return count
}

// WikiPageTitle is the last segment of a page path.
func WikiPageTitle(path string) string {
	trimmed := strings.TrimRight(path, WikiPathSeparator)
	return trimmed[strings.LastIndex(trimmed, WikiPathSeparator)+1:]
}

// sortSubPages puts sub pages in the order the wiki's navigation shows them;
// the API lists them alphabetically.
func sortSubPages(p *WikiPage) {
	slices.SortStableFunc(p.SubPages, func(a, b WikiPage) int {
		if a.Order != b.Order {
			return a.Order - b.Order
		}
		return strings.Compare(a.Path, b.Path)
	})
	for i := range p.SubPages {
		sortSubPages(&p.SubPages[i])
	}
}

// WikiPageContent is a page's markdown and the ETag an edit must send back.
type WikiPageContent struct {
	Content string
	ETag    string
}

type wikiVersion struct {
	Version string `json:"version"`
}

type wikiMetadata struct {
	Versions []wikiVersion `json:"versions"`
}

type wikiPageUpdate struct {
	Content string `json:"content"`
}

func (c *Client) wikiURL(wiki string) string {
	return fmt.Sprintf("%s/_apis/wiki/wikis/%s", c.baseURL(), url.PathEscape(wiki))
}

func (c *Client) wikiPageByPathURL(wiki, path string) string {
	return fmt.Sprintf("%s/pages?path=%s", c.wikiURL(wiki), url.QueryEscape(path))
}

// FetchWikiPageTree returns the page with every page beneath it, in
// navigation order. A pageID of 0 fetches from the wiki root.
func (c *Client) FetchWikiPageTree(wiki string, pageID int) (WikiPage, error) {
	treeURL := fmt.Sprintf("%s&recursionLevel=%s&api-version=%s", c.wikiPageByPathURL(wiki, WikiRootPath), wikiRecursion, apiVersion)
	if pageID != 0 {
		treeURL = fmt.Sprintf("%s/pages/%d?recursionLevel=%s&api-version=%s", c.wikiURL(wiki), pageID, wikiRecursion, apiVersion)
	}
	var page WikiPage
	if err := c.get(treeURL, &page); err != nil {
		return WikiPage{}, err
	}
	sortSubPages(&page)
	return page, nil
}

// FetchWikiPageContent returns a page's markdown and its current ETag.
func (c *Client) FetchWikiPageContent(wiki, path string) (WikiPageContent, error) {
	pageURL := fmt.Sprintf("%s&includeContent=true&api-version=%s", c.wikiPageByPathURL(wiki, path), apiVersion)
	var page WikiPage
	headers, err := c.send(http.MethodGet, pageURL, "", nil, nil, &page)
	if err != nil {
		return WikiPageContent{}, err
	}
	return WikiPageContent{Content: page.Content, ETag: headers.Get(headerETag)}, nil
}

// wikiVersion is the branch a code wiki publishes, which edits must name; a
// project wiki reports its own internal branch, which is accepted as well.
func (c *Client) wikiVersion(wiki string) (string, error) {
	var metadata wikiMetadata
	if err := c.get(fmt.Sprintf("%s?api-version=%s", c.wikiURL(wiki), apiVersion), &metadata); err != nil {
		return "", err
	}
	if len(metadata.Versions) == 0 {
		return "", nil
	}
	return metadata.Versions[0].Version, nil
}

// UpdateWikiPageContent replaces a page's markdown. etag is the one read
// with the content being replaced, so ADO refuses the write when the page has
// changed since.
func (c *Client) UpdateWikiPageContent(wiki, path, content, etag string) error {
	version, err := c.wikiVersion(wiki)
	if err != nil {
		return fmt.Errorf("reading wiki version: %w", err)
	}
	updateURL := c.wikiPageByPathURL(wiki, path)
	if version != "" {
		updateURL += "&versionDescriptor.version=" + url.QueryEscape(version)
	}
	updateURL += "&api-version=" + apiVersion
	_, err = c.send(http.MethodPut, updateURL, "application/json", map[string]string{headerIfMatch: etag}, wikiPageUpdate{Content: content}, nil)
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusPreconditionFailed {
		return ErrWikiPageChanged
	}
	return err
}

func newWikiClient(wiki *config.Wiki, pat string) *Client {
	return NewClient(wiki.Org, wiki.Project, "", pat)
}

// FetchWikiTree returns the configured page and everything beneath it.
func FetchWikiTree(wiki *config.Wiki, pat string) (WikiPage, error) {
	page, err := newWikiClient(wiki, pat).FetchWikiPageTree(wiki.Wiki, wiki.PageID)
	if err != nil {
		return WikiPage{}, fmt.Errorf("fetching wiki %q: %w", wiki.Name, err)
	}
	return page, nil
}

// FetchWikiPage returns the markdown of the page at path in wiki.
func FetchWikiPage(wiki *config.Wiki, pat, path string) (WikiPageContent, error) {
	content, err := newWikiClient(wiki, pat).FetchWikiPageContent(wiki.Wiki, path)
	if err != nil {
		return WikiPageContent{}, fmt.Errorf("fetching wiki page %q: %w", path, err)
	}
	return content, nil
}

// UpdateWikiPage saves edited markdown to the page at path, guarded by the
// etag read with the original content.
func UpdateWikiPage(wiki *config.Wiki, pat, path, content, etag string) error {
	if err := newWikiClient(wiki, pat).UpdateWikiPageContent(wiki.Wiki, path, content, etag); err != nil {
		return fmt.Errorf("updating wiki page %q: %w", path, err)
	}
	return nil
}

// WikiPageWebURL is the page's address in the ADO web UI.
func WikiPageWebURL(wiki *config.Wiki, path string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/%s?%s=%s", defaultHost,
		url.PathEscape(wiki.Org), url.PathEscape(wiki.Project), wikiURLSegment, wikisURLSegment, url.PathEscape(wiki.Wiki),
		wikiPagePathKey, url.QueryEscape(path))
}
