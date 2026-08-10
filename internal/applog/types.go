package applog

import "time"

type Level string

const (
	LevelInfo  Level = "INFO"
	LevelError Level = "ERROR"
)

type Entry struct {
	Time    time.Time
	Level   Level
	Message string
}
