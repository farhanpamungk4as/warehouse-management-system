# Backend verification history — Phases 02–05

This is the README snapshot preserved at the Phase 05 checkpoint. Its phase/status statements are historical; use the root README and final-acceptance.md for final implementation status.

Phase 05 implements all seven required Item and Stock endpoints with PostgreSQL persistence, strict validation, global SKU uniqueness, soft deletion, item filtering/search/pagination, complete category options, atomic receiving and all-location stock lookup. The existing `GET /healthz` remains available. The React frontend is **not implemented yet**. The implemented API contract is [OpenAPI 3.0.3](../backend/docs/openapi.yaml).

The architecture is a domain-oriented modular monolith. Item and Stock each own `domain`, `handler`, `service`, and `repository/postgres` under `backend/internal/modules/`. Location owns only its domain representation and transaction-bound lookup; Item also owns its transaction-bound receive guard. Requests flow handler → service → repository → PostgreSQL. Services own narrow persistence/collaboration interfaces; constructors inject dependencies. `backend/internal/app` composes concrete dependencies and maps domain errors to HTTP; `internal/platform` stays independent of business modules. See the preserved [technical design](technical-design.md).

## Prerequisites and pinned versions

- Docker Desktop/Engine with Docker Compose v2 (verified with Engine 28.1.1 / Compose 2.35.1).
- For host development/testing: Go 1.25.5 and Goose CLI v3.24.3.
- Backend: Gin v1.10.1, pgx/v5 v5.7.6, standard-library slog; `go.mod` and `go.sum` pin dependencies.
- Containers: PostgreSQL 17.11, Go 1.25.5-bookworm, Alpine 3.22.3, all pinned by version and image digest. Runtime and migration containers run as non-root.

## Run with Docker

From the repository root:

```sh
docker compose up --build -d --wait
docker compose ps --all
curl -fsS http://localhost:18080/healthz
```

Foreground mode also works: `docker compose up --build`. No `.env` file is needed. The database must become healthy, then the single Goose job must exit successfully before the backend starts. The Go process never runs migrations itself.

Expected readiness body:

```json
{"success":true,"message":"Service ready","data":{"status":"ready"}}
```

HTTP errors, unknown routes/methods, timeouts, and recovered panics use the standard error envelope. Readiness returns 503 when its database check fails. Requests receive an `X-Request-ID`; structured stdout logs include method, route, status, duration and ID. Request deadlines use cooperative context cancellation, not competing response-writer goroutines.

Only the backend port is published, bound to localhost. PostgreSQL is accessible within Compose, without a host port. Default host API port is **18080** because 8080 was occupied in the development environment; the container listens on 8080. Override with `API_PORT=18081 docker compose up -d --wait` if needed.

Default **local-only demo** credentials: user/database `warehouse`, password `warehouse_local_only`. Do not use these for production. Optional root `.env`/exported overrides: POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB, API_PORT, LOG_LEVEL, DB_MAX_CONNS, STARTUP_TIMEOUT, REQUEST_TIMEOUT, SHUTDOWN_TIMEOUT. Backend/migration use matching PG environment fields so passwords are not interpolated into URLs. Changing credentials on an existing initialized volume also requires updating PostgreSQL itself.

Discover seeded location IDs (never assume numeric IDs):

```sh
docker compose exec db psql -U warehouse -d warehouse -c 'SELECT id, code, zone, type FROM locations ORDER BY code;'
docker compose logs migrate backend
docker compose restart backend
docker compose down
```

Substitute configured user/database if overridden. Ordinary `down` retains the named volume. `down -v` destroys local data; use it only for an explicitly disposable environment.

## Item API

| Method | Endpoint | Success |
|---|---|---|
| POST | `/api/v1/items` | 201 + Item + Location header |
| GET | `/api/v1/items` | 200 + active Items + pagination/category metadata |
| GET | `/api/v1/items/:id` | 200 + active Item |
| PUT | `/api/v1/items/:id` | 200 + updated Item |
| DELETE | `/api/v1/items/:id` | 200 + id/deleted_at |

