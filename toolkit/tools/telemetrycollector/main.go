// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"context"
	_ "embed"
	"os"
	"os/signal"

	"github.com/alecthomas/kong"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/toolversion"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/service/telemetry"
	"go.uber.org/zap"
)

//go:embed config.yaml
var defaultConfig string

type RootCmd struct {
	Version kong.VersionFlag `name:"version" help:"Print version information and quit"`
}

func main() {
	cli := &RootCmd{}
	vars := kong.Vars{
		"version": toolversion.ToolVersion,
	}

	_ = kong.Parse(cli,
		vars,
		kong.HelpOptions{
			Compact:   true,
			FlagsLast: true,
		},
		kong.UsageOnError())

	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	collectorSettings := otelcol.CollectorSettings{
		BuildInfo: component.BuildInfo{
			Command:     "telemetrycollector",
			Description: "Image Customizer telemetry collector",
			Version:     toolversion.ToolVersion,
		},
		Factories: func() (otelcol.Factories, error) {
			otlp := otlpreceiver.NewFactory()
			azureMonitor := azuremonitorexporter.NewFactory()
			normalizeArrays := newNormalizationFactory()
			factories := otelcol.Factories{
				Receivers: map[component.Type]receiver.Factory{otlp.Type(): otlp},
				Exporters: map[component.Type]exporter.Factory{azureMonitor.Type(): azureMonitor},
				Processors: map[component.Type]processor.Factory{
					normalizeArrays.Type(): normalizeArrays,
				},
				Telemetry: telemetry.NewFactory(
					func() component.Config { return &struct{}{} },
					telemetry.WithCreateLogger(
						func(ctx context.Context, ls telemetry.LoggerSettings, c component.Config,
						) (*zap.Logger, component.ShutdownFunc, error) {
							logger, err := zap.NewProduction()
							return logger, nil, err
						}),
				),
			}
			return factories, nil
		},
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				URIs:          []string{"yaml:" + defaultConfig},
				DefaultScheme: "env",
				ProviderFactories: []confmap.ProviderFactory{
					envprovider.NewFactory(),
					fileprovider.NewFactory(),
					yamlprovider.NewFactory(),
				},
			},
		},
	}

	collector, err := otelcol.NewCollector(collectorSettings)
	if err != nil {
		return err
	}

	err = collector.Run(ctx)
	if err != nil {
		return err
	}

	return nil
}
