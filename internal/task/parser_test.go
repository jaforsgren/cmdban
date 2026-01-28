package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMarkdownFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-task.md")

	content := `# Test Task Title

This is the description of the task.
It can span multiple lines.

---
@status:today @priority:2 @bug @urgent
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	task, err := ParseMarkdownFile(testFile)
	if err != nil {
		t.Fatalf("ParseMarkdownFile failed: %v", err)
	}

	if task.Title != "Test Task Title" {
		t.Errorf("Expected title 'Test Task Title', got '%s'", task.Title)
	}

	if task.Status != StatusToday {
		t.Errorf("Expected status 'today', got '%s'", task.Status)
	}

	if task.Priority != 2 {
		t.Errorf("Expected priority 2, got %d", task.Priority)
	}

	expectedTags := []string{"bug", "urgent"}
	if len(task.Tags) != len(expectedTags) {
		t.Errorf("Expected %d tags, got %d", len(expectedTags), len(task.Tags))
	}

	for i, tag := range expectedTags {
		if i < len(task.Tags) && task.Tags[i] != tag {
			t.Errorf("Expected tag '%s', got '%s'", tag, task.Tags[i])
		}
	}

	if !strings.Contains(task.Description, "description of the task") {
		t.Errorf("Description should contain task description text")
	}
}

func TestWriteMarkdownFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "write-test.md")

	task := &Task{
		ID:          "write-test.md",
		Title:       "Write Test Task",
		Description: "Testing write functionality",
		Status:      StatusTomorrow,
		Priority:    3,
		Tags:        []string{"test", "write"},
		FilePath:    testFile,
	}

	if err := WriteMarkdownFile(task); err != nil {
		t.Fatalf("WriteMarkdownFile failed: %v", err)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, "# Write Test Task") {
		t.Error("Written file should contain title")
	}

	if !strings.Contains(contentStr, "@status:tomorrow") {
		t.Error("Written file should contain status")
	}

	if !strings.Contains(contentStr, "@priority:3") {
		t.Error("Written file should contain priority")
	}

	if !strings.Contains(contentStr, "@test") {
		t.Error("Written file should contain tags")
	}
}

func TestRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "roundtrip.md")

	original := &Task{
		ID:          "roundtrip.md",
		Title:       "Roundtrip Test",
		Description: "Testing parse/write roundtrip",
		Status:      StatusBacklog,
		Priority:    1,
		Tags:        []string{"roundtrip", "test"},
		FilePath:    testFile,
	}

	if err := WriteMarkdownFile(original); err != nil {
		t.Fatalf("WriteMarkdownFile failed: %v", err)
	}

	parsed, err := ParseMarkdownFile(testFile)
	if err != nil {
		t.Fatalf("ParseMarkdownFile failed: %v", err)
	}

	if parsed.Title != original.Title {
		t.Errorf("Title mismatch: expected '%s', got '%s'", original.Title, parsed.Title)
	}

	if parsed.Status != original.Status {
		t.Errorf("Status mismatch: expected '%s', got '%s'", original.Status, parsed.Status)
	}

	if parsed.Priority != original.Priority {
		t.Errorf("Priority mismatch: expected %d, got %d", original.Priority, parsed.Priority)
	}
}

func TestLoadTasksFromDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	tasks := []struct {
		name    string
		content string
	}{
		{"task1.md", "# Task 1\n\n---\n@status:backlog"},
		{"task2.md", "# Task 2\n\n---\n@status:tomorrow"},
		{"task3.md", "# Task 3\n\n---\n@status:today"},
		{".hidden.md", "# Hidden\n\n---\n@status:tomorrow"},
		{"settings.md", "# Settings\n\ntask_directory: ./tasks"},
	}

	for _, tc := range tasks {
		path := filepath.Join(tmpDir, tc.name)
		if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	loaded, err := LoadTasksFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadTasksFromDirectory failed: %v", err)
	}

	if len(loaded) != 3 {
		t.Errorf("Expected 3 tasks (excluding hidden and settings), got %d", len(loaded))
	}
}

func TestCreateNewTask(t *testing.T) {
	tmpDir := t.TempDir()

	task, err := CreateNewTask(tmpDir, "New Task")
	if err != nil {
		t.Fatalf("CreateNewTask failed: %v", err)
	}

	if task.Title != "New Task" {
		t.Errorf("Expected title 'New Task', got '%s'", task.Title)
	}

	if task.Status != StatusBacklog {
		t.Errorf("New tasks should have backlog status, got '%s'", task.Status)
	}

	if _, err := os.Stat(task.FilePath); os.IsNotExist(err) {
		t.Error("Task file should exist on disk")
	}
}

func TestDeleteTask(t *testing.T) {
	tmpDir := t.TempDir()

	task, err := CreateNewTask(tmpDir, "Task to Delete")
	if err != nil {
		t.Fatalf("CreateNewTask failed: %v", err)
	}

	if _, err := os.Stat(task.FilePath); os.IsNotExist(err) {
		t.Fatal("Task file should exist before deletion")
	}

	if err := DeleteTask(task); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	if _, err := os.Stat(task.FilePath); !os.IsNotExist(err) {
		t.Error("Task file should not exist after deletion")
	}
}

func TestSanitizeTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Simple Title", "simple-title"},
		{"Fix API: Rate Limiting", "fix-api-rate-limiting"},
		{"Update the login flow!", "update-the-login-flow"},
		{"Add feature #123", "add-feature-123"},
		{"  spaces   everywhere  ", "spaces-everywhere"},
		{"UPPERCASE TITLE", "uppercase-title"},
		{"special@chars#here$now", "specialcharsherenow"},
		{"multiple---hyphens", "multiple-hyphens"},
		{"", ""},
		{"!@#$%", ""},
		{"123-numbers-456", "123-numbers-456"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitizeTitle(tc.input)
			if result != tc.expected {
				t.Errorf("sanitizeTitle(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestCreateNewTaskFilename(t *testing.T) {
	tmpDir := t.TempDir()

	task, err := CreateNewTask(tmpDir, "Fix the Bug")
	if err != nil {
		t.Fatalf("CreateNewTask failed: %v", err)
	}

	if !strings.HasPrefix(task.ID, "fix-the-bug-") {
		t.Errorf("Expected ID to start with 'fix-the-bug-', got '%s'", task.ID)
	}

	if !strings.HasSuffix(task.ID, ".md") {
		t.Errorf("Expected ID to end with '.md', got '%s'", task.ID)
	}
}

func TestCreateNewTaskEmptySlug(t *testing.T) {
	tmpDir := t.TempDir()

	task, err := CreateNewTask(tmpDir, "!@#$%")
	if err != nil {
		t.Fatalf("CreateNewTask failed: %v", err)
	}

	if strings.HasPrefix(task.ID, "-") {
		t.Errorf("ID should not start with hyphen when slug is empty, got '%s'", task.ID)
	}

	if !strings.HasSuffix(task.ID, ".md") {
		t.Errorf("Expected ID to end with '.md', got '%s'", task.ID)
	}
}

func TestParseFooterVariations(t *testing.T) {
	tests := []struct {
		name           string
		footer         string
		expectedStatus Status
		expectedTags   []string
	}{
		{
			name:           "status only",
			footer:         "@status:tomorrow",
			expectedStatus: StatusTomorrow,
			expectedTags:   nil,
		},
		{
			name:           "multiple tags",
			footer:         "@status:backlog @feature @backend @api",
			expectedStatus: StatusBacklog,
			expectedTags:   []string{"feature", "backend", "api"},
		},
		{
			name:           "tags with dashes",
			footer:         "@status:today @front-end @high-priority",
			expectedStatus: StatusToday,
			expectedTags:   []string{"front-end", "high-priority"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &Task{Tags: []string{}}
			parseFooter(task, tc.footer)

			if task.Status != tc.expectedStatus {
				t.Errorf("Expected status '%s', got '%s'", tc.expectedStatus, task.Status)
			}

			if len(tc.expectedTags) != len(task.Tags) {
				t.Errorf("Expected %d tags, got %d", len(tc.expectedTags), len(task.Tags))
			}
		})
	}
}