POST and PUT require `Content-Type: application/json` and all four editable fields. They trim surrounding ASCII spaces; reject blank/Unicode-whitespace-only strings, ASCII controls, unknown or duplicate fields, server IDs/timestamps, null/wrong types, malformed/trailing JSON and bodies over 64 KiB. Limits are 64/200/64/32 Unicode characters for SKU/name/category/unit. SKU is case-sensitive and never automatically uppercased. PUT replaces all four fields while preserving id/created_at.

```sh
curl -i http://localhost:18080/api/v1/items \
  -H 'Content-Type: application/json' \
  -d '{"sku":"WH-001","name":"Storage Box","category":"Packaging","unit":"pcs"}'
curl 'http://localhost:18080/api/v1/items?page=1&limit=10&category=Packaging&q=box'
curl http://localhost:18080/api/v1/items/1
curl -X PUT http://localhost:18080/api/v1/items/1 \
  -H 'Content-Type: application/json' \
  -d '{"sku":"WH-001","name":"Large Storage Box","category":"Packaging","unit":"pcs"}'
curl -X DELETE http://localhost:18080/api/v1/items/1
curl -i http://localhost:18080/api/v1/items/1  # 404 after deletion
```

Replace example ID 1 with the actual POST response ID. Repeating the example SKU may return 409, including after deletion. Delete sets deleted_at atomically and retains the physical item and all stock; subsequent detail/update/delete returns 404. Global `items_sku_key` is authoritative for create/update conflicts, including concurrent requests. Renaming an active SKU releases its former value.

List defaults to page 1, limit 10 (allowed 1–100), ordered by id ASC. Optional category is exact and case-sensitive; optional q searches SKU/name case-insensitively across the entire active dataset. `%`, `_`, and backslash are literal search characters; URL-encode them. Filters combine with AND. Unknown/duplicate parameters, invalid integers and offsets above 2,147,483,647 return 400.

```json
{"success":true,"message":"Items retrieved successfully","data":[],"meta":{"page":1,"limit":10,"total":0,"categories":[]}}
```

Rows, filtered total and sorted distinct active categories share one read-only REPEATABLE READ transaction. Category options ignore the current filters/page. Empty/out-of-range pages return arrays and accurate metadata. Transactions use bounded rollback cleanup even after request cancellation.

Failures use `{success:false,message,errors:[{field,reason}]}`. Validation uses 400, missing/deleted items 404, duplicate SKU 409, oversized body 413, unsupported write media 415, unexpected failures 500 and known database/deadline failures 503. Raw SQL details are never exposed. The [OpenAPI contract](../backend/docs/openapi.yaml) documents request/response schemas and examples.

## Stock API

| Method | Endpoint | Success |
|---|---|---|
| POST | `/api/v1/stock/receive` | 200 + committed resulting Stock |
| GET | `/api/v1/stock/:item_id` | 200 + all actual location balances |

Receive requires `application/json` with exactly three non-null integer fields: `item_id`, `location_id`, and `qty`, each within 1–2,147,483,647. Rejects missing fields, unknown/duplicate or incorrectly cased keys, numeric strings, fractional/exponent notation, nulls, overflow, malformed/trailing JSON and bodies above 64 KiB. Handler validation precedes service calls; service independently enforces IDs/quantity. Neither Stock endpoint accepts query parameters.

Create an active Item first and discover seeded Location IDs (the discovery query also appears under Docker setup):

```sh
docker compose exec db psql -U warehouse -d warehouse \
  -c 'SELECT id, code, zone, type FROM locations ORDER BY id;'

# Replace both placeholders with the actual POST Item response and discovery result.
ITEM_ID='<created active item ID>'
LOCATION_ID='<existing seeded location ID>'
curl -X POST http://localhost:18080/api/v1/stock/receive \
  -H 'Content-Type: application/json' \
  -d "{\"item_id\":${ITEM_ID},\"location_id\":${LOCATION_ID},\"qty\":5}"
curl -X POST http://localhost:18080/api/v1/stock/receive \
  -H 'Content-Type: application/json' \
  -d "{\"item_id\":${ITEM_ID},\"location_id\":${LOCATION_ID},\"qty\":3}"
curl "http://localhost:18080/api/v1/stock/${ITEM_ID}"
```

