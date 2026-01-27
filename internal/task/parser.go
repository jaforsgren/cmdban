package task

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	plainTagRegex = regexp.MustCompile(`@([\w][-\w]*)(?:\s|$)`)
	statusRegex   = regexp.MustCompile(`@status:([\w][-\w]*)`)
	priorityRegex = regexp.MustCompile(`@priority:(\d+)`)
	markedRegex   = regexp.MustCompile(`@marked`)
)

func ParseMarkdownFile(path string) (*Task, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	task := &Task{
		ID:       filepath.Base(path),
		FilePath: path,
		Status:   StatusBacklog,
		Tags:     []string{},
	}

	info, err := file.Stat()
	if err == nil {
		task.CreatedAt = info.ModTime()
		task.UpdatedAt = info.ModTime()
	}

	scanner := bufio.NewScanner(file)
	var lines []string
	var footerLines []string
	inFooter := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "---") && len(lines) > 0 {
			inFooter = true
			continue
		}
		if inFooter {
			footerLines = append(footerLines, line)
		} else {
			lines = append(lines, line)
		}
	}

	if len(lines) > 0 && strings.HasPrefix(lines[0], "# ") {
		task.Title = strings.TrimPrefix(lines[0], "# ")
		lines = lines[1:]
	}

	task.Description = strings.TrimSpace(strings.Join(lines, "\n"))

	footer := strings.Join(footerLines, " ")
	parseFooter(task, footer)

	return task, scanner.Err()
}

func parseFooter(task *Task, footer string) {
	if matches := statusRegex.FindStringSubmatch(footer); len(matches) > 1 {
		task.Status = Status(matches[1])
	}

	if matches := priorityRegex.FindStringSubmatch(footer); len(matches) > 1 {
		fmt.Sscanf(matches[1], "%d", &task.Priority)
	}

	task.Marked = markedRegex.MatchString(footer)

	tags := plainTagRegex.FindAllStringSubmatch(footer, -1)
	for _, match := range tags {
		tag := match[1]
		if tag != "marked" {
			task.Tags = append(task.Tags, tag)
		}
	}
}

func WriteMarkdownFile(task *Task) error {
	var sb strings.Builder

	sb.WriteString("# ")
	sb.WriteString(task.Title)
	sb.WriteString("\n\n")

	if task.Description != "" {
		sb.WriteString(task.Description)
		sb.WriteString("\n\n")
	}

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("@status:%s", task.Status))

	if task.Priority > 0 {
		sb.WriteString(fmt.Sprintf(" @priority:%d", task.Priority))
	}

	if task.Marked {
		sb.WriteString(" @marked")
	}

	for _, tag := range task.Tags {
		sb.WriteString(fmt.Sprintf(" @%s", tag))
	}
	sb.WriteString("\n")

	return os.WriteFile(task.FilePath, []byte(sb.String()), 0644)
}

func LoadTasksFromDirectory(dir string) ([]*Task, error) {
	var tasks []*Task

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return tasks, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "settings.md" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		task, err := ParseMarkdownFile(path)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func CreateNewTask(dir, title string) (*Task, error) {
	id := fmt.Sprintf("%d.md", time.Now().UnixNano())
	path := filepath.Join(dir, id)

	task := &Task{
		ID:        id,
		Title:     title,
		Status:    StatusBacklog,
		FilePath:  path,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := WriteMarkdownFile(task); err != nil {
		return nil, err
	}

	return task, nil
}

func DeleteTask(task *Task) error {
	return os.Remove(task.FilePath)
}
