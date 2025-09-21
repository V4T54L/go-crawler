package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	URLsInQueue prometheus.Gauge
)

func Init() {
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "code"},
	)

	URLsInQueue = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "crawler_urls_in_queue",
			Help: "The current number of URLs in the crawl queue.",
		},
	)
}
```
