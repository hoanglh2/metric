package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelMetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.27.0"
	"log"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"time"

	"go.opentelemetry.io/otel"
)

func main() {
	// Set up OTLP exporter for metrics
	ctx := context.Background()
	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithInsecure(),
		otlpmetrichttp.WithEndpoint("otel-collector:4317"),
	) // OTLP export via HTTP (default)
	if err != nil {
		log.Fatal(err)
	}
	// Set up OTLP exporter for traces
	exp, err := otlptrace.New(ctx, otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint("otel-collector:4316"),
		otlptracehttp.WithInsecure(),
	))
	if err != nil {
		log.Fatal(err)
	}
	// Create a MeterProvider with the OTLP exporter
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(
			exporter,
			metric.WithInterval(5*time.Second),
			metric.WithTimeout(2*time.Second)),
		),
		metric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("my-go-app"),
		)),
	)
	defer func() {
		if err = meterProvider.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(exp),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("my-go-app"),
		)),
	)
	defer func() {
		if err = traceProvider.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	// Set the global meter provider
	otel.SetMeterProvider(meterProvider)
	otel.SetTracerProvider(traceProvider)

	r := gin.Default()

	r.Use(NewMetricMiddleware(otel.GetMeterProvider().Meter("my-go-app")))
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.GET("/hello", func(c *gin.Context) {
		time.Sleep(1 * time.Second)
		c.JSON(200, gin.H{
			"message": "hello",
		})
	})
	r.GET("/world", func(c *gin.Context) {
		time.Sleep(2 * time.Second)
		c.JSON(200, gin.H{
			"message": "world",
		})
	})
	err = r.Run(":9000")
	if err != nil {
		panic(err)
	}
}

func NewMetricMiddleware(meter otelMetric.Meter) gin.HandlerFunc {
	return func(c *gin.Context) {
		durationHistogram, _ := meter.Int64Histogram("http.server.latency",
			otelMetric.WithUnit("ms"),
			otelMetric.WithExplicitBucketBoundaries(0.5, 0.9, 0.95, 0.99),
		)

		initialTime := time.Now()

		c.Next()
		duration := time.Since(initialTime)

		pathTemplate := c.Request.URL.Path
		metricAttributes := attribute.NewSet(
			semconv.URLPath(pathTemplate),
			semconv.HTTPRequestMethodKey.String(c.Request.Method),
			semconv.HTTPResponseStatusCode(c.Writer.Status()),
		)

		durationHistogram.Record(
			c,
			duration.Milliseconds(),
			otelMetric.WithAttributeSet(metricAttributes),
		)
	}
}
