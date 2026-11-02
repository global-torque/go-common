# Step 3: Atomic operations

Status: planning document only. This file does not claim that Atomic has been
implemented or released.

This plan follows [the Echo v5 migration plan](./echo-5-migration.md). The
initial Atomic rollout is intentionally independent of [step 2](./step-2-idempotency-key.md).

## Goal

Provide request-level atomic behavior for two existing database-only HTTP routes:
business PostgreSQL writes commit, and the buffered HTTP response is delivered
only after that commit. The HTTP response does not commit with the database.
An error rolls back the business database writes before an error response is
delivered.

Atomic is one PostgreSQL transaction plus a buffered HTTP response. It is not a
distributed transaction and cannot roll back Plaid, North Capital, Pub/Sub, or
any other external side effect.

Planning estimate: 13 points — this requires the Echo v5/server v3 redesign,
context-aware PostgreSQL scope behavior, nested transaction compatibility, two
service route changes, and real PostgreSQL HTTP tests.

## Verified current state

### Shared Go repository

- The current worktree contains candidate `db/scope.go`,
  `server/middleware/atomic.go`, and `server/middleware/atomic_buffer.go` files.
  The candidate Atomic implementation still imports Echo v4 and accepts
  `echo.Context` by value, so it is not the required Echo v5 implementation.
- Published `db/v2 v2.0.2` does not contain `scope.go`. Published
  `server/v2 v2.0.3` does not contain `middleware/atomic.go`.
- `db/scope.go` in the current worktree demonstrates the intended direction:
  `BeginScope` binds a root transaction to context, DB operations use that
  context, and nested `Begin` calls become savepoints. `orm.WithTx` already
  calls `Begin`, so the compatibility behavior can be tested without an
  immediate repository-wide refactor.
- `server/middleware/atomic_test.go` and
  `server/middleware/atomic_integration_test.go` already identify important
  behaviors, including commit, rollback, panic cleanup, response limits, hooks,
  and real PostgreSQL scope behavior. Port those tests to Echo v5 rather than
  treating the v4 implementation as a drop-in design.
- `server/http.go` registers route middleware from explicit `route.Route` values
  through `AddRoute`; `server/handler.go` supplies route groups through Fx.
  This supports explicit route wiring and does not authorize global Atomic.

### Current consumers and route behavior

- KYC and escrow currently pin `db/v2 v2.0.2` and `server/v2 v2.0.3`, and their
  current module graphs use Echo v4. They do not use Atomic.
- KYC's internal manual route is
  `POST /v1.0/internal/kyc/:profile_id/manual`. Its handler calls
  `ManualKYC` with the request context. `internal/adapters/db/manual_kyc.go`
  already uses `orm.WithTx` to update the profile and audit row together.
- Escrow's internal manual route is
  `POST /v1.0/internal/accreditation/manual/:user_id/:profile_id`. Its handler
  calls `Application.AccreditationManual`, and
  `internal/adapters/postgres.go` already begins a local transaction in
  `UpdateProfileAccreditationByID` for the profile and related updates.
- The current selected handlers are database-only. The authenticated KYC create
  route calls Plaid. Escrow accreditation, escrow-account, upload, issuer, and
  offering routes call North Capital or use multipart data.
- Current consumers do not use the candidate `BeginScope` or Atomic APIs because
  the released tags do not provide them.

## Prerequisites

Do not begin service route wiring until all prerequisites are true:

1. The Echo v5 migration in
   [the migration plan](./echo-5-migration.md) is complete for the shared
   server boundary and has passed its response, middleware, and service tests.
2. `db/v2` has a published release containing `BeginScope` and the context-aware
   DB methods required by the selected adapters.
3. `server/v3` has a published release containing the Echo v5 `NewAtomic`
   middleware and its bounded response contract.
4. KYC and escrow have been tested against published modules, not only local
   `replace` directives.

Step 2 observation, `Idempotency-Key`, its actor scope, and its PostgreSQL
records are not prerequisites for this initial Atomic rollout.

## Scope and non-goals

### In scope

- A v5-safe `server/v3` Atomic middleware.
- Context-bound PostgreSQL transaction scopes in `db/v2`.
- Savepoint behavior for existing `Begin` and `orm.WithTx` callers.
- Explicit route wiring for two existing database-only manual routes.
- Integration tests and operational canary checks.
- A later, clearly gated design for coupling Atomic with step 2.

### Out of scope for the initial rollout

- Any `Idempotency-Key` requirement, idempotency claim, replay, fingerprint, or
  idempotency record.
