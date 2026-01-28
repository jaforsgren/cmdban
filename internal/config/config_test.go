package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.TaskDirectory() != DefaultTaskDir {
		t.Errorf("Expected default task directory '%s', got '%s'", DefaultTaskDir, cfg.TaskDirectory())
	}

	if cfg.CurrentBoard != "default" {
		t.Errorf("Expected current board 'default', got '%s'", cfg.CurrentBoard)
	}

	if len(cfg.Boards) != 1 {
		t.Errorf("Expected 1 board, got %d", len(cfg.Boards))
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	cfg := &Config{
		CurrentBoard: "test",
		Boards: []Board{
			{
				Name:      "test",
				Directory: "/custom/path/to/tasks",
				Columns: []Column{
					{Name: "today", Hidden: false},
					{Name: "done", Hidden: false},
				},
			},
		},
		configPath: configPath,
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if !strings.Contains(string(content), "directory: /custom/path/to/tasks") {
		t.Error("Config file should contain directory setting")
	}
}

func TestLoadExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	content := `current_board: myboard
boards:
  - name: myboard
    directory: /my/tasks/folder
    columns:
      - name: todo
        hidden: false
      - name: done
        hidden: false
`

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.TaskDirectory() != "/my/tasks/folder" {
		t.Errorf("Expected task directory '/my/tasks/folder', got '%s'", cfg.TaskDirectory())
	}
}

func TestEnsureTaskDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "nested", "task", "dir")

	cfg := &Config{
		CurrentBoard: "default",
		Boards: []Board{
			{
				Name:      "default",
				Directory: taskDir,
				Columns:   []Column{{Name: "backlog", Hidden: false}},
			},
		},
	}

	if err := cfg.EnsureTaskDirectory(); err != nil {
		t.Fatalf("EnsureTaskDirectory failed: %v", err)
	}

	info, err := os.Stat(taskDir)
	if err != nil {
		t.Fatalf("Task directory should exist: %v", err)
	}

	if !info.IsDir() {
		t.Error("Task directory should be a directory")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()

	if path == "" {
		t.Error("DefaultConfigPath should not be empty")
	}

	if !strings.HasSuffix(path, ConfigFileName) {
		t.Errorf("DefaultConfigPath should end with %s", ConfigFileName)
	}
}

func TestHiddenColumns(t *testing.T) {
	cfg := &Config{
		CurrentBoard: "default",
		Boards: []Board{
			{
				Name:      "default",
				Directory: DefaultTaskDir,
				Columns: []Column{
					{Name: "today", Hidden: false},
					{Name: "backlog", Hidden: true},
					{Name: "done", Hidden: true},
				},
			},
		},
	}

	if !cfg.IsColumnHidden("backlog") {
		t.Error("backlog should be hidden")
	}

	if !cfg.IsColumnHidden("done") {
		t.Error("done should be hidden")
	}

	if cfg.IsColumnHidden("today") {
		t.Error("today should not be hidden")
	}

	if cfg.IsColumnHidden("tomorrow") {
		t.Error("tomorrow should not be hidden (not in config)")
	}

	hidden := cfg.HiddenColumns()
	if len(hidden) != 2 {
		t.Errorf("Expected 2 hidden columns, got %d", len(hidden))
	}
}

func TestSaveAndLoadHiddenColumns(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	cfg := &Config{
		CurrentBoard: "default",
		Boards: []Board{
			{
				Name:      "default",
				Directory: "/custom/path",
				Columns: []Column{
					{Name: "today", Hidden: false},
					{Name: "backlog", Hidden: true},
					{Name: "done", Hidden: true},
				},
			},
		},
		configPath: configPath,
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "hidden: true") {
		t.Errorf("Config file should contain hidden columns, got: %s", contentStr)
	}
}

