package sendgrid

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient() *Client {
	return NewClient("test-api-key")
}

func TestCheckHealth_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Bearer test-api-key, got %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"scopes": ["mail.send"]}`))
	}))
	defer srv.Close()

	orig := HealthEndpoint
	HealthEndpoint = srv.URL
	defer func() { HealthEndpoint = orig }()

	status := newTestClient().CheckHealth()
	if status.Up != 1 {
		t.Errorf("expected Up=1, got %f", status.Up)
	}
	if status.AuthOk != 1 {
		t.Errorf("expected AuthOk=1, got %f", status.AuthOk)
	}
}

func TestCheckHealth_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	orig := HealthEndpoint
	HealthEndpoint = srv.URL
	defer func() { HealthEndpoint = orig }()

	status := newTestClient().CheckHealth()
	if status.Up != 1 {
		t.Errorf("expected Up=1, got %f", status.Up)
	}
	if status.AuthOk != 0 {
		t.Errorf("expected AuthOk=0, got %f", status.AuthOk)
	}
}

func TestCheckHealth_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	orig := HealthEndpoint
	HealthEndpoint = srv.URL
	defer func() { HealthEndpoint = orig }()

	status := newTestClient().CheckHealth()
	if status.Up != 1 {
		t.Errorf("expected Up=1, got %f", status.Up)
	}
	if status.AuthOk != 0 {
		t.Errorf("expected AuthOk=0, got %f", status.AuthOk)
	}
}

func TestCheckHealth_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := HealthEndpoint
	HealthEndpoint = srv.URL
	defer func() { HealthEndpoint = orig }()

	status := newTestClient().CheckHealth()
	if status.Up != 0 {
		t.Errorf("expected Up=0, got %f", status.Up)
	}
	if status.AuthOk != 0 {
		t.Errorf("expected AuthOk=0, got %f", status.AuthOk)
	}
}

func TestCheckHealth_NetworkError(t *testing.T) {
	orig := HealthEndpoint
	HealthEndpoint = "http://127.0.0.1:1"
	defer func() { HealthEndpoint = orig }()

	status := newTestClient().CheckHealth()
	if status.Up != 0 {
		t.Errorf("expected Up=0, got %f", status.Up)
	}
	if status.AuthOk != 0 {
		t.Errorf("expected AuthOk=0, got %f", status.AuthOk)
	}
}

func TestCollectByDate_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Bearer test-api-key, got %s", r.Header.Get("Authorization"))
		}
		q := r.URL.Query()
		if q.Get("start_date") == "" || q.Get("end_date") == "" {
			t.Error("expected start_date and end_date query params")
		}
		if q.Get("aggregated_by") != "day" {
			t.Errorf("expected aggregated_by=day, got %s", q.Get("aggregated_by"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"date":"2024-01-01","stats":[{"metrics":{"requests":10,"delivered":8,"bounces":2}}]}]`))
	}))
	defer srv.Close()

	orig := StatsEndpoint
	StatsEndpoint = srv.URL
	defer func() { StatsEndpoint = orig }()

	now := time.Now()
	stats, err := newTestClient().CollectByDate(now, now, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 statistics entry, got %d", len(stats))
	}
	if stats[0].Stats[0].Metrics.Requests != 10 {
		t.Errorf("expected requests=10, got %d", stats[0].Stats[0].Metrics.Requests)
	}
	if stats[0].Stats[0].Metrics.Delivered != 8 {
		t.Errorf("expected delivered=8, got %d", stats[0].Stats[0].Metrics.Delivered)
	}
	if stats[0].Stats[0].Metrics.Bounces != 2 {
		t.Errorf("expected bounces=2, got %d", stats[0].Stats[0].Metrics.Bounces)
	}
}

func TestCollectByDate_Accumulated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("aggregated_by") != "month" {
			t.Errorf("expected aggregated_by=month, got %s", r.URL.Query().Get("aggregated_by"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"date":"2024-01-01","stats":[{"metrics":{"requests":100}}]}]`))
	}))
	defer srv.Close()

	orig := StatsEndpoint
	StatsEndpoint = srv.URL
	defer func() { StatsEndpoint = orig }()

	now := time.Now()
	_, err := newTestClient().CollectByDate(now, now, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCollectByDate_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	orig := StatsEndpoint
	StatsEndpoint = srv.URL
	defer func() { StatsEndpoint = orig }()

	now := time.Now()
	_, err := newTestClient().CollectByDate(now, now, false)
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
	if err.Error() != "API rate limit exceeded" {
		t.Errorf("expected 'API rate limit exceeded', got '%s'", err.Error())
	}
}

func TestCollectByDate_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	orig := StatsEndpoint
	StatsEndpoint = srv.URL
	defer func() { StatsEndpoint = orig }()

	now := time.Now()
	_, err := newTestClient().CollectByDate(now, now, false)
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestCollectByDate_NetworkError(t *testing.T) {
	orig := StatsEndpoint
	StatsEndpoint = "http://127.0.0.1:1"
	defer func() { StatsEndpoint = orig }()

	now := time.Now()
	_, err := newTestClient().CollectByDate(now, now, false)
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestCheckHealth_InvalidURL(t *testing.T) {
	orig := HealthEndpoint
	HealthEndpoint = "://invalid"
	defer func() { HealthEndpoint = orig }()

	status := newTestClient().CheckHealth()
	if status.Up != 0 {
		t.Errorf("expected Up=0, got %f", status.Up)
	}
	if status.AuthOk != 0 {
		t.Errorf("expected AuthOk=0, got %f", status.AuthOk)
	}
}

func TestCollectByDate_InvalidURL(t *testing.T) {
	orig := StatsEndpoint
	StatsEndpoint = "://invalid"
	defer func() { StatsEndpoint = orig }()

	now := time.Now()
	_, err := newTestClient().CollectByDate(now, now, false)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestCollectByDate_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	orig := StatsEndpoint
	StatsEndpoint = srv.URL
	defer func() { StatsEndpoint = orig }()

	now := time.Now()
	_, err := newTestClient().CollectByDate(now, now, false)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
