# FiberPulse Observatory

FiberPulse Observatory is the public, privacy-preserving receiver for measurements voluntarily shared by FiberPulse applications.

## Public fields

The public feed contains a 15-minute timestamp bucket, coarse country/region/city, measured download and upload, latency, connection type, confidence, ISP, offer name, subscription category, and advertised plan speeds.

It does not accept or publish subscriber names, email addresses, telephone numbers, account numbers, service addresses, exact IP addresses, GPS coordinates, postcodes, SSIDs, hostnames, device identifiers, hardware profiles, complaint profiles, or local logs. Free-form custom ISP and offer names are replaced in the app before upload.

## Run locally

```sh
FIBERPULSE_OBSERVATORY_LOCATION_MODE=plan-country \
FIBERPULSE_OBSERVATORY_DATA_DIR=/absolute/path/to/data \
FIBERPULSE_OBSERVATORY_ADDRESS=127.0.0.1:8090 \
go run ./cmd/fiberpulse-observatory
```

Then build or run an app with `FIBERPULSE_SHARE_URL=http://127.0.0.1:8090`. Anonymous sharing remains off until the user enables the separate FiberPulse sharing permission.

## Online deployment

Run one Observatory process behind an HTTPS reverse proxy. For the intended Cloudflare setup:

1. Keep the origin private or firewall it to Cloudflare only.
2. Enable Cloudflare's **Add visitor location headers** Managed Transform.
3. Set `FIBERPULSE_OBSERVATORY_LOCATION_MODE=cloudflare`.
4. Disable origin access logs and enable Cloudflare's **Remove visitor IP headers** transform when the chosen edge rate-limiting policy permits it.
5. Set `FIBERPULSE_SHARE_URL=https://your-observatory-domain.example` while packaging the apps. The URL is embedded with linker flags; no end-user configuration is required.
6. Back up `observatory.db` and its WAL files consistently, monitor `/healthz`, and terminate with SIGTERM for graceful shutdown.

The hardened single-host container is defined in `packaging/observatory/compose.yaml`. It binds the origin only to loopback, runs without Linux capabilities as an unprivileged user, and keeps only `/data` writable.

The `plan-country` mode is for local development and fallback deployments. It cannot provide a trustworthy city. Public production data should use the Cloudflare location mode or a future local GeoIP resolver.

## API

- `POST /api/v1/installations`: idempotent Ed25519 public-key registration.
- `POST /api/v1/measurements`: signed, replay-protected, idempotent measurement ingestion.
- `GET /api/v1/public/measurements`: paginated search (`q`, `country`, `provider`, `page`, `limit`).
- `GET /api/v1/public/summary`: public totals and averages.
- `GET /api/v1/public/facets`: complete country and provider search filters.
- `GET /healthz`: health probe.

The receiver never writes an installation identifier into a measurement row and never exposes one through the public API. Public measurements therefore cannot be grouped by device. Signing keys are held separately for verification, replay prevention and abuse control, then removed after 400 days without activity.