- Step 2 validated actor scope or provider-outcome reporting.
- Global Atomic registration, broad method-based switches, or hidden environment
  feature flags.
- Provider routes and distributed transaction claims.
- KYC create, KYC sync, KYC webhooks, KYC domain events, and all Pub/Sub routes.
- Escrow upload, create, update, sync, escrow-account, issuer, offering,
  provider-webhook, and Pub/Sub routes.
- Frontend changes.
- A provider saga, outbox, compensation framework, or durable command redesign.

## Design and contract

### Request and response boundary

`Atomic` starts a root PostgreSQL transaction for a selected request and stages
the HTTP response in a bounded buffer. The handler must finish its database work
before returning.

The Echo v5 implementation must:

1. Save the original request and the current `http.ResponseWriter`.
2. Install a separate staged Echo response backed by a bounded writer.
3. Run the handler with the original `*echo.Context` type required by Echo v5.
4. Restore the original request and response before delivery.
5. Commit or roll back before writing a successful response to the original
   writer.
6. Deliver the buffered successful response exactly once through the original
   writer after commit.

The database commit and HTTP response delivery are separate events. Atomic only
guarantees that business database writes commit before the response is delivered.
If delivery fails after commit, the database cannot be rolled back.

Do not embed or replace `*echo.Context`. Echo v5 makes `Context` a concrete
struct, and `Context.Response()` returns `http.ResponseWriter`. Use
`echo.UnwrapResponse` when response metadata or committed state is required.
Do not mutate the outer Echo response in place. Outer middleware such as body
dump and request logging must observe one final delivery, not staged writes and a
second delivery.

The staged writer must bound response bytes and reject flush, hijack, server push,
trailers, `text/event-stream`, transfer encoding, and unbounded streams. Finite
stream behavior must follow the decision and tests in step 1. Do not infer
streaming from write count or reader type.

### Initial middleware order and wiring

For the two selected routes, the initial invocation order is:

```text
recovery/logging and current access boundary -> Atomic -> handler
```

The current access boundary remains the service's existing boundary. Do not add a
new authentication system as part of Atomic. Authentication or authorization
changes made separately remain outside Atomic.

Register Atomic through explicit route middleware values constructed by Fx. Do
not call `Echo.Use(Atomic)` globally. Append Atomic only to the two route
definitions listed in the route matrix.

Echo registration order is easy to misread because middleware composition wraps
handlers. Add a route integration test that records entry and exit for the
current access boundary, Atomic, and the handler. The test must prove that the
root scope exists before the handler and that no successful response is delivered
before commit.

### Transaction behavior

`db.DB.BeginScope(ctx, options)` starts one root transaction and returns a context
that service adapters pass through their existing repository methods. Contextual
`Exec`, `Query`, `QueryRow`, batches, copy operations, and `Ping` must use the
root transaction while it is open.

Inside an existing scope:

- `db.DB.Begin` and `BeginTx` with zero options create PostgreSQL savepoints;
- `orm.WithTx` uses those savepoints and retains its current rollback-on-error
  behavior;
- nested transaction success releases a savepoint but does not commit the root;
- nested failure rolls back only the savepoint until the outer request decides
  the final outcome;
- non-zero nested transaction options are rejected rather than silently changing
  the root isolation level;
- direct pool acquisition inside a scope is rejected because it would bypass the
  root transaction.

Preserve existing local transactions initially. Audit the selected KYC and escrow
paths for direct `pgxpool.Pool` use, then prove that their current `Begin` and
`orm.WithTx` calls become savepoints through the context. Do not combine this
work with a broad repository transaction refactor.

### Outcome rules

| Outcome | Database | HTTP response |
| --- | --- | --- |
| Handler returns `nil`, status `2xx` or `3xx`, valid bounded response | Commit business writes | Deliver buffered response after commit |
| Handler error | Roll back business writes | Use the existing error handler; do not deliver staged success |
| Handler writes status `>=400` | Roll back business writes | Deliver the error response after rollback |
| Request cancellation | Roll back using a context that preserves cleanup time | Return the existing cancellation/error contract |
| Invalid, oversized, or unsupported response | Roll back business writes | Return `500` through the existing error boundary |
| Commit failure | Discard staged success and attempt cleanup rollback | Return `500`; never claim success |
| Delivery failure after commit | Business writes remain committed | Log the delivery failure; do not attempt a second response or rollback |

Recovery must remain outside Atomic. Panic cleanup must restore the original
request and response even when rollback or logging fails. Existing response
limits, short-write checks, invalid content-length checks, and callback behavior
must remain explicit tests.

