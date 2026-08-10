package ui

import "testing"

func TestParseADOBoardURLBoardURL(t *testing.T) {
	org, project, team, level, isBacklog, err := parseADOBoardURL(
		"https://dev.azure.com/myorg/MyProject/_boards/board/t/MyTeam/Features",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if org != "myorg" || project != "MyProject" || team != "MyTeam" || level != "Features" {
		t.Fatalf("got org=%q project=%q team=%q level=%q", org, project, team, level)
	}
	if isBacklog {
		t.Fatal("expected isBacklog = false for a board URL")
	}
}

func TestParseADOBoardURLBacklogURL(t *testing.T) {
	org, project, team, level, isBacklog, err := parseADOBoardURL(
		"https://dev.azure.com/lfantdevelophub/common/_backlogs/backlog/Sejfa%20Internal/Epics",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if org != "lfantdevelophub" || project != "common" || team != "Sejfa Internal" || level != "Epics" {
		t.Fatalf("got org=%q project=%q team=%q level=%q", org, project, team, level)
	}
	if !isBacklog {
		t.Fatal("expected isBacklog = true for a backlog URL")
	}
}

func TestParseADOBoardURLBacklogURLNoLevel(t *testing.T) {
	org, project, team, level, isBacklog, err := parseADOBoardURL(
		"https://dev.azure.com/myorg/MyProject/_backlogs/backlog/MyTeam",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if org != "myorg" || project != "MyProject" || team != "MyTeam" || level != "" {
		t.Fatalf("got org=%q project=%q team=%q level=%q", org, project, team, level)
	}
	if !isBacklog {
		t.Fatal("expected isBacklog = true for a backlog URL")
	}
}

func TestParseADOBoardURLInvalid(t *testing.T) {
	if _, _, _, _, _, err := parseADOBoardURL("https://example.com/not/an/ado/url"); err == nil {
		t.Fatal("expected error for unrecognised URL")
	}
}