The second receive returns the resulting balance 8. IDs/times below are illustrative; the database generates runtime values:

```json
{"success":true,"message":"Stock received successfully","data":{"id":3,"item_id":1,"location_id":2,"qty":8,"updated_at":"2026-10-15T08:00:00Z"}}
```

Lookup includes every existing stock row in location_id order, including existing zero balances, with Location information. It uses one active-item LEFT JOIN stocks LEFT JOIN locations statement; it never invents zero balances for unstocked locations. An active item with no stock returns `{"item_id":1,"stocks":[]}` inside data. A stocked response is:

```json
{"success":true,"message":"Stock retrieved successfully","data":{"item_id":1,"stocks":[{"id":3,"item_id":1,"location_id":2,"qty":8,"updated_at":"2026-10-15T08:00:00Z","location":{"id":2,"code":"RACK-B1","zone":"B","type":"rack"}}]}}
```

Each receive uses one short READ COMMITTED transaction. Item's active-row guard locks `FOR SHARE`, Location's existence lookup locks `FOR KEY SHARE`, then Stock increments using `INSERT ... ON CONFLICT (item_id, location_id) DO UPDATE SET qty = stocks.qty + EXCLUDED.qty`, guarded by `stocks.qty <= 2147483647 - EXCLUDED.qty`. The pair constraint remains authoritative; no application read-modify-write or global mutex is used. App injects the Item/Location factories against the runner's exact pgx.Tx; neither guard acquires another connection or owns transaction lifetime.

Lock order is Item → Location → Stock. If receive locks first, DELETE waits for it to finish. If DELETE wins, receive rechecks active status and returns 404. Deletion retains all stock while hiding it from public lookup; receiving never revives a deleted Item. Missing Item takes precedence over missing Location. Validation returns 400, verified missing references 404, cumulative capacity conflict 409, body/media errors 413/415, and safe technical failures 500/503.

Before COMMIT, known validation/reference/SQL/cancellation failures roll back without changing balances. Deferred cleanup uses an independent bounded context, including when the request is already canceled. Stock service receives only narrow capabilities, never pgx transaction types. A transaction session cannot perform queries after its owning transaction closes.

After an acknowledged COMMIT, exactly one `stock.receive.committed` mutation log includes `event`, request_id, item_id, location_id, stock_id, received_qty, qty_before, qty_after and a UTC timestamp. Rolled-back operations emit no committed mutation log. Logs are best-effort console output, not a durable ledger: a crash after commit and before logging can leave an audit gap.

**Receiving is not idempotent.** Two identical successful POST requests increment twice. There is no automatic retry or idempotency-key infrastructure. A timeout or lost COMMIT acknowledgement cannot prove that stock was unchanged. Uncertain acknowledgement returns HTTP 500 with `message: "Stock receive outcome unknown"`, logs `stock.receive.commit_unknown`, and tells the caller to reconcile before retrying. A failed response must not be treated as permission to resend automatically.

## Host development

Provide a PostgreSQL database, install the pinned Goose CLI, and apply migrations before starting the server. `backend/.env.example` documents configuration; it is not loaded implicitly.

```sh
go install -tags='no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb' github.com/pressly/goose/v3/cmd/goose@v3.24.3
cd backend
export DATABASE_URL='postgres://warehouse:warehouse_local_only@localhost:5432/warehouse?sslmode=disable'
goose -dir migrations postgres "$DATABASE_URL" up
GIN_MODE=release go run ./cmd/api
```

Use an available host port via HTTP_ADDR if 8080 is occupied. A database inside the default Compose stack has no host port; host mode requires your own reachable PostgreSQL instance or a separate disposable container as below. Missing/invalid required configuration exits safely without printing credentials. Startup connection/readiness is bounded (default 10s), request deadline defaults to 5s, and graceful shutdown defaults to 10s.