## Repository-specific route matrix

| Service | Method and route | Current implementation evidence | Step 3 decision |
| --- | --- | --- | --- |
| KYC | `POST /v1.0/internal/kyc/:profile_id/manual` | `internal/ports/http/inter/routers.go` calls `ManualKYC`; `internal/adapters/db/manual_kyc.go` uses `orm.WithTx`. | First Atomic candidate. Add explicit route middleware. No key or idempotency record is required. |
| KYC | `POST /v1.0/auth/kyc/:profile_id` | `internal/ports/http/auth/routers.go`; handler creates a Plaid link/session. | Exclude. Provider side effect is not rollbackable. |
| KYC | `PUT /v1.0/internal/kyc/:profile_id/sync` | `internal/ports/http/inter/routers.go`; syncs with the KYC provider. | Exclude initially. |
| KYC | `POST /pubsub/domain-events` | `internal/ports/http/pubsubpush/routers.go`; Pub/Sub status contract. | Exclude. Keep acknowledgement behavior unchanged. |
| KYC | `POST /pubsub/webhooks` | Same push route group and queue dedupe path. | Exclude. |
| Escrow | `POST /v1.0/internal/accreditation/manual/:user_id/:profile_id` | `internal/ports/http/routes.go` calls `AccreditationManual`; `UpdateProfileAccreditationByID` begins a local PostgreSQL transaction. | First Atomic candidate. Add explicit route middleware. No key or idempotency record is required. |
| Escrow | `POST /v1.0/auth/accreditation/upload_document/:user_id/:profile_id` | Multipart upload to North Capital. | Exclude. |
| Escrow | `POST /v1.0/auth/accreditation/create/:profile_id` | Creates accreditation with North Capital. | Exclude initially. |
| Escrow | `POST /v1.0/auth/accreditation/update/:profile_id` | Updates accreditation through North Capital. | Exclude initially. |
| Escrow | `PUT /v1.0/internal/accreditation/sync/:user_id/:profile_id` | Synchronizes accreditation with North Capital. | Exclude initially. |
| Escrow | `POST /v1.0/auth/escrow/:user_id/:profile_id` | Creates party/account/link with North Capital. | Exclude initially. |
| Escrow | `POST /v1.0/auth/issuer/:user_id/:profile_id` | Creates an issuer with North Capital. | Exclude initially. |
| Escrow | `POST /v1.0/internal/escrow/offering/:user_id/:offer_id` | Creates an offering with North Capital. | Exclude initially. |
| Escrow | `POST /v1.0/internal/accreditation/update_status` | Provider webhook-style status callback. | Exclude. |
| Escrow | `POST /pubsub/domain-events` and `POST /pubsub/webhooks` | Push worker routes in `internal/ports/http/pubsubpush`. | Exclude. Preserve status contracts. |

Explicitly exclude KYC create/sync/webhooks/domain events and escrow
upload/create/update/sync/escrow/issuer/offering/provider webhook/Pub/Sub routes
from the first Atomic rollout.

## Implementation sequence by repository

### 1. `go-common`

1. Finish the Echo v5 migration and create the `server/v3` module boundary as
   specified by [the Echo v5 plan](./echo-5-migration.md).
2. Publish a compatible `db/v2` release containing `BeginScope`, contextual DB
   operations, savepoint-aware `Begin`/`BeginTx`, and scope tests.
3. Redesign `server/middleware/atomic.go` for Echo v5. Use `*echo.Context`, a
   separate staged response, original writer restoration, bounded buffering, and
   one delivery after commit.
4. Keep the public constructor small, for example `NewAtomic(starter,
   AtomicConfig)`, but validate required database, response, cleanup, and method
   configuration explicitly. Do not add an environment feature flag.
5. Port the existing Atomic unit and integration tests. Add v5 tests for
   response hooks, body dump, request logging, finite streams, flush, hijack,
   trailers, short writes, content lengths, panic cleanup, and response limits.
6. Publish `server/v3` only after direct module tests and the real PostgreSQL
   Atomic test pass.

### 2. `/home/adams/projects/global-torque/i-migration-service`

No new schema is required for the initial Atomic rollout. Do not make Atomic
depend on the migration service at runtime. If step 2's additive table exists,
Atomic must still run without reading or writing it.

### 3. `kyc-api`

1. Pin published `db/v2` and `server/v3` versions. Remove temporary local
   replacements before release validation.
