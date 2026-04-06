// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Shared logger

package logger

import (
	"io"
	"log"
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
	colorsArray = []string{colorModeAlways, colorModeAuto, colorModeNever}

	// Valid log formats
	formatsArray = []string{formatText, formatJson}
)

const (
	defaultLogFileLevel   = logrus.DebugLevel
	defaultStderrLogLevel = logrus.InfoLevel

	colorModeAuto   = "auto"
	colorModeAlways = "always"
	colorModeNever  = "never"

	formatText = "text"
	formatJson = "json"
)

type LogFlags struct {
	LogColor  string
	LogFile   string
	LogLevel  string
	LogFormat string
}

// initLogFile initializes the common logger with a file
func initLogFile(filePath string, color string, outputFormat string) (err error) {
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

	fileHook = newWriterHook(file, defaultLogFileLevel, useColors, outputFormat)
	Log.Hooks.Add(fileHook)
	Log.SetLevel(defaultLogFileLevel)

	return
}

// InitStderrLog initializes the logger to print to stderr
func InitStderrLog() {
	initStderrLogInternal(colorModeAuto, formatText)
}

// SetStderrLogLevel sets the lowest log level for stderr output
func SetStderrLogLevel(level string) (err error) {
	return setHookLogLevel(stderrHook, level)
}

// InitBestEffort runs InitStderrLog always, and InitLogFile if path is not empty
func InitBestEffort(lf LogFlags) {
	level := lf.LogLevel
	color := lf.LogColor
	path := lf.LogFile
	format := lf.LogFormat

	if level == "" {
		level = defaultStderrLogLevel.String()
	}

	if format == formatJson && color == colorModeAlways {
		log.Fatal("log-color cannot be set to 'always' when log-format is 'json'")
	}

	initStderrLogInternal(color, format)

	if path != "" {
		fatalOnError(initLogFile(path, color, format), "Failed while setting log file (%s).", path)
	}

	fatalOnError(SetStderrLogLevel(level), "Failed while setting log level.")
}

// Levels returns list of strings representing valid log levels.
func Levels() []string {
	return levelsArray
}

// Colors returns list of strings representing valid log colors.
func Colors() []string {
	return colorsArray
}

// Formats returns list of strings representing valid log formats.
func Formats() []string {
	return formatsArray
}

// fatalOnError logs a fatal error and any message strings, then exits (while
// running any cleanup functions registered with the log package)
func fatalOnError(err error, args ...interface{}) {
	if err != nil {
		if len(args) > 0 {
			Log.Errorf(args[0].(string), args[1:]...)
		}
		Log.Fatalln(err)
	}
}

func initStderrLogInternal(color string, outputFormat string) {
	useColors := true
	if color == colorModeNever {
		useColors = false
	}

	Log = logrus.New()

	// By default send all log messages through stderrHook
	stderrHook = newWriterHook(os.Stderr, defaultStderrLogLevel, useColors, outputFormat)
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