## Tests

From `backend`:

```sh
go test ./...
go test -race ./...
go vet ./...
go mod verify
```

Without TEST_DATABASE_URL, real database tests explicitly **SKIP**. To execute them against isolated PostgreSQL 17 from the repository root:

```sh
docker run --detach --rm --name warehouse-phase05-test-db \
  -e POSTGRES_USER=warehouse_test -e POSTGRES_PASSWORD=warehouse_test \
  -e POSTGRES_DB=postgres -p 127.0.0.1:55439:5432 postgres:17.11
docker exec warehouse-phase05-test-db pg_isready -U warehouse_test -d postgres
cd backend
export TEST_DATABASE_URL='postgres://warehouse_test:warehouse_test@127.0.0.1:55439/postgres?sslmode=disable'
export GOOSE_BIN="$(go env GOPATH)/bin/goose"
go test -count=1 -v ./...
go test -race -count=1 ./...
go vet ./...
go mod verify
docker stop warehouse-phase05-test-db
```

Wait until `pg_isready` succeeds before tests. GOOSE_BIN must point to the installed pinned CLI. The suite creates a random `warehouse_test_*` database, invokes actual Goose Up/Down, checks PostgreSQL errors and removes only its newly created database. The configured URL is an admin connection requiring CREATE DATABASE privilege. Use an isolated test server, never operational credentials. The server lifecycle test starts a real HTTP listener against that database.

Coverage includes Phase 04 T-01–T-10: create/Location, validation before service calls, create/update conflicts, simultaneous duplicate requests (exactly one 201 and one 409), full PUT preserving created_at, retained physical item/stock after soft delete, 25-item pagination/filtering, global literal search, category options, empty/out-of-range results, and offset boundaries. Item tests use real HTTP servers and isolated randomly named databases. Technical failures are verified through actual HTTP 500/503 envelopes. The foundation suite still covers migrations/seeds, named constraints, safe rollback refusal/version consistency, down/up, readiness/errors/panics/body limits, request IDs/logging, deadlines and graceful shutdown. The deferred Stock portion of T-07 is now complete: deleted Item stock lookup and receive return 404 while physical stock stays intact.

Phase 05 coverage T-11–T-24 adds strict transport/service validation; new/existing/identical receives; missing/deleted references; actual multi-location and empty lookup; deterministic concurrency against existing and absent pairs; many concurrent receipts; capacity competitors; both receive/delete lock orderings; Location lock retention; rollback after a real write; cancellation cleanup; and single-connection app wiring/session lifetime. T-21 continues to use the foundation suite's real named CHECK/UNIQUE/FK assertions.

Concurrency tests hold explicit row locks and wait for PostgreSQL to report distinct blocked backend PIDs before releasing them. They inspect final database balances and mutation logs. The COMMIT uncertainty test uses a test-only TCP proxy against its disposable database: PostgreSQL commits, but the proxy drops its actual acknowledgement. It verifies retained stock, an explicit unknown-outcome HTTP 500, one uncertainty log, no committed-success log and no automatic retry. No fault injection targets the retained Compose database.

Rollback is intentionally opt-in: Goose's connection URL must contain `options=-c%20warehouse.allow_destructive_rollback%3Don`. Use this **only** on a database known to be disposable and empty before seeding. The pgx-backed CLI does not use PGOPTIONS for this opt-in. Seed Down rejects referenced/altered demo locations; schema Down rejects any remaining rows. A failure leaves Goose version/data unchanged. For populated/shared databases use forward corrections. No auto rollback runs at startup.

Optional independent OpenAPI validation (from the repository root):

```sh
python3 -m venv /tmp/warehouse-openapi-validator
/tmp/warehouse-openapi-validator/bin/pip install openapi-spec-validator
/tmp/warehouse-openapi-validator/bin/openapi-spec-validator backend/docs/openapi.yaml
```

