package task

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"cmdban/internal/fsutil"
)

var (
	checkedBoxRegex   = regexp.MustCompile(`(?m)^- \[x\]`)
	uncheckedBoxRegex = regexp.MustCompile(`(?m)^- \[ \]`)
)

const (
	titlePrefix  = "# "
	taskFileMode = 0644
)

func ParseMarkdownFile(path string) (*Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	task := &Task{
		ID:       filepath.Base(path),
		FilePath: path,
		Status:   StatusBacklog,
		Tags:     []string{},
	}

	if info, err := os.Stat(path); err == nil {
		task.CreatedAt = info.ModTime()
		task.UpdatedAt = info.ModTime()
	}

	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines, footerLines := splitFooter(strings.Split(normalized, "\n"))

	if len(lines) > 0 && strings.HasPrefix(lines[0], titlePrefix) {
		task.Title = strings.TrimPrefix(lines[0], titlePrefix)
		lines = lines[1:]
	}

	task.Description = strings.TrimSpace(strings.Join(lines, "\n"))
	task.CheckboxDone = len(checkedBoxRegex.FindAllString(task.Description, -1))
	task.CheckboxTotal = task.CheckboxDone + len(uncheckedBoxRegex.FindAllString(task.Description, -1))

	parseFooter(task, footerLines)

	return task, nil
}

func WriteMarkdownFile(task *Task) error {
	if task.FilePath == "" {
		return nil
	}
	var sb strings.Builder

	sb.WriteString(titlePrefix)
	sb.WriteString(task.Title)
	sb.WriteString("\n\n")

	if task.Description != "" {
		sb.WriteString(task.Description)
		sb.WriteString("\n\n")
	}

	sb.WriteString(footerRule + "\n")
	for _, line := range formatFooter(task) {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return fsutil.WriteFileAtomic(task.FilePath, []byte(sb.String()), taskFileMode)
}

// LoadTasksFromDirectory reads every markdown file in dir as a task, except
// dotfiles, settings.md, and any name matching ignoreFiles (compared
// case-insensitively) — for files that live alongside tasks but aren't
// tasks themselves, such as a CLAUDE.md.
func LoadTasksFromDirectory(dir string, ignoreFiles []string) ([]*Task, error) {
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
		if isFileIgnored(entry.Name(), ignoreFiles) {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		task, err := ParseMarkdownFile(path)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		oi, oj := tasks[i].Order, tasks[j].Order
		if oi == 0 && oj == 0 {
			return tasks[i].ID < tasks[j].ID
		}
		if oi == 0 {
			return false
		}
		if oj == 0 {
			return true
		}
		return oi < oj
	})

	return tasks, nil
}

func isFileIgnored(filename string, ignoreFiles []string) bool {
	for _, ignored := range ignoreFiles {
		if strings.EqualFold(ignored, filename) {
			return true
		}
	}
	return false
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

// CopyTaskToDirectory writes t's title, description, and metadata into a new
// markdown file under dir, using a freshly generated ID and status. ADO
// linkage fields are intentionally not copied, since moving a task to a
// plain markdown board detaches it from Azure DevOps. When sourceURL is
// non-empty, it is appended to the description as a link back to where the
// task came from, since the copy otherwise carries no trace of its origin.
// A task carrying an ADO work item ID has that ID prepended to its title,
// so it stays identifiable once detached from Azure DevOps.
func CopyTaskToDirectory(t *Task, dir string, status Status, sourceURL string) (*Task, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	title := t.Title
	if t.ADOItemID != 0 {
		title = fmt.Sprintf("#%d %s", t.ADOItemID, t.Title)
	}

	slug := sanitizeTitle(title)
	var filename string
	if slug == "" {
		filename = generateShortID() + ".md"
	} else {
		filename = uniqueFilename(dir, slug)
	}
	path := filepath.Join(dir, filename)

	description := t.Description
	if t.ReproSteps != "" {
		description = strings.TrimSpace(description + "\n\n## Repro Steps\n\n" + t.ReproSteps)
	}
	if sourceURL != "" {
		description = strings.TrimSpace(description + "\n\nSource: " + sourceURL)
	}

	now := time.Now()
	newTask := &Task{
		ID:            filename,
		Title:         title,
		Description:   description,
		Status:        status,
		Tags:          slices.Clone(t.Tags),
		Priority:      t.Priority,
		Marked:        t.Marked,
		CheckboxDone:  t.CheckboxDone,
		CheckboxTotal: t.CheckboxTotal,
		ExtraMetadata: slices.Clone(t.ExtraMetadata),
		CreatedAt:     now,
		UpdatedAt:     now,
		FilePath:      path,
	}

	if err := WriteMarkdownFile(newTask); err != nil {
		return nil, err
	}

	return newTask, nil
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
