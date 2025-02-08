package main

import (
	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"context"
	"fmt"
	"log"
	"net/http"

	"connectrpc.com/otelconnect/internal/gen/observability/ping/v1"
	"connectrpc.com/otelconnect/internal/gen/observability/ping/v1/pingv1connect"
)

func main() {
	mux := http.NewServeMux()
	otelInterceptor, err := otelconnect.NewInterceptor(otelconnect.WithFilter(func(ctx context.Context, spec connect.Spec) bool {
		return spec.Procedure != "connect.ping.v1.Ping"
	}))

	if err != nil {
		log.Fatal(err)
	}

	// otelconnect.NewInterceptor provides an interceptor that adds tracing and
	// metrics to both clients and handlers. By default, it uses OpenTelemetry's
	// global TracerProvider and MeterProvider, which you can configure by
	// following the OpenTelemetry documentation. If you'd prefer to avoid
	// globals, use otelconnect.WithTracerProvider and
	// otelconnect.WithMeterProvider.
	mux.Handle(pingv1connect.NewPingServiceHandler(
		&pingv1connect.UnimplementedPingServiceHandler{},
		connect.WithInterceptors(otelInterceptor),
	))

	http.ListenAndServe("localhost:8080", mux)
}

func makeRequest() {
	otelInterceptor, err := otelconnect.NewInterceptor()
	if err != nil {
		log.Fatal(err)
	}

	client := pingv1connect.NewPingServiceClient(
		http.DefaultClient,
		"http://localhost:8080",
		connect.WithInterceptors(otelInterceptor),
	)
	resp, err := client.Ping(
		context.Background(),
		connect.NewRequest(&pingv1.PingRequest{}),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)
}
