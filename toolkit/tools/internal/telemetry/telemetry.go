package telemetry

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/osinfo"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/imagecustomizerlib"
	autoexport "go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var shutdownFn func(ctx context.Context) error

func InitTelemetry(disableTelemetry bool) error {
	if disableTelemetry || os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		logger.Log.Debug("Disabled telemetry collection")
		return nil
	}

	exporter, err := autoexport.NewSpanExporter(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	distro, version := osinfo.GetDistroAndVersion()

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewSchemaless(
			attribute.String("host.architecture", runtime.GOARCH),
			attribute.String("host.os", distro),
			attribute.String("host.os.version", version),
			attribute.String("imagecustomizer.version", imagecustomizerlib.ToolVersion),
		)),
	)
	otel.SetTracerProvider(tp)
	shutdownFn = tp.Shutdown
	return nil
}

func ShutdownTelemetry(ctx context.Context) error {
	if shutdownFn == nil {
		return nil
	}
	return shutdownFn(ctx)
}
