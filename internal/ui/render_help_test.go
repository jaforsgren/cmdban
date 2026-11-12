package ui

import (
	"testing"
)

func TestHelpSectionsDoNotRepeatGeneralOrBoardShortcuts(t *testing.T) {
	sections := helpSections()
	seen := map[shortcut]string{}
	for _, section := range sections {
		for _, sc := range section.shortcuts {
			if previous, ok := seen[sc]; ok {
				t.Errorf("%q <%s> in %q repeats %q", sc.desc, sc.key, section.title, previous)
			}
			seen[sc] = section.title
		}
	}
}

func TestHelpSectionsOmitGeneralKeysOutsideGeneral(t *testing.T) {
	for _, section := range helpSections()[1:] {
		if got := withoutGeneralShortcuts(section.shortcuts); len(got) != len(section.shortcuts) {
			t.Errorf("section %q repeats general keys", section.title)
		}
	}
}

func TestHelpUsesSeveralColumnsOnWideTerminals(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	var blocks []string
	for _, section := range helpSections() {
		blocks = append(blocks, renderHelpSection(section.title, section.shortcuts))
	}

	columns := m.fitHelpColumns(blocks)
	if len(columns) < 3 {
		t.Fatalf("fitHelpColumns() = %d columns at width %d, want at least 3", len(columns), m.width)
	}
	if got := helpColumnsWidth(columns); got > m.width {
		t.Fatalf("help columns are %d wide, more than the terminal's %d", got, m.width)
	}
}

func TestBalanceHelpColumnsKeepsOrderAndFillsColumns(t *testing.T) {
	blocks := []string{"a\na", "b\nb", "c\nc", "d\nd", "e\ne", "f\nf"}

	columns := balanceHelpColumns(blocks, 3)

	if len(columns) != 3 {
		t.Fatalf("got %d columns, want 3", len(columns))
	}
	var flat []string
	for _, column := range columns {
		if len(column) != 2 {
			t.Errorf("column %v, want 2 blocks each", column)
		}
		flat = append(flat, column...)
	}
	for i := range blocks {
		if flat[i] != blocks[i] {
			t.Fatalf("order = %v, want %v", flat, blocks)
		}
	}
}