2. Construct Atomic through the existing Fx composition root.
3. Add Atomic only to the route entry for
   `POST /v1.0/internal/kyc/:profile_id/manual` in
   `internal/ports/http/inter/routers.go`.
4. Pass the request context unchanged into `ManualKYC` and its DB adapter. Verify
   that `internal/adapters/db/manual_kyc.go` and its `orm.WithTx` call use the
   root scope and create savepoints rather than a separate pool transaction.
5. Add real route tests that force a later failure and prove all business writes
   roll back, then prove a successful request commits its business writes and
   delivers its buffered response after commit.
6. Do not add an `Idempotency-Key` requirement, idempotency store, actor scope,
   or idempotency completion record to this route in the initial rollout.
7. Do not change Plaid or Pub/Sub code in this step.

### 4. `escrow-api`

1. Pin published `db/v2` and `server/v3` versions and remove local replacements
   before release validation.
2. Construct Atomic through the existing Fx composition root.
3. Add Atomic only to the route entry for
   `POST /v1.0/internal/accreditation/manual/:user_id/:profile_id` in
   `internal/ports/http/routes.go`.
4. Preserve `Application.AccreditationManual` and its current repository
   transaction initially. Verify that `pg.client.Begin(ctx)` resolves through
   the context-aware DB adapter and becomes a savepoint inside the root scope.
5. Add real route tests that force a related-profile or business-event failure
   and prove all business writes roll back, then prove a successful request
   commits its business writes and delivers its buffered response after commit.
6. Do not add an `Idempotency-Key` requirement, idempotency store, actor scope,
   or idempotency completion record to this route in the initial rollout.
7. Do not change North Capital, upload, provider webhook, or Pub/Sub behavior.

### 5. `sdk` and `torque-packages`

No frontend changes are required for Atomic. Step 2 observation may continue
independently, but its header and actor contract must not be added to these
manual routes as part of the initial Atomic rollout.

## Later coupling with step 2

This is a later design, not an initial Atomic task. It is gated on all of the
following:

1. Internal clients send an explicit `Idempotency-Key` under a documented client
   contract.
2. Each service has a stable validated actor/subject context. Raw
   `Authorization` is never used.
3. Step 2's non-blocking `pg_try_advisory_xact_lock` claim coordination and exact
   unique constraint are proven with independent database connections.
4. Step 2's failure reporter and reconciliation procedure are stable.
5. Route tests prove this order:

   ```text
   validated actor -> Atomic -> Idempotency -> handler
   ```

When approved, the idempotency claim and successful completion record may use the
Atomic root transaction. Business writes and the completion record then commit
together. A failed handler rolls both back. This later coupling does not make
provider calls atomic and does not change the initial two-route Atomic scope.

## Tests and validation

### Shared module tests

Add or retain tests proving:

- `BeginScope` commits and rolls back all contextual DB operations;
- aliases sharing one pool see the root transaction;
- nested `Begin` and `orm.WithTx` use savepoints;
- nested rollback does not commit the root;
- nested transaction options are rejected;
- pool acquisition is rejected inside a scope;
- Atomic starts no scope for excluded methods or skipped routes;
- Atomic begins before the handler and does not deliver a response early;
- successful `2xx` and `3xx` responses commit business writes and deliver the
  buffered response once after commit;
- handler errors, status `>=400`, cancellation, invalid content length, and
  oversized responses roll back business writes;
- commit failure discards staged success and returns `500`;
- delivery failure after commit is logged without a second response or rollback;
- request and response state is restored after panic and cleanup failure;
- the two selected routes do not read, claim, or write idempotency records;
- provider and Pub/Sub routes do not start a transaction scope;
- Pub/Sub acknowledgement status contracts remain unchanged.

Run the affected shared-module checks from their module directories:

```sh
(cd /home/adams/projects/global-torque/go-common/db && go vet ./... && go test -count=1 ./... && go test -race -count=1 ./...)
(cd /home/adams/projects/global-torque/go-common/server && go vet ./... && go test -count=1 ./... && go test -race -count=1 ./...)
```

Run the real PostgreSQL Atomic test against a disposable PostgreSQL instance,
using the Echo v5 test name after the migration:

```sh
docker run --rm --detach --name go-common-atomic-postgres \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=atomic \
  -p 55432:5432 postgres:17-alpine
until docker exec go-common-atomic-postgres pg_isready -U postgres -d atomic; do sleep 1; done
ATOMIC_TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:55432/atomic?sslmode=disable' \
  go test -count=1 -run '^TestAtomicPostgres$' ./middleware
docker rm --force go-common-atomic-postgres
```

