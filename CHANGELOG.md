# Changelog

## v0.2.1

- Automate Helm chart version and image tag from git tag in release workflow

## v0.2.0

- Add `sendgrid_account_type` and `sendgrid_reputation` metrics from `/v3/user/account`
- Account metrics gated behind `SENDGRID_COLLECT_ACCOUNT_INFO` flag (disabled by default, requires Billing Read permission)
- Add rate limit handling (429) to health check endpoint
- Update documentation and environment variable references

## v0.1.4

- Add RPM packages (x86_64 and aarch64) to release workflow with systemd service and sysconfig
- Add `/etc/sysconfig/sendgrid-stats-exporter` for RPM-based distros
- Systemd service now checks both `/etc/default/` and `/etc/sysconfig/` for environment file

## v0.1.3

- Bump Helm chart to v0.1.3
- Include Helm chart package in GitHub release artifacts

## v0.1.2

- Add Helm chart packaging to release workflow
- Update Helm chart to use `ashleykleynhans/sendgrid-stats-exporter` image
- Bump Helm chart version to 0.1.2

## v0.1.1

- Fix error messages to read response body instead of printing Go struct pointer

## v0.1.0

- Add `sendgrid_api_up` and `sendgrid_api_auth_ok` health monitoring metrics via `/v3/scopes`
- Restructure project into standard Go layout (`cmd/`, `internal/`)
- Add full test suite with 98.9% coverage
- Migrate dependencies to latest versions (Go 1.25, kingpin v2, promslog, prometheus client v1.23)
- Replace `go-kit/log` with stdlib `log/slog`
- Add GitHub Actions workflows for CI testing and automated releases
- Add Debian packages (amd64 and arm64) with systemd service unit
- Rename binary to `sendgrid_exporter`
- Update Alpine to 3.21, Prometheus to v3.4.1 in Docker Compose
- Rewrite README

## Pre-fork (chatwork/sendgrid-stats-exporter)

- Initial Prometheus exporter for SendGrid Stats API (v3)
- 16 email delivery metrics (requests, delivered, bounces, blocks, etc.)
- Docker and Helm chart support
- Timezone and accumulated metrics options
- CircleCI build pipeline
