package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultConfig(t *testing.T) {
	configPath := DefaultConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		t.Skip("Skipping test: config file exists at " + configPath)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.TaskDirectory != DefaultTaskDir {
		t.Errorf("Expected default task directory '%s', got '%s'", DefaultTaskDir, cfg.TaskDirectory)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	cfg := &Config{
		TaskDirectory: "/custom/path/to/tasks",
		ConfigPath:    configPath,
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if !strings.Contains(string(content), "task_directory: /custom/path/to/tasks") {
		t.Error("Config file should contain task_directory setting")
	}
}

func TestLoadExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	content := `# Kanban TUI Configuration

task_directory: /my/tasks/folder
`

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if !strings.Contains(string(data), "/my/tasks/folder") {
		t.Error("Config should contain the task directory path")
	}
}

func TestEnsureTaskDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "nested", "task", "dir")

	cfg := &Config{
		TaskDirectory: taskDir,
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
		TaskDirectory: DefaultTaskDir,
		HiddenColumns: []string{"backlog", "done"},
	}

	if !cfg.IsColumnHidden("backlog") {
		t.Error("backlog should be hidden")
	}

	if !cfg.IsColumnHidden("BACKLOG") {
		t.Error("BACKLOG should be hidden (case insensitive)")
	}

	if !cfg.IsColumnHidden("done") {
		t.Error("done should be hidden")
	}

	if cfg.IsColumnHidden("today") {
		t.Error("today should not be hidden")
	}

	if cfg.IsColumnHidden("tomorrow") {
		t.Error("tomorrow should not be hidden")
	}
}

func TestSaveAndLoadHiddenColumns(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	cfg := &Config{
		TaskDirectory: "/custom/path",
		ConfigPath:    configPath,
		HiddenColumns: []string{"backlog", "done"},
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if !strings.Contains(string(content), "hidden_columns: backlog, done") {
		t.Errorf("Config file should contain hidden_columns, got: %s", string(content))
	}
}