## Implemented decisions and trade-offs

1. **Database-enforced integrity:** global SKU uniqueness survives soft deletion, and a unique item/location pair plus nonnegative stock check protects receiving. Item handler and service independently enforce normalized domain validation; these constraints remain the concurrency-safe backstop.
2. **Single migration owner:** health-gated one-shot Goose job avoids backend/migration startup races and duplicate migration paths. Migrations failing prevents backend startup.
3. **Technical platform with explicit app wiring:** shared helpers never import business modules; the app owns error mapping and process resources. Item is wired explicitly; Stock receives use app-injected Item/Location guards on the same transaction without restructuring Item CRUD.
4. **Explicit receive consistency:** atomic guarded upsert plus transaction-scoped reference locks avoids lost updates, duplicate balances, capacity overflow and receive/delete races. Narrow app-injected factories keep ownership intact at the cost of a little explicit wiring.
5. **No implicit replay:** receiving has no idempotency key; uncertain commit outcomes remain explicit and callers must reconcile. This keeps the baseline small but does not provide retry-safe receipts.
6. **Conservative rollback:** explicit disposable-DB opt-in and checks prioritize data preservation over convenient destructive reset.

## Phase 05 verification and next phases

Executed on **15 October 2026 (Asia/Jakarta)**:

| Check | Actual result |
|---|---|
| Baseline `go test -count=1 ./...` with real PostgreSQL before Stock changes | PASS |
| Full `go test -count=1 ./...` with TEST_DATABASE_URL/GOOSE_BIN | PASS, including Item regressions and Stock transaction/concurrency tests |
| Full `go test -race -count=1 ./...` with real PostgreSQL | PASS |
| `go vet ./...`; `go mod verify` | PASS; all modules verified |
| `docker compose config --quiet`; `docker compose up --build -d` | PASS on the retained application volume |
| Compose health and logs | Database/backend healthy; migration exited 0 at version 2; no unexpected failures in smoke logs |
| Stock smoke on `127.0.0.1:18080` | PASS: create Item → discover actual Location IDs → receive 5 → receive 3 → qty 8 → receive 4 at another location → lookup both → delete → Item/Stock lookup and receive 404 → physical stock unchanged |
| Developer data persistence | PASS: before/after digests of preexisting items, stocks and locations match |
| OpenAPI 3.0.3 and examples | PASS: exactly seven business operations and all 48 request/response examples schema-valid |
| Postman lint in Phase 05 | NOT RUN: Phase 04 established the local Keychain failure below; independent OpenAPI validation used |

Real PostgreSQL evidence:

| Scenario | Observed result |
|---|---|
| Existing balance 10, two independent +5 receipts | Responses 15/20; final 20; one row |
| Absent pair, two independent +5 receipts | Responses 5/10; final 10; one row |
| Six independent +5 receipts | All six succeed; final 30; one row |
| Balance 2147483646, two +1 competitors | One 200, one 409; final 2147483647; one row |
| Receive first / delete first | DELETE waits / receive rechecks and returns 404 |
| COMMIT reply deliberately dropped after actual server commit | Stock retains +5; HTTP 500 explicitly unknown; one unknown log; no retry or success log |

A source-only baseline commit `bbd89d8` was created after a secret scan. Phase 05 changes remain uncommitted for review; no push, publication or submission occurred. The approved technical design, SQL schema/migrations, Go dependencies and Compose configuration are unchanged. No product-scope deviation was required. Initial checks found and corrected unkeyed test literals and a YAML insertion error before the passing verification runs.

The separate disposable test server owns all generated integration databases; suites drop only those databases. The retained Compose smoke fixture is soft-deleted (SKU `phase05-smoke-bcbc1f912704`, Item ID 8), with its two stock rows at qty 8 and 4 preserved. Prior developer rows were not reset or modified.

