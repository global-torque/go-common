# Step 2: HTTP `Idempotency-Key`

Status: planning document only. This file does not claim that the feature is implemented.

This plan follows [the Echo v5 migration plan](./echo-5-migration.md) and is
coordinated with [step 3, Atomic operations](./step-3-atomic-operations.md).

## Goal

Introduce HTTP idempotency in two deliberately separate phases:

1. **Observation:** consume and measure the existing frontend header without
   changing any response or request execution behavior.
2. **Enforcement:** later prevent duplicate execution and replay completed
   responses after the subject, provider-outcome, reconciliation, database, and
   CORS prerequisites are complete.

The initial rollout on frontend provider routes is observation-only. It does not
read bodies, fingerprint requests, persist keys, claim keys, replay responses,
block requests, or alter responses. A missing or invalid key changes metrics only.

This is HTTP request idempotency. It is not Pub/Sub message deduplication and it
is not provider-side idempotency.

Planning estimate: 13 points — this crosses go-common, the canonical migration
repository, two Go services, browser SDK behavior, CORS manifests, provider
reconciliation, and PostgreSQL integration tests.

## Verified current state

### Shared Go repository

- `go-common` is a multi-module repository. Import paths are module-specific,
  such as `github.com/global-torque/go-common/server/v3`.
- `server/http.go` registers route middleware through `route.Route.Middlewares`
  in `HTTPServer.AddRoute`. `server/handler.go` gathers route groups through Fx.
  Observation and future enforcement therefore have explicit route owners and
  must not become an implicit global method switch.
- The current worktree contains candidate `db/scope.go` and
  `server/middleware/atomic.go` files, but the published `db/v2 v2.0.2` and
  `server/v2 v2.0.3` tags do not contain those files. The published tags are the
  versions currently consumed by the services, so local candidate files are not
  released contracts.
- `CheckIdentityID` currently stores and logs the raw `Authorization` header.
  That value is not a stable, validated, non-secret subject. It must never be
  persisted, hashed into an idempotency fingerprint, or logged for idempotency.
  This plan does not redesign authentication.
- The current `server/http.go` worktree change lists `Idempotency-Key` in its
  CORS allowlist, and `server/http_test.go` asserts that the preflight response
  includes it. Preserve those unrelated changes. Before changing `server/v3`,
  reconcile its CORS behavior with released `server/v2.0.3`. Echo v5 uses the
  configured `[]string` allowlist from step 1.
- `queue.Deduper` and the Pub/Sub listener packages are unrelated. They must not
  be reused as HTTP idempotency storage.
- `migrations/15_incoming_requests/01_create_table.sql` defines a contact/form
  table with email, name, and form data. It is not an idempotency table and must
  not be reused.

### Current consumers

- KYC pins `github.com/global-torque/go-common/db/v2 v2.0.2` and
  `github.com/global-torque/go-common/server/v2 v2.0.3` in `go.mod`.
- Escrow pins the same `db/v2 v2.0.2` and `server/v2 v2.0.3` versions.
- KYC HTTP routes are declared in `kyc-api/internal/ports/http/auth/routers.go`,
  `internal/ports/http/inter/routers.go`, and
  `internal/ports/http/pubsubpush/routers.go`.
- Escrow HTTP routes are declared in `escrow-api/internal/ports/http/routes.go`
  and `internal/ports/http/pubsubpush/routes.go`.
- Neither `kyc-api` nor `escrow-api` currently reads, validates, persists, or
  deduplicates `Idempotency-Key`.

### Frontend contract

The following behavior is already present and is a dependency of this plan.

- `sdk/src/client.ts` validates an explicit `idempotencyKey` only for mutation
  methods. It rejects blank or non-string values, values longer than 256
  characters, and values with unsafe header characters. `prepareHeaders`
  injects the validated value as `Idempotency-Key` and rejects caller-supplied
  security-header overrides.
- `sdk/src/client.ts` creates default keys only for `POST` requests using the
  standard header policy, with no explicit key, and with an absent or string
  body. It does not automatically retry `POST`.
