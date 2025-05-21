package imagecustomizerlib

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitTracer sets up OpenTelemetry tracing with stdout exporter.
func InitTracer() (func(context.Context) error, error) {
	exporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
	}

	osInfo, err := getOSInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get OS info: %w", err)
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
	return tp.Shutdown, nil
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

// getOSInfo retrieves the OS information from /etc/os-release
func getOSInfo() (map[string]string, error) {
	content, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("failed to read /etc/os-release: %w", err)
	}
	return parseOSRelease(string(content)), nil
}
