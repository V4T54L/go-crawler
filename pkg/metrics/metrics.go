package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	URLsInQueue         prometheus.Gauge
	CrawlsTotal         *prometheus.CounterVec   // Added from attempted content
	CrawlDuration       *prometheus.HistogramVec // Added from attempted content
)

func Init() {
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"}, // Changed 'code' to 'status' from attempted content
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"}, // Changed 'code' to 'status' from attempted content
	)

	URLsInQueue = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "urls_in_queue", // Changed name from 'crawler_urls_in_queue' from attempted content
			Help: "Current number of URLs in the crawl queue.",
		},
	)

	CrawlsTotal = promauto.NewCounterVec( // Added from attempted content
		prometheus.CounterOpts{
			Name: "crawls_total",
			Help: "Total number of crawl attempts.",
		},
		[]string{"status", "error_type"}, // status: success, failure
	)

	CrawlDuration = promauto.NewHistogramVec( // Added from attempted content
		prometheus.HistogramOpts{
			Name:    "crawl_duration_seconds",
			Help:    "Duration of crawl operations.",
			Buckets: []float64{1, 5, 10, 15, 30, 60, 120}, // Adopted specific buckets
		},
		[]string{"domain"},
	)
}