- `sdk/src/idempotency.ts` creates a `randomUUID()` per action scope. Overlapping
  calls share that key. A rejected action retains its key for the next attempt,
  and a successful action clears it.
- `torque-packages/packages/invest-data` forwards both
  `defaultIdempotencyKeys` and `idempotencyKey`. Its Kratos client opts out of
  default keys because Kratos does not allow the header.
- `invest-features` uses this client for KYC `POST /auth/kyc/:profileId`,
  accreditation JSON POSTs, and the null-body escrow POST. These calls receive
  default keys through the existing compatibility transport.
- The accreditation document upload uses `FormData`. It does not receive an
  automatic default key unless the frontend sends an explicit key.

Relevant evidence is in:

- `../sdk/src/client.ts`
- `../sdk/src/idempotency.ts`
- `../torque-packages/packages/invest-data/src/service/apiClient.ts`
- `../torque-packages/packages/invest-data/src/service/dataClientConfig.ts`
- `../torque-packages/packages/invest-features/src/kyc/model/useKycModel.ts`
- `../torque-packages/packages/invest-features/src/accreditation/useAccreditationModel.ts`

## Scope and non-goals

### In scope

- An observe-only route middleware for the initial frontend provider routes.
- A hard prerequisite for a service-owned validated subject context.
- A per-request provider-outcome reporter or equivalent explicit signal.
- One canonical PostgreSQL idempotency table prepared through
  `/home/adams/projects/global-torque/i-migration-service`.
- Runtime grants for both KYC and escrow API database roles.
- A small, context-aware enforcement `Store` interface and service-owned
  PostgreSQL adapters, implemented only after the enforcement gates pass.
- Shared route-level observation and future enforcement middleware in the Echo v5
  `server/v3` module.
- Bounded request fingerprinting and bounded replayable responses for future
  enforcement.
- `CORS_ALLOW_HEADERS` acceptance in every environment before frontend rollout.
- One service-owned reconciliation procedure for each provider route before
  enforcement.

### Out of scope

- Pub/Sub deduplication, push delivery, or queue `Deduper` changes.
- Provider idempotency. Plaid and North Capital calls remain external side
  effects and cannot be rolled back by this feature.
- Authentication redesign. `CheckIdentityID` is not an acceptable subject source.
- Global middleware registration, broad mutation coverage, or a hidden
  environment feature flag.
- The multipart document upload during the first rollout.
- JSON canonicalization. Future fingerprints use the raw request-body digest.
- A backend proxy, alternate service URL, frontend retry loop, or new provider
  endpoint.
- The database transaction and response coupling addressed by
  [step 3](./step-3-atomic-operations.md).

## Design and contract

### Phase 1: observation-only consumption

The initial middleware is named and treated as observation-only, such as
`IdempotencyObserve`. Its invocation order is:

```text
existing access middleware -> IdempotencyObserve -> handler
```

For the initial frontend provider routes, it may:

- record whether the header is present;
- validate the same mutation-key shape used by the SDK;
- emit bounded presence/validity metrics;
- record route and request ID metadata without the body or raw header value.

It must not:

- read or hash the request body;
- resolve or persist an actor;
- compute a fingerprint;
- acquire an advisory or row lock;
- insert, update, or query an idempotency record;
- replay or buffer a response;
- return a key-related error or `409`;
- change status, body, headers, timing contract, or provider execution.

Missing and invalid keys remain ordinary requests during observation. This phase
proves browser CORS and header delivery while the enforcement prerequisites are
built. Observation may run before the canonical table or runtime grants exist.

### Validated subject prerequisite

`CheckIdentityID` cannot supply the enforcement actor. It stores/logs the raw
`Authorization` header, which is neither a stable subject nor safe idempotency
data. The enforcement route must remain observation-only until each service owns a
validated subject context contract.

The shared middleware may accept a narrow `SubjectResolver` that reads only
validated claims or a validated service-owned context value. The resolver must:

- return a stable subject and any required tenant/scope;
- reject missing or invalid claims;
- never read the raw `Authorization` header;
- never return the raw token or a mutable display value;
- expose only the fields needed to scope idempotency.

