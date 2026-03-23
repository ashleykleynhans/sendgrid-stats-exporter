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

func okAccountSrv() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"type":"paid","reputation":99.5}`))
	}))
}

func newTestCollector(healthURL, statsURL string) *Collector {
	return newTestCollectorWithConfig(healthURL, statsURL, Config{UserName: "test-user", CollectAccountInfo: true})
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

	// 16 stats + 2 health + 2 account = 20
	expected := 20
	if len(descs) != expected {
		t.Errorf("expected %d descriptors, got %d", expected, len(descs))
	}

	names := []string{"api_up", "api_auth_ok", "account_type", "reputation"}
	for _, name := range names {
		found := false
		for _, d := range descs {
			if strings.Contains(d.String(), name) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s descriptor", name)
		}
	}
}

func TestCollect_HealthyWithStats(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	accountSrv := okAccountSrv()
	defer accountSrv.Close()
	sendgrid.AccountEndpoint = accountSrv.URL

	statsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"date":"2024-01-01","stats":[{"metrics":{"requests":5,"delivered":3,"bounces":1}}]}]`))
	}))
	defer statsSrv.Close()

	c := newTestCollector(healthSrv.URL, statsSrv.URL)
	ch := make(chan prometheus.Metric, 50)
	c.Collect(ch)

	metrics := drainMetrics(ch)

	// 2 health + 2 account + 16 stats = 20
	if len(metrics) != 20 {
		t.Errorf("expected 20 metrics, got %d", len(metrics))
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

	repMetric := findMetric(metrics, "reputation")
	if repMetric == nil {
		t.Fatal("reputation metric not found")
	}
	if repMetric.GetGauge().GetValue() != 99.5 {
		t.Errorf("expected reputation=99.5, got %f", repMetric.GetGauge().GetValue())
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

	accountSrv := okAccountSrv()
	defer accountSrv.Close()
	sendgrid.AccountEndpoint = accountSrv.URL

	statsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer statsSrv.Close()

	c := newTestCollector(healthSrv.URL, statsSrv.URL)
	ch := make(chan prometheus.Metric, 50)
	c.Collect(ch)

	metrics := drainMetrics(ch)

	// 2 health + 2 account, no stats
	if len(metrics) != 4 {
		t.Errorf("expected 4 metrics when stats fail, got %d", len(metrics))
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

	// Account will also fail with auth error
	accountSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer accountSrv.Close()
	sendgrid.AccountEndpoint = accountSrv.URL

	statsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer statsSrv.Close()

	c := newTestCollector(healthSrv.URL, statsSrv.URL)
	ch := make(chan prometheus.Metric, 50)
	c.Collect(ch)

	metrics := drainMetrics(ch)

	// 2 health only, account and stats both fail
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

	accountSrv := okAccountSrv()
	defer accountSrv.Close()
	sendgrid.AccountEndpoint = accountSrv.URL

	statsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"date":"2024-01-01","stats":[{"metrics":{"requests":7}}]}]`))
	}))
	defer statsSrv.Close()

	config := Config{
		UserName:           "test-user",
		Location:           "Asia/Tokyo",
		TimeOffset:         32400,
		CollectAccountInfo: true,
	}
	c := newTestCollectorWithConfig(healthSrv.URL, statsSrv.URL, config)
	ch := make(chan prometheus.Metric, 50)
	c.Collect(ch)

	metrics := drainMetrics(ch)

	// 2 health + 2 account + 16 stats = 20
	if len(metrics) != 20 {
		t.Errorf("expected 20 metrics, got %d", len(metrics))
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

	accountSrv := okAccountSrv()
	defer accountSrv.Close()
	sendgrid.AccountEndpoint = accountSrv.URL

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
		CollectAccountInfo: true,
	}
	c := newTestCollectorWithConfig(healthSrv.URL, statsSrv.URL, config)
	ch := make(chan prometheus.Metric, 50)
	c.Collect(ch)

	metrics := drainMetrics(ch)

	if len(metrics) != 20 {
		t.Errorf("expected 20 metrics, got %d", len(metrics))
	}

	reqMetric := findMetric(metrics, "\"requests\"")
	if reqMetric == nil {
		t.Fatal("requests metric not found")
	}
	if reqMetric.GetGauge().GetValue() != 100 {
		t.Errorf("expected requests=100, got %f", reqMetric.GetGauge().GetValue())
	}
}

func TestCollect_AccountInfoFailsGracefully(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	// Account endpoint fails
	sendgrid.AccountEndpoint = "http://127.0.0.1:1"

	statsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"date":"2024-01-01","stats":[{"metrics":{"requests":5}}]}]`))
	}))
	defer statsSrv.Close()

	c := newTestCollector(healthSrv.URL, statsSrv.URL)
	ch := make(chan prometheus.Metric, 50)
	c.Collect(ch)

	metrics := drainMetrics(ch)

	// 2 health + 0 account + 16 stats = 18
	if len(metrics) != 18 {
		t.Errorf("expected 18 metrics when account fails, got %d", len(metrics))
	}
}

func TestCollect_AccountInfoDisabled(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	statsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"date":"2024-01-01","stats":[{"metrics":{"requests":5}}]}]`))
	}))
	defer statsSrv.Close()

	config := Config{
		UserName:           "test-user",
		CollectAccountInfo: false,
	}
	c := newTestCollectorWithConfig(healthSrv.URL, statsSrv.URL, config)
	ch := make(chan prometheus.Metric, 50)
	c.Collect(ch)

	metrics := drainMetrics(ch)

	// 2 health + 16 stats = 18, no account metrics
	if len(metrics) != 18 {
		t.Errorf("expected 18 metrics when account disabled, got %d", len(metrics))
	}

	repMetric := findMetric(metrics, "reputation")
	if repMetric != nil {
		t.Error("expected no reputation metric when account info disabled")
	}
}
