# Warehouse Management System

## Deklarasi penggunaan AI


- **Kontribusi saya:**  Menetapkan arsitektur, mengarahkan kebutuhan, Revisi structure, serta menjalankan setup Docker, PostgreSQL, Redis, environment, DNS dan deployment server.
- **Bantuan AI:** implementasi backend/frontend, API, migration, integrasi Redis, script dummy, pengujian otomatis dan dokumentasi.


## Overview

A Go/PostgreSQL warehouse application with a React/shadcn dashboard, item and location CRUD, inventory, stock receiving and atomic transfers. Redis provides an optional location cache.

## Run the complete application

Requires Docker Engine/Desktop and Compose v2. From the repository root:

```sh
docker compose up --build
```

- **Frontend:** http://localhost:3002/dashboard
- **Backend:** http://localhost:18080/api/v1
- **Readiness:** http://localhost:18080/healthz
- **API contract:** [backend/docs/openapi.yaml](backend/docs/openapi.yaml), OpenAPI 3.0.3

No `.env` or manual migration step is needed for the local demo. Redis starts as an optional cache dependency; its outage does not affect readiness or core operations. PostgreSQL health gates the one-shot Goose migration; successful migrations gate the backend; backend health gates the frontend. Nginx serves the React production bundle, proxies `/api/` without rewriting its path, and falls back to index.html for all direct application route reloads. The browser uses same-origin API requests.

```sh
docker compose up --build -d --wait
docker compose ps --all
docker compose logs migrate backend frontend
docker compose down
```

Normal shutdown retains PostgreSQL's named volume. **Do not use `down -v` on retained developer data.** Frontend port 3002 was selected because another local container occupies 3000; override it with `FRONTEND_PORT=3000 docker compose up --build`. `API_PORT` overrides backend port 18080. Both are bound to localhost.

## Stack and architecture

Go 1.25.5, Gin 1.10.1, pgx/v5 5.7.6, PostgreSQL 17.11 and Goose 3.24.3; React 19.3.1, TypeScript 5.9.3, React Router **6.30.6**, TanStack Query 5.104.3, Vite 7.3.7 and Tailwind CSS 4.3.3, with shadcn/ui, Lucide, Sonner and Redis 7.4/go-redis v9.7.3. Dependency resolutions are locked in go.mod/go.sum and package-lock.json; container bases use immutable image digests. Go and Nginx runtime containers use non-root users.

The backend is a domain-oriented modular monolith: handler → service → repository → PostgreSQL. Item, Stock, Location and Inventory own cohesive layers. Location owns read-through caching and transaction-bound location guards; Inventory supplies dashboard aggregates and paginated joined reads. Services declare narrow interfaces. `internal/app` explicitly composes dependencies and maps domain errors to HTTP; `internal/platform` contains technical infrastructure without business imports. Frontend features live in dashboard/items/inventory/receiving/transfers/locations; shared API transport, typed warehouse contracts and shadcn primitives remain separate. A global Error Boundary handles rendering failures; Query errors stay in normal page error states.

```text
backend/
  cmd/api/                 # process entrypoint
  internal/app/            # composition and error mapping
  internal/modules/        # item, stock, location, inventory
  internal/platform/       # config, pool, Redis transport, HTTP, middleware
  migrations/              # versioned Goose SQL and Location seeds
  docs/openapi.yaml
  tests/integration/       # real PostgreSQL/API/concurrency tests
  Dockerfile, .env.example
frontend/
  src/app/                 # routes and QueryClient/toast providers
  src/features/            # dashboard, items, inventory, receiving, transfers, locations
  src/components/ui/       # reusable feedback and controls
  src/hooks/               # debounce
  src/pages/               # list, create, edit
  src/services/, types/    # shared API client and envelopes
  src/styles/
  tests/                   # real Chrome acceptance
  Dockerfile, nginx.conf, .env.example
  package.json, package-lock.json
deploy/                    # unpublished HTTPS/authenticated deployment configuration
docs/                      # designs, audit, deployment guide, actual screenshots
docker-compose.yml
```

