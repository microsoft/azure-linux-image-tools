// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Shared logger

package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

var (
	// Log contains the shared Logger
	Log *logrus.Logger

	stderrHook *writerHook
	fileHook   *writerHook

	// Valid log levels
	levelsArray = []string{"panic", "fatal", "error", "warn", "info", "debug", "trace"}

	// Valid log colors
	colorsArray = []string{"always", "auto", "never"}
)

const (
	// LevelsPlaceholder are all valid log levels separated by '|' character.
	LevelsPlaceholder = "(panic|fatal|error|warn|info|debug|trace)"

	// LevelsFlag is the suggested name of the flag for loglevel
	LevelsFlag = "log-level"

	// LevelsHelp is the suggested help message for the loglevel flag
	LevelsHelp = "The minimum log level."

	// FileFlag is the suggested name for logfile flag
	FileFlag = "log-file"

	// FileFlagHelp is the suggested help message for the logfile flag
	FileFlagHelp = "Path to the image's log file."

	// ColorsPlaceholder are all valid log colors separated by '|' character.
	ColorsPlaceholder = "(always|auto|never)"

	// ColorFlag is the suggested name for logcolor flag
	ColorFlag = "log-color"

	// ColorFlagHelp is the suggested help message for the logcolor flag
	ColorFlagHelp = "Color setting for log terminal output."

	defaultLogFileLevel   = logrus.DebugLevel
	defaultStderrLogLevel = logrus.InfoLevel
	colorModeAuto         = "auto"
	colorModeAlways       = "always"
	colorModeNever        = "never"
)

type LogFlags struct {
	LogColor *string
	LogFile  *string
	LogLevel *string
}

// initLogFile initializes the common logger with a file
func initLogFile(filePath string, color string) (err error) {
	useColors := false
	if color == colorModeAlways {
		useColors = true
	}

	err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		return
	}

	file, err := os.Create(filePath)
	if err != nil {
		return
	}

	fileHook = newWriterHook(file, defaultLogFileLevel, useColors)
	Log.Hooks.Add(fileHook)
	Log.SetLevel(defaultLogFileLevel)

	return
}

// InitStderrLog initializes the logger to print to stderr
func InitStderrLog() {
	initStderrLogInternal(colorModeAuto)
}

// SetFileLogLevel sets the lowest log level for file output
func SetFileLogLevel(level string) (err error) {
	return setHookLogLevel(fileHook, level)
}

// SetStderrLogLevel sets the lowest log level for stderr output
func SetStderrLogLevel(level string) (err error) {
	return setHookLogLevel(stderrHook, level)
}

// InitBestEffort runs InitStderrLog always, and InitLogFile if path is not empty
func InitBestEffort(lf *LogFlags) {
	level := *lf.LogLevel
	color := *lf.LogColor
	path := *lf.LogFile

	if level == "" {
		level = defaultStderrLogLevel.String()
	}

	initStderrLogInternal(color)

	if path != "" {
		PanicOnError(initLogFile(path, color), "Failed while setting log file (%s).", path)
	}

	PanicOnError(SetStderrLogLevel(level), "Failed while setting log level.")
}

// Levels returns list of strings representing valid log levels.
func Levels() []string {
	return levelsArray
}

// Colors returns list of strings representing valid log colors.
func Colors() []string {
	return colorsArray
}

// PanicOnError logs the error and any message strings and then panics
func PanicOnError(err interface{}, args ...interface{}) {
	if err != nil {
		if len(args) > 0 {
			Log.Errorf(args[0].(string), args[1:]...)
		}

		Log.Panicln(err)
	}
}

func initStderrLogInternal(color string) {
	useColors := true
	if color == colorModeNever {
		useColors = false
	}

	Log = logrus.New()

	// By default send all log messages through stderrHook
	stderrHook = newWriterHook(os.Stderr, defaultStderrLogLevel, useColors)
	Log.AddHook(stderrHook)
	Log.SetLevel(defaultStderrLogLevel)
	Log.SetOutput(io.Discard)
}

func setHookLogLevel(hook *writerHook, level string) (err error) {
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		return
	}

	// Update the base logger level if its not at least equal to the hook level
	// Otherwise the hook will not receive any entries
	if logLevel > hook.CurrentLevel() {
		Log.SetLevel(logLevel)
	}

	hook.SetLevel(logLevel)
	return
}
