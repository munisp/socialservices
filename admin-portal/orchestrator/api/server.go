package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metrics
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
	activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_connections",
			Help: "Number of active connections",
		},
	)
	healthStatus = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "health_status",
			Help: "Health status of the service (1 = healthy, 0 = unhealthy)",
		},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(activeConnections)
	prometheus.MustRegister(healthStatus)
	healthStatus.Set(1)
}

type APIServer struct {
	journeyHandler *JourneyHandler
	server         *http.Server
	isShuttingDown atomic.Bool
}

func NewAPIServer() *APIServer {
	return &APIServer{
		journeyHandler: NewJourneyHandler(),
	}
}

func (s *APIServer) Start(addr string) error {
	mux := http.NewServeMux()

	// API routes with metrics middleware
	mux.Handle("/api/offline/", s.metricsMiddleware(http.HandlerFunc(s.handleOffline)))
	mux.Handle("/api/federation/", s.metricsMiddleware(http.HandlerFunc(s.handleFederation)))
	mux.Handle("/api/interop/", s.metricsMiddleware(http.HandlerFunc(s.handleInterop)))
	mux.Handle("/api/loadtest/", s.metricsMiddleware(http.HandlerFunc(s.handleLoadTest)))

	// Journey orchestration routes
	mux.Handle("/api/journeys/", s.metricsMiddleware(http.HandlerFunc(s.journeyHandler.HandleJourneys)))
	mux.Handle("/api/journey-contracts", s.metricsMiddleware(http.HandlerFunc(s.journeyHandler.HandleJourneyContracts)))

	// Health and metrics endpoints
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)
	mux.Handle("/metrics", promhttp.Handler())

	s.server = &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start graceful shutdown handler
	go s.gracefulShutdown()

	log.Printf("Starting API server on %s", addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// metricsMiddleware wraps handlers with Prometheus metrics
func (s *APIServer) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		activeConnections.Inc()
		defer activeConnections.Dec()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		endpoint := strings.Split(r.URL.Path, "/")[2] // Get the API category

		httpRequestsTotal.WithLabelValues(r.Method, endpoint, http.StatusText(wrapped.statusCode)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, endpoint).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// gracefulShutdown handles graceful shutdown on SIGTERM/SIGINT
func (s *APIServer) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	s.isShuttingDown.Store(true)
	healthStatus.Set(0)

	// Give load balancers time to stop sending traffic
	time.Sleep(5 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

// handleReady returns readiness status (for k8s readiness probe)
func (s *APIServer) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.isShuttingDown.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "shutting_down"})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (s *APIServer) handleOffline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/api/offline/")

	switch {
	case path == "status" && r.Method == "GET":
		s.getOfflineStatus(w, r)
	case path == "sync" && r.Method == "POST":
		s.syncOffline(w, r)
	case strings.HasPrefix(path, "changes") && r.Method == "GET":
		s.getOfflineChanges(w, r)
	case path == "devices/register" && r.Method == "POST":
		s.registerDevice(w, r)
	case strings.HasPrefix(path, "conflicts") && r.Method == "GET":
		s.getConflicts(w, r)
	case path == "conflicts/resolve" && r.Method == "POST":
		s.resolveConflict(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func (s *APIServer) getOfflineStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"online":         true,
		"lastSync":       nil,
		"pendingChanges": 0,
		"conflicts":      0,
	}
	json.NewEncoder(w).Encode(status)
}

func (s *APIServer) syncOffline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"deviceId"`
		Changes  []struct {
			ID        string      `json:"id"`
			Type      string      `json:"type"`
			Action    string      `json:"action"`
			Data      interface{} `json:"data"`
			Timestamp int64       `json:"timestamp"`
		} `json:"changes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := map[string]interface{}{
		"success":       true,
		"syncedCount":   len(req.Changes),
		"conflictCount": 0,
		"serverTime":    nil,
	}
	json.NewEncoder(w).Encode(result)
}

func (s *APIServer) getOfflineChanges(w http.ResponseWriter, r *http.Request) {
	changes := []interface{}{}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"changes":    changes,
		"serverTime": nil,
	})
}

