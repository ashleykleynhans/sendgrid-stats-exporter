# sendgrid-stats-exporter

Prometheus exporter for SendGrid metrics via the Stats API (v3).

```
SendGrid API ──── /v3/stats ────▶ exporter ◀──── /metrics ──── Prometheus
                  /v3/scopes ──▶
```

## Usage

```sh
make
./dist/sendgrid_exporter --sendgrid.api-key='YOUR_KEY'
```

```sh
curl localhost:9154/-/healthy
curl localhost:9154/metrics
```

### Flags

```
./dist/sendgrid_exporter -h

Flags:
  -h, --help                     Show help.
      --web.listen-address=":9154"
                                 Address to listen on for web interface and telemetry.
      --web.disable-exporter-metrics
                                 Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).
      --sendgrid.api-key="secret"
                                 [Required] Set SendGrid API key.
      --sendgrid.username=""     [Optional] Label for identifying multiple SendGrid users.
      --sendgrid.location=""     [Optional] Timezone name (e.g. 'Asia/Tokyo'). Default is UTC.
      --sendgrid.time-offset=0   [Optional] UTC offset in seconds (e.g. 32400). Must be set with --location.
      --sendgrid.accumulated-metrics
                                 [Optional] Aggregate metrics by month for monthly email limit tracking.
      --log.level=info           Log severity. One of: [debug, info, warn, error]
      --log.format=logfmt        Log format. One of: [logfmt, json]
      --version                  Show application version.
```

### Environment variables

All flags can be set via environment variables:

| Variable | Flag |
|---|---|
| `SENDGRID_API_KEY` | `--sendgrid.api-key` |
| `SENDGRID_USER_NAME` | `--sendgrid.username` |
| `SENDGRID_LOCATION` | `--sendgrid.location` |
| `SENDGRID_TIME_OFFSET` | `--sendgrid.time-offset` |
| `SENDGRID_ACCUMULATED_METRICS` | `--sendgrid.accumulated-metrics` |
| `LISTEN_ADDRESS` | `--web.listen-address` |
| `DISABLE_EXPORTER_METRICS` | `--web.disable-exporter-metrics` |

## Endpoints

| Path | Description |
|---|---|
| `/metrics` | Prometheus metrics |
| `/-/healthy` | Health check (returns `OK`) |

## Metrics

### Email delivery metrics

All metrics are prefixed with `sendgrid_` and labelled with `user_name`.

| Metric | Description |
|---|---|
| `requests` | Emails requested to be delivered |
| `processed` | Emails processed by SendGrid |
| `delivered` | Emails confirmed delivered to a recipient |
| `deferred` | Emails that temporarily could not be delivered |
| `bounces` | Emails that bounced instead of being delivered |
| `bounce_drops` | Emails dropped because of a prior bounce |
| `blocks` | Emails not allowed to be delivered by ISPs |
| `invalid_emails` | Recipients with malformed or invalid email addresses |
| `spam_reports` | Recipients who marked your email as spam |
| `spam_report_drops` | Emails dropped due to a prior spam report |
| `opens` | Total email opens |
| `unique_opens` | Unique recipients who opened your emails |
| `clicks` | Total link clicks |
| `unique_clicks` | Unique recipients who clicked links |
| `unsubscribes` | Recipients who unsubscribed |
| `unsubscribe_drops` | Emails dropped due to a prior unsubscribe |

### API health metrics

| Metric | Description |
|---|---|
| `api_up` | `1` if the SendGrid API is reachable, `0` otherwise. Probes `/v3/scopes` on each scrape. |
| `api_auth_ok` | `1` if the API key is valid, `0` on 401/403. Only meaningful when `api_up` is `1`. |

## Dashboard

A sample Grafana dashboard is published [here](https://grafana.com/grafana/dashboards/16319).

## Running

### Docker

```sh
docker run -d -p 9154:9154 -e SENDGRID_API_KEY='YOUR_KEY' chatwork/sendgrid-stats-exporter
```

### Docker Compose

```sh
cp .env.example .env
vi .env
docker-compose up -d
```

Metrics are available at [http://127.0.0.1:9154/metrics]().

### Helm

See [charts/](charts/).

## Building

### Locally

```sh
make
```

### Docker

```sh
docker build -t sendgrid-stats-exporter .
```

### Running tests

```sh
make test
```