### Service integration checks

Use the existing service commands after the services pin published modules:

```sh
(cd /home/adams/projects/global-torque/kyc-api && go build -tags=mockplaidkyc ./cmd/http && go build -tags=mockplaidkyc ./cmd/worker && go build -tags=mockplaidkyc ./cmd/cloudrun && ./make.sh test)
(cd /home/adams/projects/global-torque/escrow-api && ./make.sh test)
(cd /home/adams/projects/global-torque/escrow-api && ./make.sh test docker)
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

Atomic adds no migration. If the release is validated together with the
canonical migration set, run its existing disposable-database check from
`/home/adams/projects/global-torque/i-migration-service`:

```sh
(cd /home/adams/projects/global-torque/i-migration-service && ./make.sh test)
```

Run KYC and escrow integration stacks separately when they share database state.
Do not use a local `replace` directive as evidence that a published module
contract works.

## Rollout and rollback

- Canary one route in one service. Start with KYC manual, then escrow manual
  only after KYC has stable transaction-duration and pool metrics.
- Monitor transaction duration, pool saturation, begin/commit/rollback counts,
  response-buffer rejects, commit failures, and delivery failures.
- Verify selected routes, error responses, existing health, metrics, and Pub/Sub
  endpoints after each deployment. Do not require an idempotency header or
  idempotency record for these routes.
- Roll back by removing Atomic from the explicit route middleware and deploying
  the previous service revision. Do not globally disable it with an environment
  flag. Leave additive shared modules available to the previous revision.
- A delivery failure after commit cannot be rolled back. Log it with the request
  ID and transaction identifier, then use the existing operator process.

## Acceptance criteria

The initial Atomic rollout is ready for review only when all of the following are
true:

1. Echo v5/server v3 Atomic is published and tested separately from the old v2
   contract.
2. `db/v2` publishes context-aware `BeginScope` and savepoint behavior, and
   both selected services consume the published version.
3. Atomic is registered only on the two explicit manual routes.
4. The initial order is recovery/logging and the current access boundary,
   followed by Atomic, followed by the handler. Tests prove actual Echo order.
5. The selected business PostgreSQL writes commit on success and roll back on
   handler error, status `>=400`, cancellation, invalid response, or oversized
   response.
6. The HTTP response is buffered and delivered only after commit. Commit failure
   discards success, and post-commit delivery failure never attempts unsafe
   rollback or a second response.
7. Existing local `Begin` and `orm.WithTx` callers behave as savepoints inside
   the root scope without a broad simultaneous refactor.
8. The selected routes do not require `Idempotency-Key`, do not resolve step 2's
   actor scope, and do not read or write idempotency records.
9. KYC provider, escrow provider, webhook, and Pub/Sub routes do not start
   Atomic scopes, and their acknowledgement/status contracts remain unchanged.
10. Real PostgreSQL shared-module and service route tests pass with no unverified
    local replacement dependency.
11. One-route canary metrics and a route-removal rollback procedure are recorded.
12. No frontend change or alternate service URL is required for this step.

Later coupling with step 2 is not an acceptance criterion for this rollout. It
requires the separate gates in the later coupling section.

## Risks and open decisions

- **Echo v5 response semantics:** `Context.Response()` and response hooks differ
  from Echo v4. The staged response must be tested with the actual v5 API, not
  adapted by embedding a v4 context.
- **Middleware order:** Echo registration order must be proven with an entry and
  exit test. A wrong order can commit before the handler or bypass the current
  access boundary.
- **Direct pool access:** Any selected repository call that bypasses contextual
  `db.DB` methods escapes Atomic. Audit the two selected paths before enabling
  them and change only the smallest required boundary.
- **Nested transaction compatibility:** Existing `Begin` and `orm.WithTx`
  behavior must remain correct as savepoints. Do not release the route until
  real PostgreSQL tests prove nested rollback and root rollback.
- **Response limits and finite streams:** Step 1 must finalize finite-stream
  behavior and explicit operational limits. Flush, hijack, SSE, trailers, and
  unbounded streams remain rejected.
- **External side effects:** Atomic is not a provider transaction. Expanding to
  provider routes requires durable commands, outbox/reconciliation, and a saga
  design. That future redesign is explicitly outside this step.
- **Later idempotency coupling:** If internal clients later send keys, the
  validated actor, non-blocking claim protocol, and exact order must be proven
  before adding Idempotency inside Atomic. That later change is not part of this
  rollout.
