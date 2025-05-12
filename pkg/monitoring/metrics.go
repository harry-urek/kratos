package monitoring

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// SessionsCreated tracks the number of sessions created
	SessionsCreated = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kratos_sessions_created_total",
			Help: "Total number of sessions created",
		},
	)

	// SessionsValidated tracks the number of session validations
	SessionsValidated = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kratos_sessions_validated_total",
			Help: "Total number of session validations",
		},
	)

	// SessionsInvalidated tracks the number of sessions invalidated
	SessionsInvalidated = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kratos_sessions_invalidated_total",
			Help: "Total number of sessions invalidated",
		},
	)

	// SessionsExpired tracks the number of sessions expired
	SessionsExpired = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kratos_sessions_expired_total",
			Help: "Total number of sessions expired",
		},
	)

	// ActiveSessions tracks the number of active sessions
	ActiveSessions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "kratos_sessions_active",
			Help: "Current number of active sessions",
		},
	)

	// SessionDuration tracks the distribution of session durations
	SessionDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "kratos_session_duration_seconds",
			Help:    "Distribution of session durations",
			Buckets: prometheus.ExponentialBuckets(60, 2, 10), // 1min, 2min, 4min, ..., ~17hours
		},
	)

	// RequestDuration tracks the distribution of request durations
	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kratos_request_duration_seconds",
			Help:    "Distribution of request durations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "status"},
	)

	// ErrorsTotal tracks the total number of errors
	ErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kratos_errors_total",
			Help: "Total number of errors",
		},
		[]string{"type"},
	)
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(
		SessionsCreated,
		SessionsValidated,
		SessionsInvalidated,
		SessionsExpired,
		ActiveSessions,
		SessionDuration,
		RequestDuration,
		ErrorsTotal,
	)
}

// Handler returns an HTTP handler for exposing Prometheus metrics
func Handler() http.Handler {
	return promhttp.Handler()
}

// RecordSessionCreation records a session creation event
func RecordSessionCreation() {
	SessionsCreated.Inc()
	ActiveSessions.Inc()
}

// RecordSessionValidation records a session validation event
func RecordSessionValidation() {
	SessionsValidated.Inc()
}

// RecordSessionInvalidation records a session invalidation event
func RecordSessionInvalidation() {
	SessionsInvalidated.Inc()
	ActiveSessions.Dec()
}

// RecordSessionExpiration records a session expiration event
func RecordSessionExpiration() {
	SessionsExpired.Inc()
	ActiveSessions.Dec()
}

// RecordSessionDuration records a session's duration
func RecordSessionDuration(duration time.Duration) {
	SessionDuration.Observe(duration.Seconds())
}

// RecordRequestDuration records a request's duration
func RecordRequestDuration(method, endpoint string, status int, duration time.Duration) {
	statusStr := http.StatusText(status)
	RequestDuration.WithLabelValues(method, endpoint, statusStr).Observe(duration.Seconds())
}

// RecordError records an error event
func RecordError(errorType string) {
	ErrorsTotal.WithLabelValues(errorType).Inc()
}

// Middleware returns HTTP middleware for recording request metrics
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer that captures the status code
		rw := newResponseWriter(w)

		// Call the next handler
		next.ServeHTTP(rw, r)

		// Record the request duration
		duration := time.Since(start)
		RecordRequestDuration(r.Method, r.URL.Path, rw.statusCode, duration)
	})
}

// responseWriter is a wrapper for http.ResponseWriter that captures the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