## Local development

For the frontend, use Node 22.12+ (verified on 22.18.0) or a compatible newer LTS release:

```sh
cd frontend
npm install
npm run dev
```

Open http://localhost:5173/items. Vite proxies `/api` to the existing backend at `http://127.0.0.1:18080`. Optional `frontend/.env` values are documented in [frontend/.env.example](frontend/.env.example): `VITE_API_BASE_URL` defaults to `/api/v1`; `API_PROXY_TARGET` affects only the development proxy. Never put credentials in `VITE_*`. Docker works with its built-in defaults.

For host backend development, provide an accessible PostgreSQL database and install the pinned Goose CLI. The default Compose PostgreSQL port is internal; use a separate local PostgreSQL instance when running the backend on the host.

```sh
go install -tags='no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb' github.com/pressly/goose/v3/cmd/goose@v3.24.3
cd backend
export DATABASE_URL='postgres://warehouse:warehouse_local_only@localhost:5432/warehouse?sslmode=disable'
goose -dir migrations postgres "$DATABASE_URL" up
HTTP_ADDR=:8080 GIN_MODE=release go run ./cmd/api
```

If the host backend uses 8080, set `API_PROXY_TARGET=http://127.0.0.1:8080` for Vite. Environment templates are not implicitly loaded by Go. Backend defaults/configuration: `HTTP_ADDR=:8080`, `LOG_LEVEL=info`, `DB_MAX_CONNS=10`, startup/request/shutdown timeouts 10s/5s/10s; `DATABASE_URL` is required in host mode. Optional `REDIS_URL` defaults to empty (disabled); `REDIS_TTL` defaults to 30s and accepts a positive duration up to 1m.

Compose accepts `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, `API_PORT`, `FRONTEND_PORT` `REDIS_URL`, `REDIS_TTL` and the timeout/log/pool overrides. Compose defaults REDIS_URL to `redis://redis:6379/0`; to disable the cache use `REDIS_URL= docker compose up --build` (the empty variable is preserved). Default credentials `warehouse / warehouse_local_only` are explicitly local demo values, supplied through environment fields. Changing credentials on an initialized volume requires updating PostgreSQL too.

## Populate demo inventory

With the application running, execute from the repository root (Python 3, no pip dependencies):

```sh
python3 scripts/seed_demo.py
```

For the frontend proxy on this server, this equivalent URL also works:

```sh
python3 scripts/seed_demo.py --base-url http://127.0.0.1:3003/api/v1
```

The script uses the real API to add ten `DEMO-WMS-001`–`010` items with five categories and 900 total units. It discovers location IDs from GET /locations and distributes receipts across available locations; if none exist, it creates DEMO-RACK-A1/B1 through POST /locations. Refresh the dashboard after completion. These are actual PostgreSQL records, so existing data contributes to the displayed totals.

Run one copy at a time. Rerunning preserves existing matching demo items and any stock records (including zero balances); it does not replenish consumed/transferred demo inventory. Conflicting SKU contents or API failures stop the script; earlier successful writes remain saved. Writes are not automatically retried.

Location API examples:

```sh
curl -fsS http://127.0.0.1:18080/api/v1/locations
curl -fsS -X POST http://127.0.0.1:18080/api/v1/locations \
  -H 'Content-Type: application/json' \
  -d '{"code":"RACK-C1","zone":"C","type":"rack"}'
# Use the actual ID returned above for detail/update/delete.
curl -fsS http://127.0.0.1:18080/api/v1/locations/3
curl -fsS -X PUT http://127.0.0.1:18080/api/v1/locations/3 \
  -H 'Content-Type: application/json' \
  -d '{"code":"RACK-C2","zone":"C","type":"bin"}'
curl -fsS -X DELETE http://127.0.0.1:18080/api/v1/locations/3
```

## API and business rules

