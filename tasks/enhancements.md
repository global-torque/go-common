I loaded the referenced local Go/architecture skill docs, reviewed the repo by package, and ran targeted `go test`/`golangci-lint` checks. No files were changed.

## Flow

- Work in iterations.
- After each implementation iteration, run unit tests and required validation checks. Do not continue until tests are green, unless a failing test is explicitly documented as unrelated, flaky, or blocked with evidence.
- After every implementation iteration, create an independent review subagent using the golang-pro skills.
- The review subagent must inspect the changed code and the surrounding codebase independently.
- The review subagent must produce a written report containing:
  - Bugs or correctness issues
  - Missing requirements
  - Test gaps
  - Security or reliability concerns
  - Maintainability or architecture improvements
  - Any regressions or broken module boundaries
- The main agent must address every actionable critique from the review report.
- After fixes are made, run tests again and create a new independent review subagent.
- Repeat the review/fix/test loop until the review subagent reports no remaining actionable problems or improvements.
- The agent must not stop while there are unresolved actionable problems or improvements.
- If an issue cannot be addressed, the agent must document:
  - The issue
  - Why it cannot be fixed now
  - What evidence supports that decision
  - Whether it is blocked, out of scope, or requires user input

## Final Verification

- Once implementation appears complete, create a new independent verification subagent.
- The verification subagent must read `tasks/enhancements.md`.
- The verification subagent must compare the implemented code against the task requirements.
- The verification subagent must produce a final verification report listing:
  - Fully implemented requirements
  - Partially implemented requirements
  - Missing requirements
  - Incorrect implementations
  - Additional risks or regressions
- The main agent must address every issue found in the final verification report.
- After addressing issues, run tests again and repeat final verification with a new independent subagent.
- Continue until the final verification subagent confirms that `tasks/enhancements.md` has been correctly implemented and there are no remaining actionable issues.
- If the same issue remains unresolved after multiple attempts, the agent must not silently stop. It must escalate by documenting the blocker, the attempted fixes, and the exact remaining problem.

**Build And Module Boundaries**
- **High:** This is a multi-module repo, but modules depend on published sibling versions instead of the local checkout, e.g. [db/go.mod](/home/adams/projects/global-torque/go-common/db/go.mod:10), [server/go.mod](/home/adams/projects/global-torque/go-common/server/go.mod:12), [queue/go.mod](/home/adams/projects/global-torque/go-common/queue/go.mod:12). Add `go.work` for development/CI, or break harder and collapse into one root module if this library is intended to evolve together.
- **High:** CI has lint/tests commented out and only builds/pushes the Docker image: [ci.yaml](/home/adams/projects/global-torque/go-common/.github/workflows/ci.yaml:309), [ci.yaml](/home/adams/projects/global-torque/go-common/.github/workflows/ci.yaml:324), [ci.yaml](/home/adams/projects/global-torque/go-common/.github/workflows/ci.yaml:368).
- **Medium:** `make.sh lint` uses `--fix` and suppresses failures with `|| echo 'not ok'`, which hides broken modules: [make.sh](/home/adams/projects/global-torque/go-common/make.sh:91). `make.sh test` blindly `source`s `.env`: [make.sh](/home/adams/projects/global-torque/go-common/make.sh:109).

**Fatal/Panic APIs**
- **High:** Shared-library constructors kill the process instead of returning errors: [db.New](/home/adams/projects/global-torque/go-common/db/db.go:36), [pclient.New](/home/adams/projects/global-torque/go-common/queue/pclient/client.go:26), [queue.newListener](/home/adams/projects/global-torque/go-common/queue/pubsub.go:77), [server.NewServer](/home/adams/projects/global-torque/go-common/server/http.go:62). Break these APIs to return `(*T, error)` and add explicit `Must...` helpers only for app `main`.
- **High:** `Configurator.Get/New/Print` panic on config/print errors: [configurator.go](/home/adams/projects/global-torque/go-common/configurator/configurator.go:71). Prefer `Get(...)(..., error)` and keep panic behavior out of library code.
- **Medium:** `response.Error` can panic on nil `Err` or non-map `Message`: [error.go](/home/adams/projects/global-torque/go-common/response/error.go:37), [error.go](/home/adams/projects/global-torque/go-common/response/error.go:59). Make message shape typed, immutable, and nil-safe.

**HTTP And Security**
- **High:** `StartServer` returns success before knowing whether `Echo.Start` bound the port; startup failures are only logged in a goroutine: [http.go](/home/adams/projects/global-torque/go-common/server/http.go:204). Use an `http.Server`, startup error channel, and read/write/idle timeouts.
- **High:** CORS allows any origin with credentials: [http.go](/home/adams/projects/global-torque/go-common/server/http.go:92). Use an explicit allowlist and separate method/header values.
- **High:** Auth calls use `http.DefaultClient` without timeout/injection: [auth_auth0.go](/home/adams/projects/global-torque/go-common/server/middleware/auth_auth0.go:154), [auth_auth0.go](/home/adams/projects/global-torque/go-common/server/middleware/auth_auth0.go:162). Inject a client with timeout and reject empty/malformed tokens before network calls.
- **Medium:** `ErrorBadRequestResponse` uses a direct `err.(*echo.HTTPError)` after `errors.As`, which breaks wrapped errors: [error_handler.go](/home/adams/projects/global-torque/go-common/server/error_handler.go:50).

