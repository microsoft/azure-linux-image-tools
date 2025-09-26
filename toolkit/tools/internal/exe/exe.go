// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package exe defines QoL functions to simplify and unify creating executables
package exe

import (
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"gopkg.in/alecthomas/kingpin.v2"
)

func SetupLogFlags(k *kingpin.Application) *logger.LogFlags {
	lf := &logger.LogFlags{}
	lf.LogColor = k.Flag(logger.ColorFlag, logger.ColorFlagHelp).PlaceHolder(logger.ColorsPlaceholder).Enum(logger.Colors()...)
	lf.LogFile = k.Flag(logger.FileFlag, logger.FileFlagHelp).String()
	lf.LogLevel = k.Flag(logger.LevelsFlag, logger.LevelsHelp).PlaceHolder(logger.LevelsPlaceholder).Enum(logger.Levels()...)
	return lf
}

type ProfileFlags struct {
	EnableCpuProf *bool
	EnableMemProf *bool
	EnableTrace   *bool
	CpuProfFile   *string
	MemProfFile   *string
	TraceFile     *string
}

func SetupProfileFlags(k *kingpin.Application) *ProfileFlags {
	p := &ProfileFlags{}
	p.EnableCpuProf = k.Flag("enable-cpu-prof", "Enable CPU pprof data collection.").Bool()
	p.EnableMemProf = k.Flag("enable-mem-prof", "Enable Memory pprof data collection.").Bool()
	p.EnableTrace = k.Flag("enable-trace", "Enable trace data collection.").Bool()
	p.CpuProfFile = k.Flag("cpu-prof-file", "File that stores CPU pprof data.").String()
	p.MemProfFile = k.Flag("mem-prof-file", "File that stores Memory pprof data.").String()
	p.TraceFile = k.Flag("trace-file", "File that stores trace data.").String()
	return p
}
