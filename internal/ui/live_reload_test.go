package ui

import (
	"strings"
	"testing"
)

func TestLiveReloadIsOffByDefaultAndRToggles(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	if m.liveReload {
		t.Fatal("liveReload = true by default, want false")
	}

	got, cmd := m.Update(keyMsgFor("R"))
	m = got.(Model)
	if !m.liveReload || cmd == nil {
		t.Fatalf("liveReload = %v, cmd = %v after R; want on with a tick scheduled", m.liveReload, cmd)
	}

	m = pressKey(t, m, "R")
	if m.liveReload {
		t.Fatal("liveReload = true after second R, want off")
	}
}

func TestLiveReloadTickSyncsOnlyForCurrentLoop(t *testing.T) {
	m := pressKey(t, adoTestModel(adoTestConfig()), "R")

	if _, cmd := m.Update(liveReloadTickMsg{generation: m.liveReloadGeneration}); cmd == nil {
		t.Fatal("current tick returned no command, want a sync and the next tick")
	}
	if _, cmd := m.Update(liveReloadTickMsg{generation: m.liveReloadGeneration - 1}); cmd != nil {
		t.Fatal("stale tick returned a command, want it dropped")
	}

	m = pressKey(t, m, "R")
	if _, cmd := m.Update(liveReloadTickMsg{generation: m.liveReloadGeneration}); cmd != nil {
		t.Fatal("tick after turning live reload off returned a command, want none")
	}
}

func TestSyncedLineMarksLiveReload(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	if strings.Contains(m.syncedLine(), "live") {
		t.Fatal("synced line mentions live reload while off")
	}

	m.liveReload = true
	line := m.syncedLine()
	if !strings.Contains(line, "Synced:") || !strings.Contains(line, "live") {
		t.Fatalf("synced line = %q, want it marked live", line)
	}
	if !strings.Contains(m.renderTopBar(), "· live") {
		t.Fatal("topbar does not show the live reload marker")
	}
}

func TestBackgroundSyncKeepsFooterMessage(t *testing.T) {
	m := adoTestModel(adoTestConfig())
	m.message = "Linked PR #20"

	got, _ := m.Update(tasksLoadedMsg{background: true})
	if got.(Model).message != "Linked PR #20" {
		t.Fatalf("message = %q after background sync, want it kept", got.(Model).message)
	}

	got, _ = m.Update(tasksLoadedMsg{})
	if got.(Model).message != "" {
		t.Fatalf("message = %q after a manual sync, want it cleared", got.(Model).message)
	}
}