func (s *APIServer) registerDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID   string `json:"deviceId"`
		DeviceType string `json:"deviceType"`
		UserID     int    `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"deviceId":   req.DeviceID,
		"registered": true,
	})
}

func (s *APIServer) getConflicts(w http.ResponseWriter, r *http.Request) {
	conflicts := []interface{}{}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"conflicts": conflicts,
	})
}

func (s *APIServer) resolveConflict(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConflictID string      `json:"conflictId"`
		Resolution string      `json:"resolution"`
		MergedData interface{} `json:"mergedData"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"conflictId": req.ConflictID,
		"resolved":   true,
	})
}

func (s *APIServer) handleFederation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/api/federation/")

	switch {
	case path == "providers" && r.Method == "GET":
		s.getProviders(w, r)
	case path == "send-otp" && r.Method == "POST":
		s.sendOTP(w, r)
	case path == "verify" && r.Method == "POST":
		s.verifyIdentity(w, r)
	case strings.HasPrefix(path, "history/") && r.Method == "GET":
		s.getVerificationHistory(w, r)
	case path == "consent/revoke" && r.Method == "POST":
		s.revokeConsent(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func (s *APIServer) getProviders(w http.ResponseWriter, r *http.Request) {
	providers := []map[string]interface{}{
		{"id": "aadhaar", "name": "Aadhaar (India)", "country": "IN", "supportsBiometric": true, "supportsOTP": true},
		{"id": "nin", "name": "NIN (Nigeria)", "country": "NG", "supportsBiometric": true, "supportsOTP": true},
		{"id": "nida", "name": "NIDA (Rwanda)", "country": "RW", "supportsBiometric": true, "supportsOTP": false},
		{"id": "nadra", "name": "NADRA (Pakistan)", "country": "PK", "supportsBiometric": true, "supportsOTP": true},
		{"id": "cpf", "name": "CPF (Brazil)", "country": "BR", "supportsBiometric": false, "supportsOTP": true},
		{"id": "curp", "name": "CURP (Mexico)", "country": "MX", "supportsBiometric": false, "supportsOTP": true},
		{"id": "npr", "name": "NPR (Nepal)", "country": "NP", "supportsBiometric": true, "supportsOTP": true},
		{"id": "dukcapil", "name": "Dukcapil (Indonesia)", "country": "ID", "supportsBiometric": true, "supportsOTP": true},
		{"id": "rut", "name": "RUT (Chile)", "country": "CL", "supportsBiometric": false, "supportsOTP": true},
		{"id": "ssn", "name": "SSN (USA)", "country": "US", "supportsBiometric": false, "supportsOTP": false},
		{"id": "nhif", "name": "NHIF (Kenya)", "country": "KE", "supportsBiometric": true, "supportsOTP": true},
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"providers": providers})
}

func (s *APIServer) sendOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"providerId"`
		NationalID string `json:"nationalId"`
		Phone      string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"otpSent":   true,
		"expiresIn": 300,
	})
}

func (s *APIServer) verifyIdentity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID    string `json:"providerId"`
		NationalID    string `json:"nationalId"`
		OTP           string `json:"otp"`
		BiometricData string `json:"biometricData"`
		Consent       struct {
			DataSharing      bool `json:"dataSharing"`
			BiometricCapture bool `json:"biometricCapture"`
			TermsAccepted    bool `json:"termsAccepted"`
		} `json:"consent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := map[string]interface{}{
		"verified":   true,
		"matchScore": 0.95,
		"demographics": map[string]interface{}{
			"name":        "John Doe",
			"dateOfBirth": "1990-01-15",
			"gender":      "male",
			"address":     "123 Main St, City",
		},
		"verifiedAt": nil,
	}
	json.NewEncoder(w).Encode(result)
}

func (s *APIServer) getVerificationHistory(w http.ResponseWriter, r *http.Request) {
	history := []interface{}{}
	json.NewEncoder(w).Encode(map[string]interface{}{"history": history})
}

