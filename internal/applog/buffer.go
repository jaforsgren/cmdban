package applog

import (
	"sync"
	"time"
)

// Capacity is the maximum number of entries the ring buffer retains.
// Once full, the oldest entry is dropped as a new one is added.
const Capacity = 200

// Buffer is a fixed-size, thread-safe ring buffer of log entries. Entries
// are added from bubbletea commands, which bubbletea may run on goroutines
// other than the Update loop, so all access is mutex-guarded.
type Buffer struct {
	mu      sync.Mutex
	entries []Entry
}

func NewBuffer() *Buffer {
	return &Buffer{entries: make([]Entry, 0, Capacity)}
}

func (b *Buffer) add(entry Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = append(b.entries, entry)
	if len(b.entries) > Capacity {
		b.entries = b.entries[len(b.entries)-Capacity:]
	}
}

func (b *Buffer) Error(message string) {
	b.add(Entry{Time: time.Now(), Level: LevelError, Message: message})
}

func (b *Buffer) Info(message string) {
	b.add(Entry{Time: time.Now(), Level: LevelInfo, Message: message})
}

// Entries returns the buffered entries oldest-first.
func (b *Buffer) Entries() []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Entry, len(b.entries))
	copy(out, b.entries)
	return out
}

func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = b.entries[:0]
}
