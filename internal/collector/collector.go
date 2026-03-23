package collector

import (
	"log/slog"
	"time"

	"github.com/chatwork/sendgrid-stats-exporter/internal/sendgrid"
	"github.com/jinzhu/now"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "sendgrid"

// Config holds the configuration for the Collector.
type Config struct {
	UserName           string
	Location           string
	TimeOffset         int
	AccumulatedMetrics bool
}

// Collector implements the prometheus.Collector interface for SendGrid metrics.
type Collector struct {
	logger *slog.Logger
	client *sendgrid.Client
	config Config

	blocks           *prometheus.Desc
	bounceDrops      *prometheus.Desc
	bounces          *prometheus.Desc
	clicks           *prometheus.Desc
	deferred         *prometheus.Desc
	delivered        *prometheus.Desc
	invalidEmails    *prometheus.Desc
	opens            *prometheus.Desc
	processed        *prometheus.Desc
	requests         *prometheus.Desc
	spamReportDrops  *prometheus.Desc
	spamReports      *prometheus.Desc
	uniqueClicks     *prometheus.Desc
	uniqueOpens      *prometheus.Desc
	unsubscribeDrops *prometheus.Desc
	unsubscribes     *prometheus.Desc

	apiUp     *prometheus.Desc
	apiAuthOk *prometheus.Desc
}

// New creates a new Collector.
func New(logger *slog.Logger, client *sendgrid.Client, config Config) *Collector {
	labels := []string{"user_name"}

	return &Collector{
		logger: logger,
		client: client,
		config: config,

		blocks:           prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "blocks"), "blocks", labels, nil),
		bounceDrops:      prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "bounce_drops"), "bounce_drops", labels, nil),
		bounces:          prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "bounces"), "bounces", labels, nil),
		clicks:           prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "clicks"), "clicks", labels, nil),
		deferred:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "deferred"), "deferred", labels, nil),
		delivered:        prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "delivered"), "delivered", labels, nil),
		invalidEmails:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "invalid_emails"), "invalid_emails", labels, nil),
		opens:            prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "opens"), "opens", labels, nil),
		processed:        prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "processed"), "processed", labels, nil),
		requests:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "requests"), "requests", labels, nil),
		spamReportDrops:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "spam_report_drops"), "spam_report_drops", labels, nil),
		spamReports:      prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "spam_reports"), "spam_reports", labels, nil),
		uniqueClicks:     prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "unique_clicks"), "unique_clicks", labels, nil),
		uniqueOpens:      prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "unique_opens"), "unique_opens", labels, nil),
		unsubscribeDrops: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "unsubscribe_drops"), "unsubscribe_drops", labels, nil),
		unsubscribes:     prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "unsubscribes"), "unsubscribes", labels, nil),

		apiUp:     prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "api_up"), "1 if the SendGrid API is reachable, 0 otherwise", labels, nil),
		apiAuthOk: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "api_auth_ok"), "1 if the SendGrid API key is valid, 0 if unauthorized", labels, nil),
	}
}

// Describe sends all metric descriptors to the channel.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.blocks
	ch <- c.bounceDrops
	ch <- c.bounces
	ch <- c.clicks
	ch <- c.deferred
	ch <- c.delivered
	ch <- c.invalidEmails
	ch <- c.opens
	ch <- c.processed
	ch <- c.requests
	ch <- c.spamReportDrops
	ch <- c.spamReports
	ch <- c.uniqueClicks
	ch <- c.uniqueOpens
	ch <- c.unsubscribeDrops
	ch <- c.unsubscribes
	ch <- c.apiUp
	ch <- c.apiAuthOk
}

// Collect fetches metrics from the SendGrid API and sends them to the channel.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	health := c.client.CheckHealth()
	ch <- prometheus.MustNewConstMetric(c.apiUp, prometheus.GaugeValue, health.Up, c.config.UserName)
	ch <- prometheus.MustNewConstMetric(c.apiAuthOk, prometheus.GaugeValue, health.AuthOk, c.config.UserName)

	var today time.Time

	if c.config.Location != "" && c.config.TimeOffset != 0 {
		loc := time.FixedZone(c.config.Location, c.config.TimeOffset)
		today = time.Now().In(loc)
	} else {
		today = time.Now()
	}

	queryDate := today
	if c.config.AccumulatedMetrics {
		queryDate = now.With(today).BeginningOfMonth()
	}

	statistics, err := c.client.CollectByDate(queryDate, today, c.config.AccumulatedMetrics)
	if err != nil {
		c.logger.Error("Failed to collect stats", "err", err)
		return
	}

	for _, stats := range statistics[0].Stats {
		ch <- prometheus.MustNewConstMetric(c.blocks, prometheus.GaugeValue, float64(stats.Metrics.Blocks), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.bounceDrops, prometheus.GaugeValue, float64(stats.Metrics.BounceDrops), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.bounces, prometheus.GaugeValue, float64(stats.Metrics.Bounces), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.clicks, prometheus.GaugeValue, float64(stats.Metrics.Clicks), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.deferred, prometheus.GaugeValue, float64(stats.Metrics.Deferred), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.delivered, prometheus.GaugeValue, float64(stats.Metrics.Delivered), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.invalidEmails, prometheus.GaugeValue, float64(stats.Metrics.InvalidEmails), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.opens, prometheus.GaugeValue, float64(stats.Metrics.Opens), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.processed, prometheus.GaugeValue, float64(stats.Metrics.Processed), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.requests, prometheus.GaugeValue, float64(stats.Metrics.Requests), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.spamReportDrops, prometheus.GaugeValue, float64(stats.Metrics.SpamReportDrops), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.spamReports, prometheus.GaugeValue, float64(stats.Metrics.SpamReports), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.uniqueClicks, prometheus.GaugeValue, float64(stats.Metrics.UniqueClicks), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.uniqueOpens, prometheus.GaugeValue, float64(stats.Metrics.UniqueOpens), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.unsubscribeDrops, prometheus.GaugeValue, float64(stats.Metrics.UnsubscribeDrops), c.config.UserName)
		ch <- prometheus.MustNewConstMetric(c.unsubscribes, prometheus.GaugeValue, float64(stats.Metrics.Unsubscribes), c.config.UserName)
	}
}
