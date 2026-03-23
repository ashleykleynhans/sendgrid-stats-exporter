package collector

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chatwork/sendgrid-stats-exporter/internal/sendgrid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func newTestCollector(healthURL, statsURL string) *Collector {
	return newTestCollectorWithConfig(healthURL, statsURL, Config{UserName: "test-user"})
}

func newTestCollectorWithConfig(healthURL, statsURL string, config Config) *Collector {
	sendgrid.HealthEndpoint = healthURL
	sendgrid.StatsEndpoint = statsURL

	client := sendgrid.NewClient("test-api-key")
	return New(slog.Default(), client, config)
}

func drainMetrics(ch chan prometheus.Metric) []prometheus.Metric {
	var metrics []prometheus.Metric
	for {
		select {
		case m := <-ch:
			metrics = append(metrics, m)
		default:
			return metrics
		}
	}
}

func findMetric(metrics []prometheus.Metric, name string) *dto.Metric {
	for _, m := range metrics {
		d := m.Desc().String()
		pb := &dto.Metric{}
		m.Write(pb)
		if strings.Contains(d, name) {
			return pb
		}
	}
	return nil
}

func TestDescribe(t *testing.T) {
	c := newTestCollector("http://localhost", "http://localhost")

	ch := make(chan *prometheus.Desc, 50)
	c.Describe(ch)
	close(ch)

	var descs []*prometheus.Desc
	for d := range ch {
		descs = append(descs, d)
	}

	expected := 18
	if len(descs) != expected {
		t.Errorf("expected %d descriptors, got %d", expected, len(descs))
	}

	foundUp := false
	foundAuth := false
	for _, d := range descs {
		s := d.String()
		if strings.Contains(s, "api_up") {
			foundUp = true
		}
		if strings.Contains(s, "api_auth_ok") {
			foundAuth = true
		}
	}
	if !foundUp {
		t.Error("expected api_up descriptor")
	}
	if !foundAuth {
		t.Error("expected api_auth_ok descriptor")
	}
}

func TestCollect_HealthyWithStats(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	statsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"date":"2024-01-01","stats":[{"metrics":{"requests":5,"delivered":3,"bounces":1}}]}]`))
	}))
	defer statsSrv.Close()

	c := newTestCollector(healthSrv.URL, statsSrv.URL)
	ch := make(chan prometheus.Metric, 50)
	c.Collect(ch)

	metrics := drainMetrics(ch)

	if len(metrics) != 18 {
		t.Errorf("expected 18 metrics, got %d", len(metrics))
	}

	upMetric := findMetric(metrics, "api_up")
	if upMetric == nil {
		t.Fatal("api_up metric not found")
	}
	if upMetric.GetGauge().GetValue() != 1 {
		t.Errorf("expected api_up=1, got %f", upMetric.GetGauge().GetValue())
	}

	authMetric := findMetric(metrics, "api_auth_ok")
	if authMetric == nil {
		t.Fatal("api_auth_ok metric not found")
	}
	if authMetric.GetGauge().GetValue() != 1 {
		t.Errorf("expected api_auth_ok=1, got %f", authMetric.GetGauge().GetValue())
	}

	reqMetric := findMetric(metrics, "\"requests\"")
	if reqMetric == nil {
		t.Fatal("requests metric not found")
	}
	if reqMetric.GetGauge().GetValue() != 5 {
		t.Errorf("expected requests=5, got %f", reqMetric.GetGauge().GetValue())
	}
}

func TestCollect_HealthEmittedEvenWhenStatsFail(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	statsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer statsSrv.Close()

	c := newTestCollector(healthSrv.URL, statsSrv.URL)
	ch := make(chan prometheus.Metric, 50)
	c.Collect(ch)

	metrics := drainMetrics(ch)

	if len(metrics) != 2 {
		t.Errorf("expected 2 health metrics when stats fail, got %d", len(metrics))
	}

	upMetric := findMetric(metrics, "api_up")
	if upMetric == nil {
		t.Fatal("api_up metric not found")
	}
	if upMetric.GetGauge().GetValue() != 1 {
		t.Errorf("expected api_up=1, got %f", upMetric.GetGauge().GetValue())
	}
}

func TestCollect_AuthFailure(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer healthSrv.Close()

	statsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer statsSrv.Close()

	c := newTestCollector(healthSrv.URL, statsSrv.URL)
	ch := make(chan prometheus.Metric, 50)
	c.Collect(ch)

	metrics := drainMetrics(ch)

	if len(metrics) != 2 {
		t.Errorf("expected 2 health metrics, got %d", len(metrics))
	}

	upMetric := findMetric(metrics, "api_up")
	if upMetric == nil {
		t.Fatal("api_up metric not found")
	}
	if upMetric.GetGauge().GetValue() != 1 {
		t.Errorf("expected api_up=1 (reachable but unauthorized), got %f", upMetric.GetGauge().GetValue())
	}

	authMetric := findMetric(metrics, "api_auth_ok")
	if authMetric == nil {
		t.Fatal("api_auth_ok metric not found")
	}
	if authMetric.GetGauge().GetValue() != 0 {
		t.Errorf("expected api_auth_ok=0, got %f", authMetric.GetGauge().GetValue())
	}
}

func TestCollect_WithTimezone(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	statsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"date":"2024-01-01","stats":[{"metrics":{"requests":7}}]}]`))
	}))
	defer statsSrv.Close()

	config := Config{
		UserName:   "test-user",
		Location:   "Asia/Tokyo",
		TimeOffset: 32400,
	}
	c := newTestCollectorWithConfig(healthSrv.URL, statsSrv.URL, config)
	ch := make(chan prometheus.Metric, 50)
	c.Collect(ch)

	metrics := drainMetrics(ch)

	// 2 health + 16 stats = 18
	if len(metrics) != 18 {
		t.Errorf("expected 18 metrics, got %d", len(metrics))
	}

	reqMetric := findMetric(metrics, "\"requests\"")
	if reqMetric == nil {
		t.Fatal("requests metric not found")
	}
	if reqMetric.GetGauge().GetValue() != 7 {
		t.Errorf("expected requests=7, got %f", reqMetric.GetGauge().GetValue())
	}
}

func TestCollect_WithAccumulatedMetrics(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	statsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("aggregated_by") != "month" {
			t.Errorf("expected aggregated_by=month, got %s", q.Get("aggregated_by"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"date":"2024-01-01","stats":[{"metrics":{"requests":100}}]}]`))
	}))
	defer statsSrv.Close()

	config := Config{
		UserName:           "test-user",
		AccumulatedMetrics: true,
	}
	c := newTestCollectorWithConfig(healthSrv.URL, statsSrv.URL, config)
	ch := make(chan prometheus.Metric, 50)
	c.Collect(ch)

	metrics := drainMetrics(ch)

	if len(metrics) != 18 {
		t.Errorf("expected 18 metrics, got %d", len(metrics))
	}

	reqMetric := findMetric(metrics, "\"requests\"")
	if reqMetric == nil {
		t.Fatal("requests metric not found")
	}
	if reqMetric.GetGauge().GetValue() != 100 {
		t.Errorf("expected requests=100, got %f", reqMetric.GetGauge().GetValue())
	}
}
