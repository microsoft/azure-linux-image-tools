package telemetry

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/logger"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/osinfo"
	autoexport "go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

var shutdownFn func(ctx context.Context) error

func InitTelemetry(disableTelemetry bool, toolVersion string) error {
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

	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("imagecustomizer"),
			semconv.ServiceVersionKey.String(toolVersion),
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

// ForceFlush attempts to flush any pending spans to the exporter
func ForceFlush(ctx context.Context) error {
	err := otel.GetTracerProvider().(*sdktrace.TracerProvider).ForceFlush(ctx)
	if err != nil {
		return err
	}
	return nil
}

func ShutdownTelemetry(ctx context.Context) error {
	if shutdownFn == nil {
		return nil
	}

	if err := ForceFlush(ctx); err != nil {
		logger.Log.Warnf("Failed to flush telemetry spans: %v", err)
	}

	return shutdownFn(ctx)
}