| Method | Endpoint | Success |
|---|---|---|
| POST | `/api/v1/items` | 201, Item and Location header |
| GET | `/api/v1/items` | 200, active items and pagination/category metadata |
| GET | `/api/v1/items/:id` | 200, active Item |
| PUT | `/api/v1/items/:id` | 200, full replacement of editable fields |
| DELETE | `/api/v1/items/:id` | 200, id and deleted_at |
| POST | `/api/v1/stock/receive` | 200, committed resulting balance |
| GET | `/api/v1/stock/:item_id` | 200, actual balances with Location details |
| GET | `/api/v1/dashboard/summary` | Bonus: active item/quantity/location/occupied-pair counts and actual location distribution |
| GET | `/api/v1/inventory/overview` | Bonus: joined stock rows with q/category/location_id/page/limit filters and metadata |
| GET | `/api/v1/locations` | Bonus: actual locations, ordered by code/id |
| POST | `/api/v1/locations` | 201, created location and Location header |
| GET | `/api/v1/locations/:id` | 200, actual location |
| PUT | `/api/v1/locations/:id` | 200, full replacement of code/zone/type |
| DELETE | `/api/v1/locations/:id` | 200, deleted unused location; 409 if stock records exist |
| POST | `/api/v1/stock/transfer` | Bonus: committed source/destination balances |

Location CRUD uses `GET /api/v1/locations`, `POST /api/v1/locations`, and `GET`/`PUT`/`DELETE /api/v1/locations/:id`. Writes require exactly `code`, `zone`, `type` strings (64/64/32 Unicode characters); codes are unique and case-sensitive. PUT preserves the location ID and stock references. DELETE removes only an unused location; any stock record, including a zero balance or soft-deleted item's balance, returns 409. Existing foreign keys serialize deletion against receipts and transfers, so no schema migration is required. The Locations card contains shadcn create/edit dialogs and delete confirmation. Receive/transfer/inventory dropdowns load the list endpoint and refresh after location mutations.

```sh
curl -i http://localhost:18080/api/v1/items \
  -H 'Content-Type: application/json' \
  -d '{"sku":"WH-001","name":"Storage Box","category":"Packaging","unit":"pcs"}'
curl 'http://localhost:18080/api/v1/items?page=1&limit=10&category=Packaging&q=box'

# Discover actual Location IDs; do not assume they are 1/2.
curl http://localhost:18080/api/v1/locations

# Replace IDs with the created Item and discovered Location.
curl http://localhost:18080/api/v1/items/1
curl -X PUT http://localhost:18080/api/v1/items/1 \
  -H 'Content-Type: application/json' \
  -d '{"sku":"WH-001","name":"Large Storage Box","category":"Packaging","unit":"pcs"}'
curl -X POST http://localhost:18080/api/v1/stock/receive \
  -H 'Content-Type: application/json' -d '{"item_id":1,"location_id":2,"qty":5}'
curl http://localhost:18080/api/v1/stock/1
curl -X DELETE http://localhost:18080/api/v1/items/1
```

Item fields trim surrounding ASCII spaces, reject blank/Unicode-whitespace-only strings and ASCII controls, and enforce 64/200/64/32 Unicode-character limits for SKU/name/category/unit. Client and server validate independently. SKU is case-sensitive; the global named unique constraint is authoritative, including deleted items. PUT requires all four fields and preserves id/created_at. DELETE sets deleted_at atomically, retains physical Item/Stock rows and reserves the SKU; subsequent active detail/update/delete/stock lookup/receive returns 404. There is no Item hard-delete or undelete UI/API.

List defaults to page 1/limit 10, accepts limit 1–100, orders by id ASC, and rejects invalid/unknown/duplicate parameters or offsets above 2,147,483,647. Category is an exact case-sensitive match; q is a case-insensitive **literal** SKU/name substring across the entire dataset. `%`, `_` and backslash are escaped for SQL LIKE. Filtered total, rows and complete sorted active category options share one read-only REPEATABLE READ snapshot. Empty/out-of-range pages return arrays and correct metadata; the UI recovers to the last valid page. Separate offset-page requests can shift as data changes.

