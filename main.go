package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"strconv"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func NewResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

var latencyHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_duration_seconds",
	Help:    "Duration of HTTP requests",
	Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10},
}, []string{"status", "path", "method"})

var totalRequests = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Number of get requests.",
	},
	[]string{"path"},
)

var postsLatencySummary = prometheus.NewSummary(prometheus.SummaryOpts{
	Name: "post_request_duration_seconds",
	Help: "Duration of requests to https://jsonplaceholder.typicode.com/posts",
	Objectives: map[float64]float64{
		0.5:  0.05,  // Median (50th percentile) with a 5% tolerance
		0.9:  0.01,  // 90th percentile with a 1% tolerance
		0.99: 0.001, // 99th percentile with a 0.1% tolerance
	},
})

var responseStatus = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "response_status",
		Help: "Status of HTTP response",
	},
	[]string{"status"},
)

var httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name: "http_response_time_seconds",
	Help: "Duration of HTTP requests.",
}, []string{"path"})

func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()
		now := time.Now()
		timer := prometheus.NewTimer(httpDuration.WithLabelValues(path))
		rw := NewResponseWriter(w)
		next.ServeHTTP(rw, r)

		postsLatencySummary.Observe(time.Since(now).Seconds())
		statusCode := rw.statusCode
		latencyHistogram.With(prometheus.Labels{
			"method": r.Method, "path": path, "status": strconv.Itoa(statusCode),
		}).Observe(time.Since(now).Seconds())
		responseStatus.WithLabelValues(strconv.Itoa(statusCode)).Inc()
		totalRequests.WithLabelValues(path).Inc()

		timer.ObserveDuration()
	})
}

func init() {
	prometheus.Register(totalRequests)
	prometheus.Register(responseStatus)
	prometheus.Register(httpDuration)
	prometheus.Register(latencyHistogram)
	prometheus.Register(postsLatencySummary)
}

func main() {
	router := mux.NewRouter()
	router.Use(prometheusMiddleware)

	// Prometheus endpoint
	router.Path("/prometheus").Handler(promhttp.Handler())

	router.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Serving static files
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./static/")))

	fmt.Println("Serving requests on port 9000")
	err := http.ListenAndServe(":9000", router)
	log.Fatal(err)
}
