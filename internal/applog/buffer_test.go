package applog

import "testing"

func TestBufferEntriesOrder(t *testing.T) {
	b := NewBuffer()
	b.Info("first")
	b.Error("second")

	entries := b.Entries()
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Message != "first" || entries[0].Level != LevelInfo {
		t.Fatalf("entries[0] = %+v", entries[0])
	}
	if entries[1].Message != "second" || entries[1].Level != LevelError {
		t.Fatalf("entries[1] = %+v", entries[1])
	}
}

func TestBufferDropsOldestBeyondCapacity(t *testing.T) {
	b := NewBuffer()
	for i := 0; i < Capacity+10; i++ {
		b.Info("entry")
	}

	entries := b.Entries()
	if len(entries) != Capacity {
		t.Fatalf("len(entries) = %d, want %d", len(entries), Capacity)
	}
}

func TestBufferClear(t *testing.T) {
	b := NewBuffer()
	b.Error("boom")
	b.Clear()

	if got := len(b.Entries()); got != 0 {
		t.Fatalf("len(entries) after clear = %d, want 0", got)
	}
}
