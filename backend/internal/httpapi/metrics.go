package httpapi

import (
	"crypto/subtle"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

var (
	requestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Requests handled, by method, matched route pattern and status class.",
	}, []string{"method", "route", "status"})

	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Request latency by method and matched route pattern.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"method", "route"})

	inFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Requests currently being handled.",
	})
)

func init() {
	prometheus.MustRegister(requestsTotal, requestDuration, inFlight)
}

// Metrics records one observation per request. The route label is the
// registered pattern (/v1/bookings/:id), never the raw path, so cardinality
// stays bounded; unmatched paths collapse into one bucket.
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		inFlight.Inc()
		c.Next()
		inFlight.Dec()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		requestsTotal.WithLabelValues(c.Request.Method, route, strconv.Itoa(c.Writer.Status())).Inc()
		requestDuration.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
	}
}

// MetricsHandler serves the Prometheus text exposition. With a non-empty
// token it requires `Authorization: Bearer <token>`; without one it is open,
// which is fine only when the port isn't reachable from the internet.
func MetricsHandler(token string) gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		if token != "" {
			got := c.GetHeader("Authorization")
			if subtle.ConstantTimeCompare([]byte(got), []byte("Bearer "+token)) != 1 {
				web.Unauthorized(c, "A valid metrics token is required.")
				return
			}
		}
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// poolCollector exposes pgxpool.Stat as gauges, sampled on each scrape.
type poolCollector struct {
	pool                                     *pgxpool.Pool
	total, idle, acquired, constructing, max *prometheus.Desc
	acquireCount, emptyAcquire               *prometheus.Desc
}

// RegisterPoolMetrics adds the database pool gauges. Safe to skip (nil pool).
func RegisterPoolMetrics(pool *pgxpool.Pool) {
	if pool == nil {
		return
	}
	prometheus.MustRegister(&poolCollector{
		pool:         pool,
		total:        prometheus.NewDesc("pgxpool_total_conns", "Connections open, idle or acquired.", nil, nil),
		idle:         prometheus.NewDesc("pgxpool_idle_conns", "Connections idle in the pool.", nil, nil),
		acquired:     prometheus.NewDesc("pgxpool_acquired_conns", "Connections checked out.", nil, nil),
		constructing: prometheus.NewDesc("pgxpool_constructing_conns", "Connections being opened.", nil, nil),
		max:          prometheus.NewDesc("pgxpool_max_conns", "Configured pool ceiling.", nil, nil),
		acquireCount: prometheus.NewDesc("pgxpool_acquire_total", "Successful acquires since start.", nil, nil),
		emptyAcquire: prometheus.NewDesc("pgxpool_empty_acquire_total", "Acquires that had to wait for a connection.", nil, nil),
	})
}

func (p *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{p.total, p.idle, p.acquired, p.constructing, p.max, p.acquireCount, p.emptyAcquire} {
		ch <- d
	}
}

func (p *poolCollector) Collect(ch chan<- prometheus.Metric) {
	s := p.pool.Stat()
	ch <- prometheus.MustNewConstMetric(p.total, prometheus.GaugeValue, float64(s.TotalConns()))
	ch <- prometheus.MustNewConstMetric(p.idle, prometheus.GaugeValue, float64(s.IdleConns()))
	ch <- prometheus.MustNewConstMetric(p.acquired, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(p.constructing, prometheus.GaugeValue, float64(s.ConstructingConns()))
	ch <- prometheus.MustNewConstMetric(p.max, prometheus.GaugeValue, float64(s.MaxConns()))
	ch <- prometheus.MustNewConstMetric(p.acquireCount, prometheus.CounterValue, float64(s.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(p.emptyAcquire, prometheus.CounterValue, float64(s.EmptyAcquireCount()))
}