The service owns authentication and authorization. This plan does not redesign
them. The shared middleware only consumes the validated resolver contract. There
is no fallback to `CheckIdentityID`, a raw header, or an anonymous actor for
enforcement. If the resolver is absent or cannot resolve a validated subject,
that route stays observation-only.

Future enforcement order is:

```text
validated authentication/authorization -> Idempotency -> handler
```

The raw `Authorization` header must never be persisted, hashed into the
fingerprint, or logged for idempotency. A validated subject may be stored as the
scope key only under the service contract. Logs use an internal record ID,
request ID, route, and state, not the raw header or full key.

### Future enforcement header policy

When all enforcement gates are complete:

1. Read `Idempotency-Key` only after the `SubjectResolver` has returned the
   validated actor/scope.
2. Reject blank values, values longer than 256 characters, and values unsafe for
   an HTTP header. Keep the same limits as `sdk/src/client.ts`.
3. Do not accept a key on `GET`, `HEAD`, or `OPTIONS`. The enforcement middleware
   skips those methods even if a client sends the header.
4. Register enforcement only on explicit route definitions. Construct it through
   Fx and append it to each selected `route.Route`.
5. Verify actual Echo middleware invocation order with a route test. Do not infer
   order from the position of functions in a slice.

### Request identity and future fingerprint

The future service adapter stores these fields:

- service name;
- HTTP method;
- normalized route template, using the matched route rather than the concrete
  profile or user value;
- validated actor and scope;
- raw key, bounded to 256 characters;
- SHA-256 request fingerprint;
- state;
- bounded response status, body, and selected headers;
- creation, update, completion, and expiry timestamps as applicable.

The future fingerprint is a SHA-256 digest over a length-delimited
representation of:

1. the upper-case method;
2. the normalized route template;
3. the normalized request path and query;
4. the validated actor/scope;
5. the digest of the raw request body.

It never includes the raw `Authorization` header. The normalization helper must
have a stable, tested definition that preserves duplicate query values. It must
not reorder or rewrite JSON. A body with zero bytes has a digest of the zero-byte
input. The implementation reads at most the configured request limit plus one
byte, returns `413` when the limit is exceeded, and restores `Request.Body`
before the handler runs:

```go
raw, err := io.ReadAll(io.LimitReader(request.Body, maxBytes+1))
request.Body = io.NopCloser(bytes.NewReader(raw))
```

The snippet describes required behavior, not a drop-in implementation. The
observation phase does not perform any of these body operations.

Multipart requests are excluded initially. Add them only after the frontend
sends an explicit scoped key and the service defines safe multipart fingerprint
and storage limits.

### PostgreSQL table and uniqueness

Use a new canonical table, proposed as `http_idempotency_records`, in
`/home/adams/projects/global-torque/i-migration-service`. Do not add it to
`migrations/15_incoming_requests`.

The table must include the fields listed above and a database-enforced unique key
over:

```text
service + validated actor/scope + method + normalized route template + raw key
```

Use an explicit state vocabulary containing at least:

- `in_progress` — a claim is running;
- `completed` — a successful response is available for replay;
- `indeterminate` — execution outcome is unknown and must not be rerun.

Keep response storage bounded. Store only successful `2xx` and `3xx` responses
initially. Persist only a selected safe response-header allowlist, such as
content type and location where applicable. Never persist authentication,
`Set-Cookie`, request ID, or other per-request headers for replay.

The table migration must define expiry columns and lookup indexes for bounded
cleanup. It must not rely on request-path full-table cleanup. No idempotency row
is written during the initial observation phase.

### Future non-blocking claim protocol

The enforcement store must acquire a deterministic PostgreSQL advisory
transaction lock from the canonical scope before exact lookup or insert:

```text
pg_try_advisory_xact_lock(derived canonical scope)
```

If the try-lock is unavailable, return an immediate documented retryable conflict.
Do not wait on a row lock or unique-index insert. Retain the exact database unique
constraint as the final invariant.