Responses use `{success:true,message,data}` with meta on item lists, or `{success:false,message,errors:[{field,reason}]}`. Inventory lists also include meta. Statuses: validation 400, missing/deleted references 404, duplicates/insufficient stock/capacity 409, body above 64 KiB 413, non-JSON writes 415, safe technical failures 500/503. Unknown methods/routes and panics use the same centralized envelope. Responses include X-Request-ID; SQL/credentials are not exposed.

Stock receives accept integer IDs/qty in 1–2,147,483,647. One READ COMMITTED transaction locks active Item `FOR SHARE` → Location `FOR KEY SHARE` → guarded atomic upsert. All guards/writes use the exact same pgx.Tx. `qty = stocks.qty + EXCLUDED.qty` with the capacity predicate prevents lost increments, duplicate pairs and overflow. Receive-first makes DELETE wait; delete-first makes receive recheck active eligibility and return 404. Lookup uses one active-item LEFT JOIN and returns every actual location balance, including stored zero balances, or `stocks: []` for an unstocked active item.

After confirmed COMMIT, one `stock.receive.committed` log records request ID, item/location/stock IDs, received_qty, qty_before/after and UTC time. Known precommit failures roll back, with bounded independent cleanup after cancellation. A lost COMMIT acknowledgement returns explicit `Stock receive outcome unknown` and emits `stock.receive.commit_unknown`. **Receiving is not idempotent: identical successful requests add twice. Do not retry automatically after uncertain results; reconcile first.** Console logs are best-effort, not a durable ledger.

## Enterprise features and consistency

Routes: `/dashboard`, `/items`, `/items/new`, `/items/:id`, `/items/:id/edit`, `/inventory`, `/receiving`, `/transfers`, `/locations`. `/` redirects to dashboard. Item detail sums integer balances with BigInt, shows real location/timestamp data, and provides edit/receive/transfer actions. Delete uses a confirmation dialog. Inventory reads active items in one paginated join with a coherent count/category snapshot, including stored zero balances. Dashboard occupied pairs count only `qty > 0`; available quantity excludes deleted items and returns zero for an empty warehouse. No historical movement or growth data is fabricated.

Transfer body: `{"item_id":1,"from_location_id":1,"to_location_id":2,"qty":5}` (replace with discovered IDs). Service and handler validate positive bounded integers and distinct locations. One READ COMMITTED transaction locks active Item FOR SHARE, then **both Location rows FOR UPDATE in ascending ID order**, before touching stock. Conditional source decrement enforces sufficient quantity; the existing bounded atomic upsert adds to destination. Both changes commit together or roll back. Ordered exclusive Location locks serialize transfers sharing a location, even for different items; this conservative contention cost is appropriate for the assessment. Receive's KEY SHARE occurs before stock writes, preserving a compatible order; soft delete conflicts with the Item share lock.

Only acknowledged COMMIT produces `stock.transfer.committed`; lost acknowledgement produces `stock.transfer.commit_unknown` and an explicit unknown-outcome response. **Receive and transfer are non-idempotent and have no automatic mutation retries.** On network/5xx uncertainty, the UI disables resubmission until balances are refreshed; entered quantity is cleared before manual retry. This is reconciliation guidance, not exactly-once delivery. Browser previews may be stale; the database is authoritative.

Redis caches **only GET Location list results**, under `warehouse:v1:locations`, default TTL 30s. Get/Set/Delete use bounded 100ms contexts, client retries are disabled, and misses, malformed values, timeout or outage fall back to PostgreSQL. Successful location writes invalidate the key; a local read/write lock prevents an earlier list request from refilling stale data after invalidation. If invalidation fails, the write stays successful and this backend bypasses the old key for its TTL. This is a single-backend deployment; other backend replicas can retain a stale read-through snapshot until TTL. Direct SQL changes also wait for TTL. Stock quantities, dashboard totals and transfers are never cached.

## Tests and verification

```sh
cd frontend
npm run typecheck
npm test
npm run build
```

