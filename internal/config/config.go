package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const (
	ConfigFileName = ".kanban-config.md"
	DefaultTaskDir = "tasks"
)

type Config struct {
	TaskDirectory string
	ConfigPath    string
}

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ConfigFileName
	}
	return filepath.Join(home, ConfigFileName)
}

func Load() (*Config, error) {
	configPath := DefaultConfigPath()
	cfg := &Config{
		TaskDirectory: DefaultTaskDir,
		ConfigPath:    configPath,
	}

	file, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "task_directory:") {
			cfg.TaskDirectory = strings.TrimSpace(strings.TrimPrefix(line, "task_directory:"))
		}
	}

	return cfg, scanner.Err()
}

func (c *Config) Save() error {
	var sb strings.Builder
	sb.WriteString("# Kanban TUI Configuration\n\n")
	sb.WriteString("task_directory: ")
	sb.WriteString(c.TaskDirectory)
	sb.WriteString("\n")

	return os.WriteFile(c.ConfigPath, []byte(sb.String()), 0644)
}

func (c *Config) EnsureTaskDirectory() error {
	return os.MkdirAll(c.TaskDirectory, 0755)
}