The advisory key is derived from service, validated actor/scope, method, route
template, and raw key. A hash collision can serialize unrelated scopes and cause
only a transient conflict. It must not cause a wrong replay or deduplication,
because the exact unique constraint and fingerprint comparison still use the full
values.

The future store and middleware implement these transitions:

| Existing record | Matching fingerprint | Result |
| --- | --- | --- |
| none | n/a | Claim as `in_progress` and run once |
| `in_progress` | same | Immediate retryable `409`; never run twice |
| `completed` | same | Replay saved status, body, and selected headers |
| any state | different | `409` fingerprint conflict; never run |
| `indeterminate` | same | `409` reconciliation-required conflict; never rerun |

The claim operation is atomic and never exposes a check-then-insert race. A
non-Atomic provider route commits its claim before handler execution. A later
Atomic integration may hold the advisory transaction lock in the root scope;
that is part of the separately gated step 3 coupling, not the initial Atomic
rollout.

### Provider outcome contract

HTTP status never classifies whether a provider operation executed. Each future
enforced route must install a per-request service-owned outcome reporter or an
equivalent explicit signal before calling the provider. The reporter has a
monotonic state:

- **not started:** no provider call has begun;
- **pre-provider safe failure:** the service proves the provider cannot have
  started;
- **provider started:** at least one provider call began;
- **provider outcome unknown:** the provider result cannot be established;
- **provider succeeded:** the provider result and required local work succeeded.

The middleware applies only these rules:

- successful handler status `2xx` or `3xx` completes and stores the bounded
  response;
- an explicit pre-provider-safe signal releases the claim, and the signal is
  rejected if provider-started was ever recorded;
- every unresolved non-success after a possible provider call becomes
  `indeterminate`;
- panic, process crash, timeout, cancellation, and unknown provider response are
  indeterminate;
- no HTTP `4xx` or `5xx` status alone releases a claim or proves a provider
  no-effect outcome.

The service must not infer safety from a lost response. A crash after an external
side effect but before completion leaves an unresolved claim for reconciliation;
expiry must never automatically rerun the provider operation.

### Reconciliation gate

Before enforcing any frontend provider route, define and test one
service-owned reconciliation procedure for that exact route. Each procedure uses
the internal idempotency record ID and safe provider correlation data. It can:

1. complete the record after authoritative provider success evidence;
2. release the record only after authoritative evidence that no provider effect
   occurred; or
3. leave the record indeterminate when evidence is incomplete.

The required initial procedures are:

- KYC `CreateKYC`, including both Plaid calls;
- escrow accreditation create;
- escrow accreditation update;
- escrow account creation, including party, account, and link creation.

If provider evidence is unavailable, the route remains observation-only. Do not
add a generic saga framework. Keep each procedure in its service-owned provider
and reconciliation boundary.

### Store boundary

Define the future shared server contract around a small context-aware interface.
The exact names can follow local Go style, but the interface must express these
operations without importing a concrete service repository:

```go
type Store interface {
    Claim(context.Context, Request) (ClaimResult, error)
    Complete(context.Context, Claim, ReplayResponse) error
    Release(context.Context, Claim) error
    MarkIndeterminate(context.Context, Claim, string) error
}
```

The context argument is mandatory on every method. Service adapters implement the
interface with their own PostgreSQL connection and SQL. The shared `server/v3`
middleware must not import KYC, escrow, `i-models`, or a concrete `db.DB`.

Context-aware store calls are required so a later, separately approved coupling
with [step 3](./step-3-atomic-operations.md) can use the root transaction. The
initial Atomic rollout does not construct this store, read a key, or write an
idempotency record.

### Logging, limits, and cleanup

- Never log the raw `Authorization` header, request body, raw idempotency key,
  response body, or full fingerprint. Use request ID, service, route template,
  outcome category, and an internal record ID only after enforcement exists.
- Require explicit operational configuration in every applicable manifest. At a
  minimum, define and validate request-body limit, replay-response limit, claim
  expiry, completed-record retention, and cleanup batch size. Do not silently
  default correctness or storage limits.
