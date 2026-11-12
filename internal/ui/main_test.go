package ui

import (
	"os"
	"testing"
)

// TestMain points HOME at a temp dir so models never read or write the
// user's real cmdban state files (such as comment read state).
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "cmdban-ui-test-home-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", home)
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
