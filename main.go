package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	otelMetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.27.0"
	otelTrace "go.opentelemetry.io/otel/trace"
	"log"
	"os"

	"time"

	"go.opentelemetry.io/otel"
)

func main() {

	// Set up OTLP exporter for metrics
	ctx := context.Background()
	otelPromExporter, err := prometheus.New()
	if err != nil {
		panic(err)
	}
	meterProvider := metric.NewMeterProvider(metric.WithReader(otelPromExporter))
	otel.SetMeterProvider(meterProvider)

	defer func() {
		if err = meterProvider.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		panic("OTEL_EXPORTER_OTLP_ENDPOINT environment variable not set")
	}
	fmt.Println("endpoint: ", endpoint)
	traceExporter, err := otlptrace.New(ctx, otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	))
	if err != nil {
		log.Fatal(err)
	}
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			semconv.ServiceName("my-go-app"),
			semconv.ServiceVersion("v0.1.0"),
		),
	)

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter,
			trace.WithBatchTimeout(2*time.Second),
		),
		trace.WithResource(res),
		trace.WithSampler(trace.AlwaysSample()),
	)
	otel.SetTracerProvider(traceProvider)

	r := gin.Default()
	meter := otel.GetMeterProvider().Meter("my-go-app")
	tracer := otel.GetTracerProvider().Tracer("my-go-app")
	r.Use(NewMetricMiddleware(meter))
	r.Use(NewTraceMiddleware(tracer))

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.GET("/hello", func(c *gin.Context) {
		time.Sleep(500 * time.Millisecond)
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
			otelMetric.WithExplicitBucketBoundaries(5, 10, 25, 50, 75, 100, 250, 500, 1000, 2500, 5000),
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

func NewTraceMiddleware(tracer otelTrace.Tracer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start a span
		_, span := tracer.Start(c, "HTTP Request")
		defer span.End()

		// Set the span name to the path of the request
		span.SetAttributes(semconv.HTTPRouteKey.String(c.Request.URL.Path))

		c.Next()
	}
}
