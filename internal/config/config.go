package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ConfigFileName       = ".cmdban.yaml"
	LegacyConfigFileName = ".kanban-config.md"
	DefaultTaskDir       = "tasks"
)

var DefaultColumns = []string{"today", "tomorrow", "backlog", "done"}

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ConfigFileName
	}
	return filepath.Join(home, ConfigFileName)
}

func legacyConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return LegacyConfigFileName
	}
	return filepath.Join(home, LegacyConfigFileName)
}

func defaultConfig() *Config {
	columns := make([]Column, len(DefaultColumns))
	for i, name := range DefaultColumns {
		columns[i] = Column{Name: name, Hidden: false}
	}

	return &Config{
		CurrentBoard: "default",
		Boards: []Board{
			{
				Name:      "default",
				Directory: DefaultTaskDir,
				Columns:   columns,
			},
		},
		configPath: DefaultConfigPath(),
	}
}

func Load() (*Config, error) {
	configPath := DefaultConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if migrated, migrateErr := migrateFromLegacy(); migrateErr == nil && migrated != nil {
			return migrated, nil
		}

		cfg := defaultConfig()
		if saveErr := cfg.Save(); saveErr != nil {
			return nil, saveErr
		}
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	cfg := &Config{configPath: configPath}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if cfg.CurrentBoard == "" && len(cfg.Boards) > 0 {
		cfg.CurrentBoard = cfg.Boards[0].Name
	}

	return cfg, nil
}

func migrateFromLegacy() (*Config, error) {
	legacyPath := legacyConfigPath()
	file, err := os.Open(legacyPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	taskDir := DefaultTaskDir
	var hiddenColumns []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "task_directory:") {
			taskDir = strings.TrimSpace(strings.TrimPrefix(line, "task_directory:"))
		}
		if strings.HasPrefix(line, "hidden_columns:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "hidden_columns:"))
			if value != "" {
				cols := strings.Split(value, ",")
				for _, col := range cols {
					col = strings.TrimSpace(col)
					if col != "" {
						hiddenColumns = append(hiddenColumns, col)
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	columns := make([]Column, len(DefaultColumns))
	for i, name := range DefaultColumns {
		hidden := false
		for _, h := range hiddenColumns {
			if strings.EqualFold(h, name) {
				hidden = true
				break
			}
		}
		columns[i] = Column{Name: name, Hidden: hidden}
	}

	cfg := &Config{
		CurrentBoard: "default",
		Boards: []Board{
			{
				Name:      "default",
				Directory: taskDir,
				Columns:   columns,
			},
		},
		configPath: DefaultConfigPath(),
	}

	if err := cfg.Save(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(c.configPath, data, 0644)
}

func (c *Config) EnsureTaskDirectory() error {
	return os.MkdirAll(c.TaskDirectory(), 0755)
}