func (s *APIServer) revokeConsent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BeneficiaryID string `json:"beneficiaryId"`
		ProviderID    string `json:"providerId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"revoked": true,
	})
}

func (s *APIServer) handleInterop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/api/interop/")

	switch {
	case strings.HasPrefix(path, "profile/") && strings.HasSuffix(path, "/refresh") && r.Method == "POST":
		s.refreshProfile(w, r)
	case strings.HasPrefix(path, "profile/") && r.Method == "GET":
		s.getProfile(w, r)
	case strings.HasPrefix(path, "consents/") && r.Method == "GET":
		s.getConsents(w, r)
	case path == "consent/grant" && r.Method == "POST":
		s.grantConsent(w, r)
	case path == "consent/revoke" && r.Method == "POST":
		s.revokeInteropConsent(w, r)
	case path == "sectors" && r.Method == "GET":
		s.getSectors(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func (s *APIServer) getProfile(w http.ResponseWriter, r *http.Request) {
	profile := map[string]interface{}{
		"beneficiaryId": "123",
		"health":        nil,
		"education":     nil,
		"tax":           nil,
		"labor":         nil,
		"lastUpdated":   nil,
	}
	json.NewEncoder(w).Encode(profile)
}

func (s *APIServer) getConsents(w http.ResponseWriter, r *http.Request) {
	consents := []interface{}{}
	json.NewEncoder(w).Encode(map[string]interface{}{"consents": consents})
}

func (s *APIServer) grantConsent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BeneficiaryID string `json:"beneficiaryId"`
		Sector        string `json:"sector"`
		Purpose       string `json:"purpose"`
		ExpiresAt     string `json:"expiresAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"consentId": "consent-123",
		"granted":   true,
	})
}

func (s *APIServer) revokeInteropConsent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BeneficiaryID string `json:"beneficiaryId"`
		Sector        string `json:"sector"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"revoked": true,
	})
}

func (s *APIServer) refreshProfile(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"refreshed": true,
	})
}

func (s *APIServer) getSectors(w http.ResponseWriter, r *http.Request) {
	sectors := []map[string]interface{}{
		{"id": "health", "name": "Health", "description": "Health records and insurance"},
		{"id": "education", "name": "Education", "description": "Education records and enrollment"},
		{"id": "tax", "name": "Tax", "description": "Tax records and filings"},
		{"id": "labor", "name": "Labor", "description": "Employment and labor records"},
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"sectors": sectors})
}

func (s *APIServer) handleLoadTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/api/loadtest/")

	switch {
	case path == "history" && r.Method == "GET":
		s.getLoadTestHistory(w, r)
	case path == "start" && r.Method == "POST":
		s.startLoadTest(w, r)
	case strings.HasSuffix(path, "/status") && r.Method == "GET":
		s.getLoadTestStatus(w, r)
	case strings.HasSuffix(path, "/metrics") && r.Method == "GET":
		s.getLoadTestMetrics(w, r)
	case strings.HasSuffix(path, "/stop") && r.Method == "POST":
		s.stopLoadTest(w, r)
	case strings.HasSuffix(path, "/report") && r.Method == "GET":
		s.getLoadTestReport(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func (s *APIServer) getLoadTestHistory(w http.ResponseWriter, r *http.Request) {
	history := []interface{}{}
	json.NewEncoder(w).Encode(map[string]interface{}{"history": history})
}

func (s *APIServer) startLoadTest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		ScalePreset string   `json:"scalePreset"`
		TestType    string   `json:"testType"`
		Scenarios   []string `json:"scenarios"`
		Duration    int      `json:"duration"`
		TargetRPS   int      `json:"targetRPS"`
		SLOConfig   struct {
			MaxLatencyP50  int     `json:"maxLatencyP50"`
			MaxLatencyP95  int     `json:"maxLatencyP95"`
			MaxLatencyP99  int     `json:"maxLatencyP99"`
			MinSuccessRate float64 `json:"minSuccessRate"`
			MaxErrorRate   float64 `json:"maxErrorRate"`
		} `json:"sloConfig"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"testId":  "test-123",
		"started": true,
	})
}

func (s *APIServer) getLoadTestStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"testId":   "test-123",
		"status":   "running",
		"progress": 50,
	}
	json.NewEncoder(w).Encode(status)
}

func (s *APIServer) getLoadTestMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"testId":        "test-123",
		"currentRPS":    1000,
		"latencyP50":    50,
		"latencyP95":    150,
		"latencyP99":    300,
		"successRate":   99.5,
		"errorRate":     0.5,
		"activeUsers":   100,
		"totalRequests": 50000,
	}
	json.NewEncoder(w).Encode(metrics)
}

func (s *APIServer) stopLoadTest(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"stopped": true,
	})
}

func (s *APIServer) getLoadTestReport(w http.ResponseWriter, r *http.Request) {
	report := map[string]interface{}{
		"testId":  "test-123",
		"summary": "Test completed successfully",
		"sloResults": map[string]bool{
			"latencyP50":  true,
			"latencyP95":  true,
			"latencyP99":  true,
			"successRate": true,
			"errorRate":   true,
		},
	}
	json.NewEncoder(w).Encode(report)
}
