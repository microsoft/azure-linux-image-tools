// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package logger

import (
	"fmt"
	"io"
	"regexp"
	"sync"

	"github.com/sirupsen/logrus"
)

// writerHook is a hook to handle writing to a writer at a custom log level
type writerHook struct {
	lock      sync.Mutex
	level     logrus.Level
	writer    io.Writer
	formatter logrus.Formatter
	useColors bool
}

var (

	// colorCodeRegex is of type '\x1b[0m' or '\x1b[31m', etc.
	colorCodeRegex = regexp.MustCompile(`\x1b\[[0-9]+m`)
)

// newWriterHook returns new writerHook
func newWriterHook(writer io.Writer, level logrus.Level, useColors bool) *writerHook {
	formatter := &logrus.TextFormatter{
		ForceColors: useColors,
	}

	return &writerHook{
		level:     level,
		writer:    writer,
		formatter: formatter,
		useColors: useColors,
	}
}

// Fire writes the log entry to the writer
func (h *writerHook) Fire(entry *logrus.Entry) (err error) {
	// Filter out entries that are at a higher level (more verbose) than the current filter
	if entry.Level > h.level {
		return
	}

	if !h.useColors {
		entry.Message = colorCodeRegex.ReplaceAllString(entry.Message, "")
	}

	h.lock.Lock()
	defer h.lock.Unlock()

	msg, err := h.formatter.Format(entry)
	if err != nil {
		return
	}

	_, err = fmt.Fprint(h.writer, string(msg))
	return
}

// SetLevel sets the lowest log level
func (h *writerHook) SetLevel(level logrus.Level) {
	h.level = level
}

// Levels returns configured log levels
func (h *writerHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// CurrentLevel returns the current log level for the hook
func (h *writerHook) CurrentLevel() logrus.Level {
	return h.level
}
