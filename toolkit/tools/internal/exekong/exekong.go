package exekong

import (
	"strings"

	"github.com/alecthomas/kong"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
)

var (
	KongVars = kong.Vars{
		"logcolorhelp":   logger.ColorFlagHelp,
		"logcolorvalues": strings.Join(logger.Colors(), ", ") + ",",
		"logfilehelp":    logger.FileFlagHelp,
		"loglevelhelp":   logger.LevelsHelp,
		"loglevelvalues": strings.Join(logger.Levels(), ", ") + ",",
	}
)

type LogFlags struct {
	LogColor string `name:"log-color" placeholder:"(always|auto|never)" help:"${logcolorhelp}" enum:"${logcolorvalues}" default:""`
	LogFile  string `name:"log-file" help:"${logfilehelp}"`
	LogLevel string `name:"log-level" placeholder:"(panic|fatal|error|warn|info|debug|trace)" help:"${loglevelhelp}" enum:"${loglevelvalues}" default:""`
}

func (f LogFlags) AsLoggerFlags() logger.LogFlags {
	return logger.LogFlags{
		LogColor: &f.LogColor,
		LogFile:  &f.LogFile,
		LogLevel: &f.LogLevel,
	}
}