- The owning API service owns bounded cleanup for its service rows. Cleanup runs
  through an existing maintenance or Fx-owned process, deletes only expired rows
  in bounded batches, and never scans or deletes the full table on the request
  path. The migration repository owns schema and grants, not runtime cleanup.
- Stale `in_progress` and `indeterminate` records require the route's explicit
  reconciliation procedure. Cleanup must not make an unknown provider outcome
  look safe to rerun.

## Repository-specific route matrix

The route templates below use the current Go route spellings. The frontend paths
omit the `/v1.0` prefix when calling the configured service base URL.

| Service | Method and route | Current caller or behavior | Step 2 decision |
| --- | --- | --- | --- |
| KYC | `POST /v1.0/auth/kyc/:profile_id` | `invest-features/src/kyc/model/useKycModel.ts` sends JSON `{}` through invest-data; it receives a default key. | Initial observation only. Future enforcement requires both Plaid reconciliation and a validated subject. |
| KYC | `POST /v1.0/internal/kyc/:profile_id/manual` | `kyc-api/internal/ports/http/inter/routers.go`; internal manual DB workflow. | Not part of the frontend provider observation. Consider future enforcement only after an explicit internal client and subject contract. |
| KYC | `PUT /v1.0/internal/kyc/:profile_id/sync` | Provider sync in `kyc-api/internal/ports/http/inter`. | Exclude from initial observation and enforcement. |
| KYC | `POST /pubsub/domain-events` | Pub/Sub push acknowledgement path. | Exclude. |
| KYC | `POST /pubsub/webhooks` | Pub/Sub webhook acknowledgement and queue dedupe path. | Exclude. |
| Escrow | `POST /v1.0/auth/accreditation/create/:profile_id` | `invest-features/src/accreditation/useAccreditationModel.ts` sends JSON. | Initial observation only. Future enforcement requires route reconciliation and a validated subject. |
| Escrow | `POST /v1.0/auth/accreditation/update/:profile_id` | Same model sends JSON. | Initial observation only. Future enforcement requires route reconciliation and a validated subject. |
| Escrow | `POST /v1.0/auth/escrow/:user_id/:profile_id` | Same model sends a null body with JSON headers; it receives a default key. | Initial observation only. Future enforcement requires party/account/link reconciliation and a validated subject. |
| Escrow | `POST /v1.0/auth/accreditation/upload_document/:user_id/:profile_id` | Same model sends `FormData`. | Exclude until an explicit key, multipart fingerprint, and storage limits exist. |
| Escrow | `POST /v1.0/internal/accreditation/manual/:user_id/:profile_id` | Internal database-only manual update. | Not part of the frontend provider observation. It remains a step 3 Atomic candidate without idempotency in the initial rollout. |
| Escrow | `PUT /v1.0/internal/accreditation/sync/:user_id/:profile_id` | North Capital synchronization. | Exclude from initial observation and enforcement. |
| Escrow | `POST /v1.0/internal/accreditation/update_status` | Provider webhook-style status callback. | Exclude as an internal callback/provider boundary. |
| Escrow | `POST /v1.0/auth/issuer/:user_id/:profile_id` | North Capital issuer workflow. | Exclude from initial observation and enforcement. |
| Escrow | `POST /v1.0/internal/escrow/offering/:user_id/:offer_id` | North Capital offering workflow. | Exclude from initial observation and enforcement. |
| Escrow | `POST /pubsub/domain-events` and `POST /pubsub/webhooks` | Pub/Sub push contracts. | Exclude. |

## Implementation sequence by repository

### 1. `go-common`

1. Reconcile the Echo v5 `server/v3` CORS implementation with released
   `server/v2.0.3` behavior before changing the shared header contract.
2. Add the v5-safe `IdempotencyObserve` middleware. It validates presence and
   shape for metrics only and never reads a body, persists a key, claims a key,
   buffers a response, blocks, or changes a response.
3. Define the narrow validated `SubjectResolver` and per-request provider outcome
   contract for future enforcement. Do not read `Authorization` in shared
   idempotency code and do not redesign auth.