func TestVisibleColumns(t *testing.T) {
	cfg := &Config{
		CurrentBoard: "default",
		Boards: []Board{
			{
				Name:      "default",
				Directory: DefaultTaskDir,
				Columns: []Column{
					{Name: "today", Hidden: false},
					{Name: "tomorrow", Hidden: false},
					{Name: "backlog", Hidden: true},
					{Name: "done", Hidden: false},
				},
			},
		},
	}

	visible := cfg.VisibleColumns()
	if len(visible) != 3 {
		t.Errorf("Expected 3 visible columns, got %d", len(visible))
	}

	expected := []string{"today", "tomorrow", "done"}
	for i, col := range visible {
		if col != expected[i] {
			t.Errorf("Expected column %d to be '%s', got '%s'", i, expected[i], col)
		}
	}
}

func TestAllColumns(t *testing.T) {
	cfg := &Config{
		CurrentBoard: "default",
		Boards: []Board{
			{
				Name:      "default",
				Directory: DefaultTaskDir,
				Columns: []Column{
					{Name: "custom1", Hidden: false},
					{Name: "custom2", Hidden: true},
					{Name: "custom3", Hidden: false},
				},
			},
		},
	}

	all := cfg.AllColumns()
	if len(all) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(all))
	}

	expected := []string{"custom1", "custom2", "custom3"}
	for i, col := range all {
		if col != expected[i] {
			t.Errorf("Expected column %d to be '%s', got '%s'", i, expected[i], col)
		}
	}
}

func TestMigrateFromLegacy(t *testing.T) {
	tmpDir := t.TempDir()

	legacyContent := `# Kanban TUI Configuration

task_directory: /legacy/tasks
hidden_columns: backlog, done
`

	legacyPath := filepath.Join(tmpDir, LegacyConfigFileName)
	if err := os.WriteFile(legacyPath, []byte(legacyContent), 0644); err != nil {
		t.Fatalf("Failed to create legacy config: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.TaskDirectory() != "/legacy/tasks" {
		t.Errorf("Expected migrated task directory '/legacy/tasks', got '%s'", cfg.TaskDirectory())
	}

	if !cfg.IsColumnHidden("backlog") {
		t.Error("backlog should be hidden after migration")
	}

	if !cfg.IsColumnHidden("done") {
		t.Error("done should be hidden after migration")
	}

	if cfg.IsColumnHidden("today") {
		t.Error("today should not be hidden after migration")
	}

	newConfigPath := filepath.Join(tmpDir, ConfigFileName)
	if _, err := os.Stat(newConfigPath); os.IsNotExist(err) {
		t.Error("New YAML config should have been created")
	}
}

func TestActiveBoard(t *testing.T) {
	cfg := &Config{
		CurrentBoard: "second",
		Boards: []Board{
			{Name: "first", Directory: "/first"},
			{Name: "second", Directory: "/second"},
		},
	}

	board := cfg.ActiveBoard()
	if board == nil {
		t.Fatal("ActiveBoard should not be nil")
	}

	if board.Name != "second" {
		t.Errorf("Expected active board 'second', got '%s'", board.Name)
	}

	if board.Directory != "/second" {
		t.Errorf("Expected directory '/second', got '%s'", board.Directory)
	}
}

func TestMultipleBoards(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	cfg := &Config{
		CurrentBoard: "work",
		Boards: []Board{
			{
				Name:      "personal",
				Directory: "/personal/tasks",
				Columns: []Column{
					{Name: "today", Hidden: false},
					{Name: "done", Hidden: false},
				},
			},
			{
				Name:      "work",
				Directory: "/work/tasks",
				Columns: []Column{
					{Name: "urgent", Hidden: false},
					{Name: "normal", Hidden: false},
					{Name: "done", Hidden: true},
				},
			},
		},
		configPath: configPath,
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if cfg.TaskDirectory() != "/work/tasks" {
		t.Errorf("Expected task directory '/work/tasks', got '%s'", cfg.TaskDirectory())
	}

	cfg.CurrentBoard = "personal"
	if cfg.TaskDirectory() != "/personal/tasks" {
		t.Errorf("Expected task directory '/personal/tasks', got '%s'", cfg.TaskDirectory())
	}
}
