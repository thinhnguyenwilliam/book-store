# Monitoring and visitor analytics

## Start

From backend: `make observability-up`, then `make monitoring-check`.

- Grafana http://localhost:3001 (local admin/admin): **Book Store / Book Store Operations**.
- Prometheus http://localhost:9090: Targets and Alerts.
- Alertmanager http://localhost:9093: alerts and silences.
- Node Exporter http://localhost:9100/metrics.

Node Exporter monitors Linux host CPU, RAM and disks through read-only host
mounts and host PID namespace, without privileged mode. Network collectors are
disabled because it uses Docker bridge networking. On Docker Desktop it reports
the Linux VM, not the physical host.

Alerts: scrape target down for 2m, CPU >85% for 10m, available RAM <10% for 5m,
available disk <15% for 10m, failed notification delivery. These are infrastructure
alerts, not probes of every Go service or checkout business outcomes.
A stopped Prometheus cannot report itself; production needs external uptime
monitoring too. Tune thresholds for real capacity.

## Slack / Telegram

Default alertmanager.yml is local-only: alerts appear in the UI, no outbound messages.

1. Slack: create an app, enable **Incoming Webhooks**, authorize your channel,
   and obtain its webhook URL.
2. Telegram: create a bot via **@BotFather**, /start it or add it to your group.
   Obtain numeric chat_id from a bot update (getUpdates); group IDs are often negative.
3. Create backend/secrets/alertmanager/slack-webhook and telegram-bot-token, each
   containing ONLY the secret value. The directory is gitignored.
   Docker may create this directory as root; if so give your local user ownership
   of this specific directory before editing.
4. Copy observability/alertmanager.local.yml.example to
   observability/alertmanager.local.yml using your editor; fill channel/chat_id.
   Remove the unused receiver config if you only use one provider.
5. Make secret files readable by container UID/GID 65534. On Linux use group
   65534 and mode 0640, with a traversable directory. Do not make them world-readable.
6. Run `make alertmanager-local-up`. Subsequent Makefile starts retain the local
   config automatically. To return to local-only, move the local file elsewhere
   or pass ALERTMANAGER_CONFIG=./observability/alertmanager.yml.

Grouping: wait 30s, update every 5m, repeat every 4h; resolved notifications enabled.
Check Alertmanager logs and alertmanager_notifications_failed_total for failures.
Config validation does not prove delivery. No real token/webhook is shipped.
Never put these credentials in VITE_* variables or commit them.

To test real delivery after configuring the destination, POST a short-lived
synthetic alert to /api/v2/alerts. This notifies channel members; the agent has not
sent a test message. Use the local UI first to inspect routing.

Official config: https://prometheus.io/docs/alerting/latest/configuration/

## Access IPs in Grafana

Operations dashboard shows Gateway access logs including remote_ip, path,
status, duration and trace ID. These represent API requests, not unique people
or all browser page views; NAT/proxies/shared IPs cannot identify individuals.
Restrict operator access. Loki retention is 14 days; file retention is configured
separately by the backend daily logger.

Gateway uses direct peer IP and omits URL query strings. Behind Nginx/CDN,
configure Echo's trusted-proxy extractor with actual proxy ranges, otherwise the
IP shown will be the proxy. Do not trust arbitrary public forwarding headers.
Do not send these IP logs to Google Analytics.

## GA4: storefront only

1. At https://analytics.google.com create/select a GA4 property → Admin →
   Data streams → Web → storefront URL → copy Measurement ID (G-...).
   This is different from Google OAuth Client ID and Firebase API key.
2. In storefront/.env set VITE_GA_MEASUREMENT_ID=G-YOURID.
   VITE_GA_ENABLE_LOCAL=false by default. For local testing set true (prefer a
   separate test property). Restart Vite after .env changes; rebuild production.
3. Turn OFF **Enhanced measurement** entirely in the GA4 Web stream. This app
   manually sends SPA page_view. Automatic history/form/search tracking can
   duplicate events and collect query data the app deliberately removes.
4. Choose **Đồng ý** in the storefront analytics prompt. The tag is never loaded
   before consent or after rejection (basic consent mode). **Tùy chọn thống kê**
   lets users revoke consent.
5. Inspect Realtime, then Reports → Engagement → Pages and screens, Acquisition,
   and Tech. Regular reports take time; refusal/ad blockers reduce recorded data.

Events: page_view, view_item, add_to_cart, remove_from_cart, begin_checkout.
No purchase event yet: backend order/payment records are the financial source
of truth. Dynamic paths become templates; query/fragment, email, name, chat,
free-text search and order IDs are excluded. Referrers include origin only;
UTM/campaign query parameters are not collected by this setup.

GA4 provides devices, approximate location and engagement, not a list of IPs:
https://support.google.com/analytics/answer/11598602

Admin portal is not tracked. Existing internal Kafka customer-activity collection
is unchanged; this consent prompt controls Google Analytics only.
Reports are viewed in GA4. Embedding them in admin would require a separate
server-side Data API integration. Never expose service-account secrets in VITE_*.
