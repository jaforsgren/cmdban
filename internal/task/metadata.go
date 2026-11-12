package task

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	metadataKeyStatus   = "status"
	metadataKeyPriority = "priority"
	metadataKeyOrder    = "order"
	metadataKeyMarked   = "marked"
	metadataKeyTags     = "tags"

	metadataPrefix = "@"
	tagsSeparator  = ","

	footerRule          = "---"
	footerRuleRune      = "-"
	footerRuleMinLength = len(footerRule)
)

var (
	// lineFieldRegex matches the one-field-per-line form, "@key value". The
	// value may not start with "@", so a legacy line like "@marked @backend"
	// is read as two tokens instead of key "marked" with value "@backend".
	lineFieldRegex = regexp.MustCompile(`^@([\w][-\w]*)\s+([^@\s].*)$`)
	// legacyFieldRegex matches the single-line form's "@key:value" token.
	legacyFieldRegex = regexp.MustCompile(`^@([\w][-\w]*):(\S*)$`)
	flagRegex        = regexp.MustCompile(`^@([\w][-\w]*)$`)
)

var knownFlags = []string{metadataKeyMarked}

// splitFooter separates a task file's content lines from its metadata
// footer: the lines after the last "---" rule, provided every non-blank one
// is metadata. Using the last rule, and only when metadata follows it, keeps
// horizontal rules inside the description from swallowing the rest of it.
func splitFooter(lines []string) (content []string, footer []string) {
	for i := len(lines) - 1; i > 0; i-- {
		if !isFooterRule(lines[i]) {
			continue
		}
		if isMetadataBlock(lines[i+1:]) {
			return lines[:i], lines[i+1:]
		}
		break
	}
	return lines, nil
}

func isFooterRule(line string) bool {
	trimmed := strings.TrimSpace(line)
	return len(trimmed) >= footerRuleMinLength && strings.Trim(trimmed, footerRuleRune) == ""
}

func isMetadataBlock(lines []string) bool {
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, metadataPrefix) {
			return false
		}
		found = true
	}
	return found
}

// parseFooter reads both footer forms into task:
//
//	@status:today @order:2 @marked @backend   (legacy, single line)
//	@status today                              (one field per line)
//	@tags backend,api
//
// The forms differ in what a bare "@word" means: in the legacy form it is a
// tag, in the line form tags live under @tags and a bare word is a flag. A
// footer counts as legacy when any token uses "@key:value".
func parseFooter(task *Task, footerLines []string) {
	legacy := isLegacyFooter(footerLines)
	seen := map[string]bool{}

	for _, line := range footerLines {
		for _, entry := range parseFooterLine(strings.TrimSpace(line), legacy) {
			applyMetadataEntry(task, entry, seen)
		}
	}
}

func isLegacyFooter(footerLines []string) bool {
	for _, line := range footerLines {
		trimmed := strings.TrimSpace(line)
		if lineFieldRegex.MatchString(trimmed) {
			continue
		}
		for _, token := range strings.Fields(trimmed) {
			if legacyFieldRegex.MatchString(token) {
				return true
			}
		}
	}
	return false
}

// parseFooterLine turns one footer line into entries. Stray words that
// aren't "@" tokens are ignored, matching how earlier versions read footers.
func parseFooterLine(line string, legacy bool) []MetadataEntry {
	if m := lineFieldRegex.FindStringSubmatch(line); m != nil {
		return []MetadataEntry{{Key: m[1], Value: strings.TrimSpace(m[2])}}
	}

	var entries []MetadataEntry
	for _, token := range strings.Fields(line) {
		if m := legacyFieldRegex.FindStringSubmatch(token); m != nil {
			entries = append(entries, MetadataEntry{Key: m[1], Value: m[2]})
			continue
		}
		m := flagRegex.FindStringSubmatch(token)
		if m == nil {
			continue
		}
		if legacy && !slices.Contains(knownFlags, m[1]) {
			entries = append(entries, MetadataEntry{Key: metadataKeyTags, Value: m[1]})
			continue
		}
		entries = append(entries, MetadataEntry{Key: m[1]})
	}
	return entries
}

// applyMetadataEntry maps an entry onto its schema field. The first value
// for a single-valued key wins, as in earlier versions; repeats and values
// that don't fit the schema (e.g. "@priority high") are kept verbatim in
// ExtraMetadata rather than dropped.
func applyMetadataEntry(task *Task, entry MetadataEntry, seen map[string]bool) {
	switch entry.Key {
	case metadataKeyTags:
		task.Tags = append(task.Tags, splitTags(entry.Value)...)
		return
	case metadataKeyMarked:
		task.Marked = true
		return
	}

	if seen[entry.Key] || !applySingleValuedEntry(task, entry) {
		task.ExtraMetadata = append(task.ExtraMetadata, entry)
		return
	}
	seen[entry.Key] = true
}

func applySingleValuedEntry(task *Task, entry MetadataEntry) bool {
	switch entry.Key {
	case metadataKeyStatus:
		if entry.Value == "" {
			return false
		}
		task.Status = Status(entry.Value)
		return true
	case metadataKeyPriority:
		return parseNonNegativeInt(entry.Value, &task.Priority)
	case metadataKeyOrder:
		return parseNonNegativeInt(entry.Value, &task.Order)
	}
	return false
}

func parseNonNegativeInt(value string, target *int) bool {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return false
	}
	*target = n
	return true
}

func splitTags(value string) []string {
	var tags []string
	for _, tag := range strings.Split(value, tagsSeparator) {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// formatFooter renders task's metadata one field per line: schema fields
// first, then ExtraMetadata in its original order. An extra entry for a
// schema key is only written when the schema field itself was not, so an
// unparseable "@priority high" survives until a real priority replaces it.
func formatFooter(task *Task) []string {
	lines := []string{formatMetadataEntry(MetadataEntry{Key: metadataKeyStatus, Value: string(task.Status)})}
	written := map[string]bool{metadataKeyStatus: true}

	if task.Priority > 0 {
		lines = append(lines, formatMetadataEntry(MetadataEntry{Key: metadataKeyPriority, Value: strconv.Itoa(task.Priority)}))
		written[metadataKeyPriority] = true
	}
	if task.Order > 0 {
		lines = append(lines, formatMetadataEntry(MetadataEntry{Key: metadataKeyOrder, Value: strconv.Itoa(task.Order)}))
		written[metadataKeyOrder] = true
	}
	if task.Marked {
		lines = append(lines, formatMetadataEntry(MetadataEntry{Key: metadataKeyMarked}))
	}
	if len(task.Tags) > 0 {
		lines = append(lines, formatMetadataEntry(MetadataEntry{Key: metadataKeyTags, Value: strings.Join(task.Tags, tagsSeparator)}))
	}

	for _, entry := range task.ExtraMetadata {
		if written[entry.Key] {
			continue
		}
		lines = append(lines, formatMetadataEntry(entry))
	}
	return lines
}

func formatMetadataEntry(entry MetadataEntry) string {
	if entry.Value == "" {
		return metadataPrefix + entry.Key
	}
	return metadataPrefix + entry.Key + " " + entry.Value
}
