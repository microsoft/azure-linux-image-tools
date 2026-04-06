package exekong

import (
	"strings"

	"github.com/alecthomas/kong"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
)

var (
	KongVars = kong.Vars{
		"logcolorvalues": strings.Join(logger.Colors(), ", ") + ",",
		"loglevelvalues": strings.Join(logger.Levels(), ", ") + ",",
		"formatvalues":   strings.Join(logger.Formats(), ", ") + ",",
	}
)

type LogFlags struct {
	LogColor  string `name:"log-color" placeholder:"(always|auto|never)" help:"Color setting for log terminal output." enum:"${logcolorvalues}" default:""`
	LogFile   string `name:"log-file" help:"Path to the log file."`
	LogLevel  string `name:"log-level" placeholder:"(panic|fatal|error|warn|info|debug|trace)" help:"The minimum log level." enum:"${loglevelvalues}" default:""`
	LogFormat string `name:"log-format" placeholder:"(text|json)" help:"Output format for the log." enum:"${formatvalues}" default:""`
}

func (f LogFlags) AsLoggerFlags() logger.LogFlags {
	return logger.LogFlags{
		LogColor:  f.LogColor,
		LogFile:   f.LogFile,
		LogLevel:  f.LogLevel,
		LogFormat: f.LogFormat,
	}
}