**Logging And Context**
- **High:** `ContextHook` is commented out, so `serviceContext` is missing from logs and logger tests fail: [context.go](/home/adams/projects/global-torque/go-common/logger/context.go:7).
- **High:** `SetLogger` is registered before IP/request-id middleware, so it snapshots empty context fields: [http.go](/home/adams/projects/global-torque/go-common/server/http.go:101), [http.go](/home/adams/projects/global-torque/go-common/server/http.go:129). Register logger after enrichment or build log context lazily.
- **Medium:** Google Cloud error-reporting hook is inverted: production stdout skips `@type`, while console output gets it: [echo_google_cloud.go](/home/adams/projects/global-torque/go-common/logger/echo_google_cloud/echo_google_cloud.go:125).

**Database**
- **High:** `db.New` no longer returns an error, but tests still expect one: [db.go](/home/adams/projects/global-torque/go-common/db/db.go:36), [db_logging_test.go](/home/adams/projects/global-torque/go-common/db/db_logging_test.go:87). This is API drift.
- **High:** `Subscribe` can block forever on `out <- &payload` if the consumer stops while context is cancelled, holding the acquired connection: [db.go](/home/adams/projects/global-torque/go-common/db/db.go:68). Use `select { case out <- payload: case <-ctx.Done(): }`, return `[]byte`, and consider `UNLISTEN`.
- **Medium:** DB config exposes `mysql/sqlite` constants while all implementation is pgx/Postgres: [config.go](/home/adams/projects/global-torque/go-common/db/config.go:215). Remove unsupported types or add real adapters.
- **Medium:** Pool/retry integer conversions are unchecked: [pool.go](/home/adams/projects/global-torque/go-common/db/pool.go:86), [conn.go](/home/adams/projects/global-torque/go-common/db/conn.go:151). Validate config bounds before casting.

**Pub/Sub Queue**
- **High:** `PublishToTopic` logs publish failure inside a goroutine, then returns `nil` error anyway: [publish.go](/home/adams/projects/global-torque/go-common/queue/pclient/publish.go:452), [publish.go](/home/adams/projects/global-torque/go-common/queue/pclient/publish.go:470). Remove the goroutine and return `res.Get(ctx)` errors.
- **High:** `verifyDeliveryAttempt` calls `Ack()` but processing continues and may later `Nack()` or run the callback: [listeners.go](/home/adams/projects/global-torque/go-common/queue/pclient/listeners.go:173), [listeners.go](/home/adams/projects/global-torque/go-common/queue/pclient/listeners.go:259). Return a bool and stop processing after ack/drop.
- **High:** `PubSubListener.Start` uses `context.Background()` and has no stop/close lifecycle for spawned goroutines: [pubsub.go](/home/adams/projects/global-torque/go-common/queue/pubsub.go:100). Make `Start(ctx) error` or an Fx lifecycle hook.
- **Medium:** `queue` still uses deprecated `cloud.google.com/go/pubsub` v1 and deprecated `option.WithCredentialsFile`: [client.go](/home/adams/projects/global-torque/go-common/queue/pclient/client.go:8), [client.go](/home/adams/projects/global-torque/go-common/queue/pclient/client.go:38).

**HTTP/Test Helpers**
- **High:** `httputils` tests do not compile: test expects four return values, `SendRequest` returns three: [httputils_test.go](/home/adams/projects/global-torque/go-common/httputils/httputils_test.go:219), [httputils.go](/home/adams/projects/global-torque/go-common/httputils/httputils.go:151).
- **High:** `CreateRequestWithFiles` drops normal form fields because `strings.Reader` is not an `io.Closer`: [httputils.go](/home/adams/projects/global-torque/go-common/httputils/httputils.go:69), [httputils.go](/home/adams/projects/global-torque/go-common/httputils/httputils.go:87).
- **Medium:** `httputils` test calls real `google.com`, making unit tests network-dependent: [httputils_test.go](/home/adams/projects/global-torque/go-common/httputils/httputils_test.go:207). Use `httptest.Server`.

**Validation And Test Framework**
- **High:** `validator` fails current `go test` vet due dynamic `errors.Errorf(strErr)`: [validate.go](/home/adams/projects/global-torque/go-common/validator/validate.go:110). Use `errors.New(strErr)` or `fmt.Errorf("%s", strErr)`.
- **Medium:** `RunTableTest` panics on fixture setup and applies fixtures once for all scenarios, causing state leakage: [general.go](/home/adams/projects/global-torque/go-common/tests/general.go:69).
- **Medium:** `CompareJSONBody` only supports top-level JSON objects, not arrays/scalars: [general.go](/home/adams/projects/global-torque/go-common/tests/general.go:164).

**Misc And DevOps**
- **Medium:** `SmartRound` lacks validation for negative diffs, `NaN/Inf`, empty/nil values, and uses exact float equality for grouping: [round.go](/home/adams/projects/global-torque/go-common/misc/round/round.go:68).
- **Medium:** `verser` is unsynchronized global mutable state with a typoed public API: [verser.go](/home/adams/projects/global-torque/go-common/verser/verser.go:4), [verser.go](/home/adams/projects/global-torque/go-common/verser/verser.go:11).
- **Medium:** Docker base image build uses `ADD .`, installs latest tools/scripts at build time, and runs as root: [Dockerfile](/home/adams/projects/global-torque/go-common/docker/Dockerfile:75).

**Validation Run**
Passed: `response`, `tests`, `misc/round`, `configurator`, `context/keys`, `server`, `verser`, `docker`.

Failed: `httputils` compile mismatch, `logger` missing `serviceContext`, `validator` vet failure, `db` compile mismatch, `queue` vet failures and a hanging integration test process.