Final resumed verification restarted Docker Desktop and ran `docker compose up -d --wait`: database/backend healthy, migration exited 0. Read-only HTTP checks and the retained smoke fixture confirmed that soft-deleted Item/Stock views still return 404 and physical balances 8/4 survive restart. The disposable Phase 05 test container is no longer present. A local `go test ./...`, `go vet ./...` and `go mod verify` rerun passed; PostgreSQL integration tests were skipped in that final local rerun because TEST_DATABASE_URL was unset. The earlier full PostgreSQL and race-detector results above remain the database-backed evidence.

All required Phase 05 cases are covered. No stock transfers, Location CRUD, ledger, idempotency keys, frontend or other optional features were added. Phase 06 can focus on final API contract/regression review; all seven required operations are already documented. Phases 07–08 add frontend/integration/submission. No frontend startup command is available yet.

## Phase 04 verification history

Executed on **15 October 2026 (Asia/Jakarta)**:

| Check | Actual result |
|---|---|
| `go test -count=1 ./...` with TEST_DATABASE_URL/GOOSE_BIN | PASS, including real PostgreSQL 17.11 foundation and Item acceptance tests |
| `go test -race -count=1 ./...` with real PostgreSQL | PASS |
| `go vet ./...` | PASS |
| `go mod verify` | PASS: all modules verified |
| `docker compose config --quiet` | PASS |
| `docker compose up --build -d`; `docker compose ps --all` | PASS: database/backend healthy; migration exited 0 at schema version 2 |
| HTTP smoke on `127.0.0.1:18080` | PASS: create → list → detail → update → delete → detail 404; duplicate 409, repeated delete/update 404, deleted SKU reuse 409, pagination and invalid query 400 |
| OpenAPI schema and examples | PASS using `openapi-spec-validator` and `openapi-schema-validator`: 3.0.3, exactly five Item operations, all 31 shipped examples checked |
| Postman CLI local spec lint | Could not run: macOS Keychain `SecItemCopyMatching failed -50` (exit 139); independent OpenAPI validation used instead |
| Backend/migration logs | No unexpected application failures during Compose smoke verification |

One early test assertion compared a PostgreSQL local-time representation with a UTC value using struct equality; corrected to `time.Equal` and reran successfully. Pagination validates the offset before multiplication, including the valid boundary `page=2147483648&limit=1`; larger offsets fail safely.

The application named volume was retained. Integration suites create and drop only their own random databases on a separate disposable server. The two Compose smoke fixtures remain soft-deleted under unique `phase04-smoke-*` SKUs, demonstrating retained SKU reservation. No developer records were reset. No schema changes or Go dependency changes were needed.

The existing Phase 02–03 verification recorded on 14 October 2026 also covered real database outage/recovery, populated-volume restarts and migration-failure startup gating. This phase reran the foundation suite and existing-volume Compose startup; it did not repeat those separate outage/failure-injection container scenarios.

These historical Phase 04 results covered Item behavior. Phase 05 status is recorded above. Offset pages can shift between requests and concurrent Item edits remain last committed write wins, as approved.

Phase 04 initialized the local Git repository without committing or publishing; Phase 05 subsequently created the baseline commit recorded above. The original assessment PDF is supplied separately and is not committed.

## AI usage disclosure

OpenAI Codex assisted with the Phase 01 design/refinement, Phase 02–03 foundation, Phase 04 Item implementation, and Phase 05 Stock receiving/lookup, transaction-bound Item/Location guards, app wiring, validation/error/logging behavior, transaction/concurrency/fault-injection tests, OpenAPI and README. Codex used the Postman API Engineer, API Documentation and API Testing skill guidance, executed the reported Go/PostgreSQL/Docker/HTTP checks, and corrected issues found during verification. Postman lint failed in the local Keychain integration during Phase 04, so Phase 05 reused independent local Python validators in a temporary virtual environment. Codex verified actual PostgreSQL concurrency, both delete/receive lock orders, rollback/cancellation, and loss of an actual COMMIT acknowledgement using a proxy limited to disposable test data. No manually written code contributions or independent human review are claimed. The candidate must review and understand this work and update this disclosure honestly as subsequent work is completed.
