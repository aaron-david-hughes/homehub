# HomeHub

A local-first home platform running on a single-node k3s cluster on a Raspberry Pi.
Services register themselves with the portal, which shows them as a dashboard.

See `docs/` and the planning overview for the full roadmap.

## Layout

```
services/<name>/   one folder per service (Go + HTMX)
  cmd/<name>/        main package
  internal/          service code
  web/               embedded static assets
shared/            Go module shared by every service (registration contract, health handler)
infra/             cluster bootstrap and GitOps manifests (later)
docs/              hardware notes, architecture
go.work            ties shared/ and every service into one workspace
```

## Quick start

Requires Go 1.24+.

```bash
make vendor      # once: download HTMX into the portal's assets, then commit it
make run         # run the portal at http://localhost:8080 (with placeholder cards)
make test        # run all tests
```

Without `make vendor` the dashboard still works: a small fallback script refreshes it.

## Registering a service

A service announces itself to the portal. Using the shared module:

```go
reg := registry.ServiceRegistration{
    Name:        "thermostat",
    DisplayName: "Thermostat",
    Icon:        "thermometer",
    URL:         "http://thermostat:8080", // in-cluster address, used for health checks
    UIPath:      "/thermostat",            // browser path the dashboard card links to
    Version:     "1.0.0",
}
go registry.KeepRegistered(ctx, "http://portal:8080", reg, time.Minute)
```

Or directly over HTTP:

```
POST /api/services/register
{"name":"thermostat","displayName":"Thermostat","icon":"thermometer",
 "url":"http://thermostat:8080","uiPath":"/thermostat","version":"1.0.0"}
```

- The portal polls `<url><healthEndpoint>` (default `/health`) every 30 seconds.
- A service missing 3 checks in a row shows as "Not responding" and recovers on its own.
- `KeepRegistered` re-registers every minute, so a portal restart never loses a service.
- Icons available: `home thermometer receipt zap camera server grid` (unknown names fall back to `grid`).

## Adding a new service

1. Create `services/<name>/` with its own `go.mod` (module `github.com/aaron-david-hughes/homehub/services/<name>`).
2. Add `./services/<name>` to `go.work`.
3. Use `shared/health` for `/health` and `shared/registry` to register.
4. `make run SERVICE=<name>`.

## Portal configuration

| Variable        | Default | Purpose                                         |
|-----------------|---------|-------------------------------------------------|
| `PORT`          | `8080`  | Listen port                                     |
| `POLL_INTERVAL` | `30s`   | How often service health is checked             |
| `SEED_DEMO`     | `false` | Show "coming soon" cards for planned services   |
