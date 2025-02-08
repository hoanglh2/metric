package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	otelMetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/semconv/v1.27.0"
	"log"
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

	r := gin.Default()
	meter := otel.GetMeterProvider().Meter("my-go-app")
	r.Use(NewMetricMiddleware(meter))

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