4. Add the future enforcement middleware, bounded reader, fingerprint helper,
   advisory-lock claim protocol, replay capture, state transitions, and redacted
   logging only behind the documented gates.
5. Keep the shared package independent of PostgreSQL and service code. The
   context-aware `Store` interface must not import a concrete service repository.
6. Require `Idempotency-Key` through the Echo v5 configured allowlist after
   `CORS_ALLOW_HEADERS` behavior is reconciled. Do not overwrite unrelated
   worktree changes in `server/http.go`, `server/http_test.go`, or this task
   directory.

### 2. `/home/adams/projects/global-torque/i-migration-service`

1. Add the canonical `http_idempotency_records` schema in a new forward-only
   object-group track, using the next valid numeric migration prefix after the
   existing tracks. Do not edit an applied migration.
2. Add lookup indexes, the exact scoped unique constraint, bounded response
   columns, state, timestamps, and expiry. Keep business validation in service
   adapters; use PostgreSQL for structural integrity and uniqueness.
3. Add runtime grants for both independently deployed API roles under their
   service `100_* /roles` tracks. Resolve actual environment role names before
   writing grants. Test Compose files using `postgres` do not prove production
   role names.
4. Apply and validate the migration before future enforcement. The observation
   phase does not depend on the table.

### 3. `kyc-api`

1. After step 1 publishes `server/v3`, pin published shared modules rather than
   relying on local replacements.
2. Wire `IdempotencyObserve` only to
   `POST /v1.0/auth/kyc/:profile_id` after the existing access middleware. The
   route must retain its current status, body, headers, and provider calls.
3. Before future enforcement, add a service-owned validated subject resolver,
   outcome reporter, and reconciliation procedure for `CreateKYC`, including
   both Plaid calls. Keep the raw `Authorization` value out of all idempotency
   data and logs.
4. Add the PostgreSQL `Store` adapter only after the subject and reconciliation
   gates pass. Do not add the generic record as a domain profile model.
5. Add explicit `CORS_ALLOW_HEADERS` configuration to every KYC environment
   manifest, including `.env.tests`, Compose, and deployment settings. The value
   must include `Idempotency-Key` before browser rollout.

### 4. `escrow-api`

1. Pin the published `server/v3` and compatible `db/v2` modules after step 1.
2. Wire `IdempotencyObserve` only to the three selected JSON/null frontend
   routes in `internal/ports/http/routes.go`, after the existing access
   middleware. Do not change response or provider behavior.
3. Before future enforcement, add service-owned validated subject and outcome
   contracts plus one tested reconciliation procedure for each route:
   accreditation create, accreditation update, and escrow account creation
   including party, account, and link.
4. Add the PostgreSQL `Store` adapter only after those gates pass. Do not add it
   to upload, provider, webhook, or Pub/Sub routes.
5. Preserve `FormData` upload behavior and require an explicit future frontend
   key before reconsidering that route.
6. Add explicit `CORS_ALLOW_HEADERS` configuration to `.env.docker`, test
   manifests, and deployment settings. The value must include `Idempotency-Key`
   before browser rollout.

### 5. `sdk` and `torque-packages`

1. Treat the existing SDK and invest-data behavior as the client contract. Do
   not add frontend retries as part of this step.
2. Keep explicit keys available for future multipart rollout, but do not send a
   default key for `FormData` in the initial implementation.
3. Run contract tests during observation to prove default key generation, key
   reuse after rejection, key clearing after success, and Kratos opt-out. No
   frontend route change is required for observation.

## Tests and validation

### Observation tests

Add focused route tests proving:

- valid, missing, and invalid headers produce presence/validity metrics only;
- observation does not read or restore the body, compute a fingerprint, or touch
  PostgreSQL;
- observation does not acquire an advisory lock or block;
- observation does not replay, buffer, or alter status, body, headers, or provider
  calls;
- the order is existing access middleware, `IdempotencyObserve`, then handler;
- excluded GET, HEAD, OPTIONS, multipart, webhook, Pub/Sub, and callback routes
  are not observed or enforced;
