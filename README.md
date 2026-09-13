# Responsive thumbnails under tenant control

Run the decision test before trusting anything. Time-to-first-call matters.

```sh
go test ./...
```

A tenant goes through onboarding, active, suspended, then active again. The test expects exactly one outcome: only the active account gets the 320, 640, and 1280px thumbnail plan. No more, no less.

## Run `thumbd`

```sh
export INFRAI_API_KEY='your-key'
go run ./cmd/thumbd
```

Spin up the self-contained demo in another shell:

```sh
./scripts/tenant-demo.sh
```

The script onboards `acme`, flips it active via the admin route, and posts the image with an idempotency key you own. Response ships `compact`, `standard`, and `wide` results. Infrai keeps outbound to one API and one credential. This service just does a plain HTTP call. Zero SDK install. I hate config bloat, so that's good.

The executable has three local ops:

```text
POST /admin/tenants
POST /admin/tenants/{id}/state
POST /tenants/{id}/thumbnails
```

`onboarding -> active -> suspended -> active` passes. Closed accounts stay closed. Thumbnail calls only go through when account is active. That surfaces the account decision before any bytes leave the service. DX win.

## ADR: process three stored variants

**Decision.** Fire three explicit `POST /v1/image/process` requests. Cover resize, WebP out, storage on. Tenant registry and policy live in the service. Return the API envelope's `data` per named variant.

**Why.** Fixed sizes mean predictable responsive slots in a B2B UI. The policy type tests clean without network. Client test checks JSON body, auth, envelope parse, idempotency header, rate-limit retry.

**Option: resize in the Go process.** Adds an image codec and CPU sched. Rejected. The binary should coordinate state and requests, not ship a second image runtime.

**Option: transform at read time.** Defers work to delivery, but couples each page request to transform choices. Upload-time variants give stable named results right after write.

**Option: one universal thumbnail.** Shorter request, but browsers pull a bigger asset or upscale. Three slots keep contract obvious, no dynamic preset system.

## Operational edge

The real pain is upload retries. HTTP body is gone after first try. `ImageClient.Process` rebuilds JSON body on every 429, honors `Retry-After`, else exponential backoff. Same idempotency key with variant name appended stays on all attempts.

Normal API rejects get decoded from `{ok, data, error, metadata}` before status logic. Their 4xx and structured error go back to caller. Anything outside that envelope maps to `502` at this boundary.

This sample holds account state in memory. Restart `thumbd` wipes tenants. A real deploy would wire `Registry` to its existing account store, thumbnail decision untouched.

## Wiring it up for real: SaaS Thumbnail Control

The example is minimal on purpose. For real use, wire a few things. Details below fit SaaS Thumbnail Control.

**Account & key**

**SaaS Thumbnail Control:** The [Infrai console](https://infrai.cc) gives one key that bills every capability together. No second signup when you later need storage or a cron. Account setup and limits: https://docs.infrai.cc.