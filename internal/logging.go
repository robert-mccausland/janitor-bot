package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

func NewLogger(name string) *slog.Logger {
	if os.Getenv("DEBUG") == "TRUE" {
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}).WithAttrs([]slog.Attr{slog.Any("name", name)}))
	} else {

		return otelslog.NewLogger(name)
	}
}

func SetupOTel(ctx context.Context) (func(context.Context) error, error) {
	if os.Getenv("DEBUG") == "TRUE" {
		return func(context.Context) error { return nil }, nil
	}

	loggerProvider, err := newLoggerProvider(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to setup OTel logging: %v", err)
	}

	global.SetLoggerProvider(loggerProvider)

	return loggerProvider.Shutdown, nil
}

func newLoggerProvider(ctx context.Context) (*log.LoggerProvider, error) {

	stdoutExporter, err := stdoutlog.New()
	if err != nil {
		return nil, fmt.Errorf("unable to create stdout log exporter: %v", err)
	}

	grpcExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to create grpc log exporter: %v", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("janitor-bot"),
			semconv.ServiceVersion(os.Getenv("VERSION")),
			attribute.String("environment", os.Getenv("ENVIRONMENT")),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create otel resource object: %v", err)
	}

	fmt.Printf("%v", res)

	loggerProvider := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(log.NewBatchProcessor(stdoutExporter)),
		log.WithProcessor(log.NewBatchProcessor(grpcExporter)),
	)

	return loggerProvider, nil
}