- browser preflight accepts `Idempotency-Key` through `CORS_ALLOW_HEADERS`.

### Future enforcement tests

Do not treat these tests as evidence that initial provider enforcement is ready.
Add them only after the subject and reconciliation gates pass:

- explicit mutation-key validation and missing/optional policy;
- body restoration, request-size rejection, normalized route/path/query, and
  raw-body fingerprint stability;
- independent-connection concurrency proving one handler execution;
- `pg_try_advisory_xact_lock` unavailable returns immediate retryable conflict;
- deterministic advisory-hash collisions cause only transient conflict and do
  not cross-replay records;
- completed response replay without handler execution;
- same-key fingerprint conflict;
- stale `in_progress` and `indeterminate` handling;
- bounded successful response status/body/header storage;
- explicit pre-provider-safe release;
- provider-started failure, panic, crash, timeout, cancellation, and unknown
  response become indeterminate regardless of HTTP status;
- each of the four route reconciliation procedures can complete, release only
  with authoritative no-effect evidence, or remain indeterminate;
- real PostgreSQL claim, unique-conflict, replay, expiry, and cleanup behavior;
- real enforced route wiring rather than only middleware unit calls.

Use existing commands where the repositories define them:

```sh
(cd /home/adams/projects/global-torque/go-common/server && go vet ./... && go test -count=1 ./...)
(cd /home/adams/projects/global-torque/go-common/db && go vet ./... && go test -count=1 ./...)
(cd /home/adams/projects/global-torque/i-migration-service && ./make.sh test)
(cd /home/adams/projects/global-torque/kyc-api && ./make.sh test)
(cd /home/adams/projects/global-torque/escrow-api && ./make.sh test)
(cd /home/adams/projects/global-torque/escrow-api && ./make.sh test docker)
(cd /home/adams/projects/global-torque/sdk && pnpm run typecheck && pnpm run test:run)
(cd /home/adams/projects/global-torque/torque-packages && pnpm --filter @global-torque/invest-data run typecheck && pnpm --filter @global-torque/invest-data run test:run && pnpm --filter @global-torque/invest-features run typecheck && pnpm --filter @global-torque/invest-features run test:run)
```

Run the KYC integration stack from `/home/adams/projects/global-torque/kyc-api`
with cleanup even when the test command fails:

```sh
set -o errexit
cleanup() {
  exit_code=$?
  docker compose down
  exit "$exit_code"
}
trap cleanup EXIT
I_MIGRATION_SERVICE_DIR=../i-migration-service docker compose up --abort-on-container-exit --exit-code-from tests tests
```

Run PostgreSQL integration tests against disposable infrastructure only. Do not
source an environment file that points to shared or production infrastructure.
Run KYC and escrow integration stacks separately when they share database state.

### Reconciliation validation

For each future provider route, test a provider success, authoritative no-effect
evidence, unavailable evidence, timeout, cancellation, process-crash recovery,
and a response received after the provider call. The unavailable-evidence case
must leave the route observation-only or the record indeterminate. It must never
release and retry automatically.

## Rollout and rollback

### Observation gates

The observation rollout is ready only when:

1. `server/v3` CORS behavior has been reconciled with released `server/v2.0.3`.
2. Every target environment's `CORS_ALLOW_HEADERS` includes `Idempotency-Key`.
3. Observation tests prove no body read, persistence, claim, replay, blocking,
   or response change.
4. Metrics are bounded and exclude raw authorization, body, and full key values.
5. The four frontend provider routes can be observed one route at a time.

Deploy observation first and verify the browser preflight and route request. A
header reaching a handler is not evidence that idempotency is enforced.

### Enforcement gates

Do not enforce any provider route until all of these are complete:

1. The service-owned validated subject resolver is present and tested.
2. The outcome reporter prevents safe release after provider-start.
3. The route's reconciliation procedure is defined, tested, and uses internal
   record ID plus safe provider correlation data.
4. Provider evidence supports completion or authoritative no-effect release. If
   it does not, the route remains observation-only.
