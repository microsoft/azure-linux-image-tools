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
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

var shutdownFn func(ctx context.Context) error

func InitTelemetry(disableTelemetry bool) error {
	if disableTelemetry {
		logger.Log.Info("Disabled telemetry collection")
		return nil
	} else if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		logger.Log.Debug("No OTLP endpoint set, telemetry will not be collected")
		return nil
	}

	exporter, err := autoexport.NewSpanExporter(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	distro, version := osinfo.GetDistroAndVersion()

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("imagecustomizer"),
			semconv.ServiceVersionKey.String(imagecustomizerlib.ToolVersion),
			attribute.String("host.architecture", runtime.GOARCH),
			attribute.String("host.os", distro),
			attribute.String("host.os.version", version),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
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
