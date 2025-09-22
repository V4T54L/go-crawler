package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the application.
type Metrics struct {
	CrawledTotal *prometheus.CounterVec
	ErrorsTotal  *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		CrawledTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "crawler_urls_processed_total",
			Help: "The total number of URLs processed",
		}, nil),
		ErrorsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "crawler_errors_total",
			Help: "The total number of errors encountered",
		}, []string{"type"}), // e.g., 'crawl_failed', 'db_save_failed'
	}
}

func (m *Metrics) IncCrawledTotal() {
	m.CrawledTotal.WithLabelValues().Inc()
}

func (m *Metrics) IncErrorsTotal(errorType string) {
	m.ErrorsTotal.WithLabelValues(errorType).Inc()
}
