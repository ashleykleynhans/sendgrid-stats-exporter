package sendgrid

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

var (
	StatsEndpoint  = "https://api.sendgrid.com/v3/stats"
	HealthEndpoint = "https://api.sendgrid.com/v3/scopes"
)

// Metrics represents the email delivery metrics returned by the SendGrid Stats API.
type Metrics struct {
	Blocks           int64 `json:"blocks,omitempty"`
	BounceDrops      int64 `json:"bounce_drops,omitempty"`
	Bounces          int64 `json:"bounces,omitempty"`
	Clicks           int64 `json:"clicks,omitempty"`
	Deferred         int64 `json:"deferred,omitempty"`
	Delivered        int64 `json:"delivered,omitempty"`
	InvalidEmails    int64 `json:"invalid_emails,omitempty"`
	Opens            int64 `json:"opens,omitempty"`
	Processed        int64 `json:"processed,omitempty"`
	Requests         int64 `json:"requests,omitempty"`
	SpamReportDrops  int64 `json:"spam_report_drops,omitempty"`
	SpamReports      int64 `json:"spam_reports,omitempty"`
	UniqueClicks     int64 `json:"unique_clicks,omitempty"`
	UniqueOpens      int64 `json:"unique_opens,omitempty"`
	UnsubscribeDrops int64 `json:"unsubscribe_drops,omitempty"`
	Unsubscribes     int64 `json:"unsubscribes,omitempty"`
}

// Stat wraps a Metrics entry from the API response.
type Stat struct {
	Metrics *Metrics `json:"metrics,omitempty"`
}

// Statistics represents a single date's stats from the API response.
type Statistics struct {
	Date  string  `json:"date,omitempty"`
	Stats []*Stat `json:"stats,omitempty"`
}

// HealthStatus captures the result of a SendGrid API health probe.
type HealthStatus struct {
	Up     float64 // 1 = reachable, 0 = network/server error
	AuthOk float64 // 1 = key valid, 0 = 401/403
}

// Client is a SendGrid API client for stats and health checking.
type Client struct {
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new SendGrid API client.
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:     apiKey,
		HTTPClient: http.DefaultClient,
	}
}

// CheckHealth probes the SendGrid API to determine reachability and key validity.
func (c *Client) CheckHealth() HealthStatus {
	req, err := http.NewRequest(http.MethodGet, HealthEndpoint, nil)
	if err != nil {
		return HealthStatus{Up: 0, AuthOk: 0}
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return HealthStatus{Up: 0, AuthOk: 0}
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		return HealthStatus{Up: 1, AuthOk: 1}
	case http.StatusUnauthorized, http.StatusForbidden:
		return HealthStatus{Up: 1, AuthOk: 0}
	default:
		return HealthStatus{Up: 0, AuthOk: 0}
	}
}

// CollectByDate fetches email stats from the SendGrid API for the given date range.
func (c *Client) CollectByDate(timeStart time.Time, timeEnd time.Time, accumulated bool) ([]*Statistics, error) {
	parsedURL, err := url.Parse(StatsEndpoint)
	if err != nil {
		return nil, err
	}

	layout := "2006-01-02"
	dateStart := timeStart.Format(layout)
	dateEnd := timeEnd.Format(layout)

	query := url.Values{}
	query.Set("start_date", dateStart)
	query.Set("end_date", dateEnd)
	if accumulated {
		query.Set("aggregated_by", "month")
	} else {
		query.Set("aggregated_by", "day")
	}
	parsedURL.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var reader io.Reader = res.Body
	reader = io.TeeReader(reader, os.Stdout)

	switch res.StatusCode {
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("API rate limit exceeded")
	case http.StatusOK:
		var stats []*Statistics
		if err := json.NewDecoder(reader).Decode(&stats); err != nil {
			return nil, err
		}

		return stats, nil
	default:
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("status code = %d, response = %s", res.StatusCode, string(body))
	}
}
