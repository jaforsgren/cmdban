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
	plainTagRegex      = regexp.MustCompile(`@([\w][-\w]*)(?:\s|$)`)
	statusRegex        = regexp.MustCompile(`@status:([\w][-\w]*)`)
	priorityRegex      = regexp.MustCompile(`@priority:(\d+)`)
	markedRegex        = regexp.MustCompile(`@marked`)
	checkedBoxRegex    = regexp.MustCompile(`(?m)^- \[x\]`)
	uncheckedBoxRegex  = regexp.MustCompile(`(?m)^- \[ \]`)
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
	task.CheckboxDone = len(checkedBoxRegex.FindAllString(task.Description, -1))
	task.CheckboxTotal = task.CheckboxDone + len(uncheckedBoxRegex.FindAllString(task.Description, -1))

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

func sanitizeTitle(title string) string {
	result := strings.ToLower(title)
	result = strings.ReplaceAll(result, " ", "_")

	sanitizeRegex := regexp.MustCompile(`[^a-z0-9_]`)
	result = sanitizeRegex.ReplaceAllString(result, "")

	multiUnderscoreRegex := regexp.MustCompile(`_+`)
	result = multiUnderscoreRegex.ReplaceAllString(result, "_")

	result = strings.Trim(result, "_")

	return result
}

func generateShortID() string {
	return fmt.Sprintf("%06x", time.Now().UnixNano()&0xFFFFFF)
}

func uniqueFilename(dir, slug string) string {
	candidate := filepath.Join(dir, slug+".md")
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return slug + ".md"
	}
	for i := 2; ; i++ {
		name := fmt.Sprintf("%s_%d.md", slug, i)
		candidate = filepath.Join(dir, name)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return name
		}
	}
}

func CreateNewTask(dir, title string) (*Task, error) {
	slug := sanitizeTitle(title)

	var filename string
	if slug == "" {
		filename = generateShortID() + ".md"
	} else {
		filename = uniqueFilename(dir, slug)
	}
	path := filepath.Join(dir, filename)

	t := &Task{
		ID:        filename,
		Title:     title,
		Status:    StatusBacklog,
		FilePath:  path,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := WriteMarkdownFile(t); err != nil {
		return nil, err
	}

	return t, nil
}

const archiveDirName = "_archive"

func ArchiveTask(t *Task) error {
	archiveDir := filepath.Join(filepath.Dir(t.FilePath), archiveDirName)
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return err
	}
	dest := filepath.Join(archiveDir, filepath.Base(t.FilePath))
	return os.Rename(t.FilePath, dest)
}