Backend commands (from `backend`): `go test ./...`, `go test -race ./...`, `go vet ./...`, `go mod verify`. Without TEST_DATABASE_URL, database tests explicitly skip. To execute them on isolated PostgreSQL:

```sh
docker run --detach --rm --name warehouse-final-test-db \
  -e POSTGRES_USER=warehouse_test -e POSTGRES_PASSWORD=warehouse_test \
  -e POSTGRES_DB=postgres -p 127.0.0.1:55439:5432 postgres:17.11
docker exec warehouse-final-test-db pg_isready -U warehouse_test -d postgres
cd backend
export TEST_DATABASE_URL='postgres://warehouse_test:warehouse_test@127.0.0.1:55439/postgres?sslmode=disable'
export GOOSE_BIN="$(go env GOPATH)/bin/goose"
# Start a dedicated Redis instance for cache integration tests.
docker run --detach --rm --name warehouse-test-redis -p 127.0.0.1:56379:6379 redis:7.4-alpine
export TEST_REDIS_URL='redis://127.0.0.1:56379/0'
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go mod verify
docker stop warehouse-final-test-db warehouse-test-redis
```

Wait for readiness before testing. Integration tests create/drop only their own random databases; TEST_DATABASE_URL requires CREATE DATABASE privilege and should target a disposable server. Goose Down is restricted to disposable empty databases with explicit rollback opt-in; use forward migrations for retained data. See [backend verification history](docs/backend-verification-history.md) for detailed rollback/test behavior.

Real Chrome acceptance requires Google Chrome installed and a **fresh disposable** Compose project. It creates fixtures and intentionally refuses a nonempty database:

```sh
API_PORT=18081 FRONTEND_PORT=3001 docker compose -p warehouse-final-acceptance up --build -d --wait
cd frontend
E2E_BASE_URL=http://127.0.0.1:3001 npm run test:e2e
cd ..
API_PORT=18081 FRONTEND_PORT=3001 docker compose -p warehouse-final-acceptance down -v
```

The `down -v` above is only for the named disposable acceptance project. Tests must not target retained data. Verification includes 18 frontend unit tests, 28 Chrome flows, PostgreSQL transaction/concurrency tests and Redis cache tests.

See the [verification audit](docs/final-acceptance.md) for detailed results, coverage and limitations. Validate the API contract with `openapi-spec-validator backend/docs/openapi.yaml`.

## Decisions and trade-offs

1. **Explicit SQL and domain modules:** pgx keeps locking/constraint behavior visible; cohesive modules and narrow interfaces support extension. This requires more SQL/scanning/wiring than an ORM but avoids a generic repository or DI framework.
2. **Atomic upsert with reference locks:** one transaction prevents lost updates and delete races while allowing concurrent compatible Item share locks. Quantity updates on the same pair serialize in PostgreSQL; no global mutex is needed.
3. **Server search/category options:** full-dataset search and snapshot metadata keep paginated UI filters correct. Small datasets use substring scans; no speculative search index is added. The small optional cache is limited to location discovery.
4. **URL state and TanStack Query:** parameter-aware keys, AbortSignal, 300ms debounce and history-aware drafts avoid stale search results. Mutations never retry, set detail cache and invalidate all lists; a failed refresh does not misreport a committed save.
5. **Router v6 retained:** the test explicitly requires v6. `npm audit` reports two moderate entries for react-router/react-router-dom with fixes requiring v7; this browser-only app has no SSR hydration and navigation targets are fixed local routes or bounded numeric IDs, never client-supplied redirect URLs. The advisories remain documented rather than silently upgrading the required major. No critical/high npm findings remain.

## Further documentation

- [Screenshots](docs/screenshots/README.md): actual Chrome captures with disposable fixture data.
- [Architecture and transaction/cache decisions](docs/enterprise-design.md).
- [OpenAPI contract](backend/docs/openapi.yaml).
- [Deployment guide](docs/deployment.md) and [alternative deployment Compose](deploy/compose.yml).
- [Verification audit](docs/final-acceptance.md).