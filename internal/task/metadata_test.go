package task

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeAndParse(t *testing.T, content string) *Task {
	t.Helper()
	path := filepath.Join(t.TempDir(), "task.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	parsed, err := ParseMarkdownFile(path)
	if err != nil {
		t.Fatalf("ParseMarkdownFile failed: %v", err)
	}
	return parsed
}

func rewriteAndRead(t *testing.T, task *Task) string {
	t.Helper()
	if err := WriteMarkdownFile(task); err != nil {
		t.Fatalf("WriteMarkdownFile failed: %v", err)
	}
	content, err := os.ReadFile(task.FilePath)
	if err != nil {
		t.Fatalf("reading rewritten file: %v", err)
	}
	return string(content)
}

func TestParseLegacyFooter(t *testing.T) {
	parsed := writeAndParse(t, "# Legacy\n\nbody\n\n---\n@status:today @priority:2 @order:5 @marked @sejfa @persona\n")

	if parsed.Status != StatusToday || parsed.Priority != 2 || parsed.Order != 5 || !parsed.Marked {
		t.Errorf("unexpected schema fields: %+v", parsed)
	}
	if !slices.Equal(parsed.Tags, []string{"sejfa", "persona"}) {
		t.Errorf("expected tags [sejfa persona], got %v", parsed.Tags)
	}
	if len(parsed.ExtraMetadata) != 0 {
		t.Errorf("expected no extra metadata, got %v", parsed.ExtraMetadata)
	}
}

func TestParseLineFooter(t *testing.T) {
	parsed := writeAndParse(t, "# Lines\n\n---\n@status in-progress\n@priority 1\n@order 3\n@marked\n@tags auth, backend\n")

	if parsed.Status != "in-progress" || parsed.Priority != 1 || parsed.Order != 3 || !parsed.Marked {
		t.Errorf("unexpected schema fields: %+v", parsed)
	}
	if !slices.Equal(parsed.Tags, []string{"auth", "backend"}) {
		t.Errorf("expected tags [auth backend], got %v", parsed.Tags)
	}
}

func TestParsePreservesUnknownMetadata(t *testing.T) {
	tests := []struct {
		name     string
		footer   string
		expected []MetadataEntry
	}{
		{
			name:   "line form fields and flags",
			footer: "@status today\n@due 2026-08-21\n@depends-on token-refresh\n@archived",
			expected: []MetadataEntry{
				{Key: "due", Value: "2026-08-21"},
				{Key: "depends-on", Value: "token-refresh"},
				{Key: "archived"},
			},
		},
		{
			name:     "legacy form field",
			footer:   "@status:today @estimate:3 @backend",
			expected: []MetadataEntry{{Key: "estimate", Value: "3"}},
		},
		{
			name:     "value outside schema",
			footer:   "@status today\n@priority high",
			expected: []MetadataEntry{{Key: "priority", Value: "high"}},
		},
		{
			name:     "value with spaces and colons",
			footer:   "@status today\n@source https://example.com/a b",
			expected: []MetadataEntry{{Key: "source", Value: "https://example.com/a b"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed := writeAndParse(t, "# T\n\n---\n"+tc.footer+"\n")
			if !slices.Equal(parsed.ExtraMetadata, tc.expected) {
				t.Errorf("expected extra metadata %v, got %v", tc.expected, parsed.ExtraMetadata)
			}
		})
	}
}

func TestRewritePreservesUnknownMetadata(t *testing.T) {
	parsed := writeAndParse(t, "# T\n\n---\n@status today\n@estimate 3\n@due 2026-08-21\n@archived\n")
	parsed.Status = StatusDone

	content := rewriteAndRead(t, parsed)

	expected := "# T\n\n---\n@status done\n@estimate 3\n@due 2026-08-21\n@archived\n"
	if content != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, content)
	}
}

func TestRewriteMigratesLegacyFooter(t *testing.T) {
	parsed := writeAndParse(t, "# Legacy\n\nbody\n\n---\n@status:today @order:2 @marked @sejfa @estimate:3\n")

	content := rewriteAndRead(t, parsed)

	expected := "# Legacy\n\nbody\n\n---\n@status today\n@order 2\n@marked\n@tags sejfa\n@estimate 3\n"
	if content != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, content)
	}

	reparsed := writeAndParse(t, content)
	if reparsed.Status != parsed.Status || reparsed.Order != parsed.Order || !reparsed.Marked ||
		!slices.Equal(reparsed.Tags, parsed.Tags) || !slices.Equal(reparsed.ExtraMetadata, parsed.ExtraMetadata) {
		t.Errorf("migrated footer should round-trip: before %+v, after %+v", parsed, reparsed)
	}
}

func TestRewriteReplacesUnparseableSchemaValueOnceSet(t *testing.T) {
	parsed := writeAndParse(t, "# T\n\n---\n@status today\n@priority high\n")

	if content := rewriteAndRead(t, parsed); content != "# T\n\n---\n@status today\n@priority high\n" {
		t.Errorf("unparseable priority should survive a rewrite, got:\n%s", content)
	}

	parsed.Priority = 2
	if content := rewriteAndRead(t, parsed); content != "# T\n\n---\n@status today\n@priority 2\n" {
		t.Errorf("a real priority should replace the unparseable one, got:\n%s", content)
	}
}

func TestDescriptionHorizontalRuleIsNotFooter(t *testing.T) {
	parsed := writeAndParse(t, "# T\n\nbefore\n\n---\n\nafter the rule\n\n---\n@status today\n")

	expected := "before\n\n---\n\nafter the rule"
	if parsed.Description != expected {
		t.Errorf("expected description %q, got %q", expected, parsed.Description)
	}
	if parsed.Status != StatusToday {
		t.Errorf("expected status today, got %q", parsed.Status)
	}
}

func TestTrailingRuleWithoutMetadataStaysInDescription(t *testing.T) {
	parsed := writeAndParse(t, "# T\n\nnotes\n\n---\n\nmore notes\n")

	if parsed.Description != "notes\n\n---\n\nmore notes" {
		t.Errorf("expected rule and text to stay in the description, got %q", parsed.Description)
	}
	if parsed.Status != StatusBacklog {
		t.Errorf("expected default status backlog, got %q", parsed.Status)
	}
}

func TestParseCRLFFile(t *testing.T) {
	parsed := writeAndParse(t, "# T\r\n\r\nbody\r\n\r\n---\r\n@status today\r\n@tags a\r\n")

	if parsed.Title != "T" || parsed.Status != StatusToday || !slices.Equal(parsed.Tags, []string{"a"}) {
		t.Errorf("unexpected parse of CRLF file: %+v", parsed)
	}
}

func TestCopyTaskToDirectoryKeepsExtraMetadata(t *testing.T) {
	source := &Task{
		Title:         "Copy",
		Status:        StatusToday,
		ExtraMetadata: []MetadataEntry{{Key: "due", Value: "2026-08-21"}},
	}

	copied, err := CopyTaskToDirectory(source, t.TempDir(), StatusBacklog, "")
	if err != nil {
		t.Fatalf("CopyTaskToDirectory failed: %v", err)
	}

	reparsed, err := ParseMarkdownFile(copied.FilePath)
	if err != nil {
		t.Fatalf("ParseMarkdownFile failed: %v", err)
	}
	if !slices.Equal(reparsed.ExtraMetadata, source.ExtraMetadata) {
		t.Errorf("expected extra metadata %v, got %v", source.ExtraMetadata, reparsed.ExtraMetadata)
	}
}
