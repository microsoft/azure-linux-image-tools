package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var (
	// OTEL Tracer used across the tool for telemetry
	Tracer trace.Tracer

	// shutdownFn used to shutdown the tracer provider
	shutdownFn func(ctx context.Context) error
)

func InitTelemetry() error {
	// exporter, err := stdouttrace.New(
	// 	stdouttrace.WithPrettyPrint(),
	// )
	// if err != nil {
	// 	return fmt.Errorf("failed to create exporter: %w", err)
	// }

	exporter, err := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint("localhost:4317"),
	)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	osInfo, err := getOSInfo()
	if err != nil {
		return fmt.Errorf("failed to get OS info: %w", err)
	}
	// create tracer provider with batcher and resource attributes
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewSchemaless(
			attribute.String("host.architecture", runtime.GOARCH),
			attribute.String("host.os", osInfo["ID"]),
			attribute.String("host.os.version", osInfo["VERSION_ID"]),
			attribute.String("imagecustomizer.version", ToolVersion),
		)),
	)
	otel.SetTracerProvider(tp)
	Tracer = tp.Tracer("imagecustomizer")
	shutdownFn = tp.Shutdown
	return nil
}

// ShutdownTelemetry flushes any buffered spans and shuts down the tracing provider
func ShutdownTelemetry(ctx context.Context) error {
	if shutdownFn == nil {
		return nil
	}
	return shutdownFn(ctx)
}

func getOSInfo() (map[string]string, error) {
	content, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("failed to read /etc/os-release: %w", err)
	}
	return parseOSRelease(string(content)), nil
}

func parseOSRelease(content string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
		result[key] = val
	}
	return result
}