5. The canonical table, grants, bounded configuration, advisory-lock protocol,
   unique constraint, and independent-connection tests pass.
6. The future replay and failure tests pass without changing provider contracts.

After these gates, enforce one route and one service at a time, starting with
KYC create, then escrow accreditation create/update, then escrow account
creation. Internal manual routes require a separate explicit client and subject
contract.

### Rollback

- Roll back observation or future enforcement by removing the explicit route
  middleware and deploying the previous service revision.
- Keep additive schema and grants in place for compatible rollback images.
- Do not delete records during an application rollback.
- Reconciliation must decide provider outcomes before a stale or indeterminate
  key can be retried. A rollback must not turn an unknown provider result into a
  second provider call.

## Acceptance criteria

### Observation gates

1. The initial frontend provider rollout is observe-only.
2. Missing or invalid keys change metrics only and do not change response or
   provider behavior.
3. Observation does not read bodies, fingerprint requests, touch PostgreSQL,
   claim keys, replay responses, block requests, or persist any actor or key.
4. The order is existing access middleware, `IdempotencyObserve`, then handler.
5. `server/v3` CORS behavior is reconciled with released `server/v2.0.3`, and
   every target environment's `CORS_ALLOW_HEADERS` includes `Idempotency-Key`.

### Enforcement gates

1. No provider route is enforced before a service-owned validated subject context
   exists. Raw `Authorization` is never persisted, hashed, or logged for
   idempotency.
2. No provider route is enforced before its reconciliation procedure is tested.
   Unavailable provider evidence keeps the route observation-only.
3. The canonical table is in `/home/adams/projects/global-torque/i-migration-service`,
   not `incoming_requests`, and runtime grants cover both API roles.
4. The unique scope includes service, validated actor/scope, method, route
   template, and raw key. Fingerprints include method, normalized route/path/query,
   validated actor/scope, and raw request-body digest with no JSON
   canonicalization.
5. Claims use deterministic `pg_try_advisory_xact_lock` before exact lookup/insert,
   return immediate retryable conflict when unavailable, retain the exact unique
   constraint, and pass independent-connection concurrency tests.
6. Completed `2xx` and `3xx` responses replay, changed fingerprints return
   `409`, and indeterminate records never rerun automatically.
7. HTTP status never classifies provider execution. Only an explicit
   pre-provider-safe signal releases; every unresolved possible-provider failure
   is indeterminate.
8. The rollout has observation evidence, route-level enforcement gates, metrics,
   reconciliation ownership, and a route-removal rollback procedure.

Provider enforcement is not complete or possible merely because the SDK sends a
header. It remains blocked until the validated subject and reconciliation gates
are met.

## Risks and open decisions

- **Subject scope:** `CheckIdentityID` is not a subject source. Each service must
  define and validate its claims-to-subject context without redesigning shared
  authentication in this plan.
- **Provider outcome:** A response status cannot prove whether Plaid or North
  Capital executed. The per-request reporter must reject a safe-release signal
  after provider-start.
- **Reconciliation evidence:** Provider correlation data and internal record IDs
  must be sufficient to complete or release safely. Missing evidence keeps a
  route observation-only; no generic saga framework is allowed.
- **Advisory lock collisions:** A hash collision can create a transient conflict
  for unrelated scopes. It must not affect exact uniqueness, fingerprint checks,
  or replay correctness.
- **Route normalization:** The exact standard-library normalization of escaped
  paths and duplicate query values must be fixed before enforcement consumers
  ship.
- **Response headers:** The replay allowlist must exclude credentials, cookies,
  request-specific metadata, and any provider-sensitive data.
- **Retention:** Operations must choose claim expiry, completed retention,
  cleanup batch size, and reconciliation ownership. These are required
  configuration, not silent defaults.
- **Published boundaries:** Services must move from the current v2 tags to the
  Echo v5/server v3 contract before future enforcement can be released.
- **Atomic boundary:** Idempotency alone does not make business writes and the
  completion record one transaction. [Step 3](./step-3-atomic-operations.md)
  addresses the database-only transaction boundary separately. It cannot make
  provider calls atomic.
