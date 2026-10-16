# Final technical-test audit

Audit date: 16 October 2026, Asia/Jakarta. Source: all five physical pages of the original `Technical Test - Fullstack Compact.pdf`, read from the user's Downloads folder with local PDFKit after Firecrawl Parse returned a connection failure. The supplied PDF and extracted temporary copy are not committed. The approved [technical design](technical-design.md) is preserved.

**PASS** = implemented and exercised, or a repository/documentation requirement inspected directly. **FAIL** = implemented but failing. **NOT VERIFIED** = insufficient evidence or an intentionally unperformed external submission action. **MISSING** = implementation absent. Submission and human-review requirements are distinct from working local product behavior.

## Executed verification

| Check | Result | Evidence and limits |
|---|---|---|
| Frontend installation / Docker npm ci | PASS | Locked dependencies, existing React 19 / Router v6 preserved; official shadcn CLI installed actual registry source |
| Vite development command/proxy | PASS | Final npm run dev on 5173: actual Chrome pages, live dashboard metrics and API validation/405 via development proxy; server then stopped |
| TypeScript strict typecheck and production build | PASS | All app/test sources; Vite build split into app/react/query chunks |
| Frontend unit tests | PASS | 18 tests in 6 files: debounce/history, API abort/error/envelope parsing, forms, render boundary, single-attempt stock transport |
| Full Go suite with PostgreSQL/Redis | PASS | TEST_DATABASE_URL and TEST_REDIS_URL enabled; disposable PostgreSQL 17.11 databases, actual Redis 7.4, pinned Goose |
| Go race detector | PASS | Full transaction/concurrency/HTTP/cache suite under `go test -race ./...` |
| go vet / go mod verify | PASS | Static checks and module checksums |
| OpenAPI 3.0.3 | PASS | Seven mandatory plus eight additional operations; schema validation and request/response example checks |
| Local Docker Compose config/build/start | PASS | Fresh disposable volume and retained volume; database readiness → one-shot migration exit 0 → healthy backend → frontend, with optional Redis |
| Chrome complete browser suite | PASS | 28 tests: 8 Item + 11 enterprise + 5 UI + 4 location tests, all actually executed; Select menus, keyboard focus, row density, action menus and module search added |
| Transaction correctness | PASS | Actual PostgreSQL success/insufficient/same/missing/deleted/overflow, same-source/opposing, receive/transfer, both delete orderings, injected rollback, conservation, cancellation, dropped COMMIT acknowledgements |
| Real Redis cache | PASS | Location service miss/hit/TTL expiration/malformed entry/outage fallback; runtime namespaced key and ~30s TTL verified |
| Redis stopped in Compose | PASS | `scripts/verify-redis-outage.mjs`: actual Chrome locations/receive still work; PostgreSQL committed balance 3; own fixture soft-deleted |
| Cache disabled | PASS | Standard real integration composition uses no cache; mandatory and enterprise APIs pass; empty REDIS_URL disables runtime cache |
| Responsive rendering | PASS | Actual Chrome 1440/1280/768/390: no document overflow, table-contained scrolling, desktop collapse, tablet/mobile Sheet, combobox focus/Escape and bounded dialogs; screenshot inspection |
| Item and stock browser workflows | PASS | Real create/edit/409, detail/location balances, receive, inventory refresh, confirmed transfer 12 → 7+5, insufficient rejection, delete and deleted receive/transfer rejection |
| Unknown receive result | PASS | Browser lets actual receipt commit then drops response; stored 7→9, one POST only, disabled resubmission until balance refresh/cleared quantity |
| Global Error Boundary | PASS | Failing unit component and instrumented served JS in actual Chrome show safe recovery; no production crash hook |
| Loading/error/retry/empty | PASS | Real requests with latency gates and controlled 503; normal Query retry remains distinct from render fallback |
| All SPA deep links | PASS | Nginx direct/reload checks for every application route |
| Retained developer data | PASS | Before/after digest of all preexisting Item/Stock/Location rows identical around final rebuild; no reset or destructive migration |
| Deployment preparation | PASS | Separate Compose with managed-DB env, private backend, authenticated Caddy edge; Compose structure and temporary-container Caddy validation passed, no public ports opened for validation |
| Live deployment / public HTTPS access | NOT VERIFIED / BLOCKED | Missing host, domain, secrets and publication authorization; no live URL or deployment claimed |
| Dependency audit | FAIL for zero-finding target | Two moderate Router v6 package entries remain; no high/critical. Required v6 retained; see limits |
| Frontend lint | NOT VERIFIED | No lint tool configured; strict types, tests and build are selected checks |
| GitHub publication/email/deadline/human review | NOT VERIFIED | External actions not authorized; candidate identity, recipient and actual receipt timestamp unknown |

Browser checks target **warehouse-enterprise-acceptance**, frontend 3001/backend 18081, with its own fresh disposable PostgreSQL volume. The retained **warehouse-assessment** project stays frontend 3002/backend 18080. Only temporary test databases/projects are destroyed. Intentional captures live under [screenshots](screenshots/README.md); temporary traces/results/node_modules/dist and .firecrawl research files are ignored.

The inventory table was widened after screenshot review to keep columns readable; narrow screens scroll inside the table. A final complete fresh-volume browser run verifies that change. Refactored stock form state/mutation policy lives in useStockOperation; uncertainty freezes item/location/quantity selection until balance reconciliation. Item totals use BigInt; aggregate parsers reject values outside safe JavaScript integer precision. All checks above describe executed local evidence, with deployment and submission boundaries kept explicit.

## PDF functional requirements

IDs follow the existing technical-design matrix so every mandatory requirement has a traceable row.

| ID | PDF | Requirement | Status | Evidence |
|---|---|---|---|---|
| MF-01 | p1 §1.2A | Item id/sku/name/category/unit/created_at | PASS | Migration, Item domain, API create/detail/update tests |
| MF-02 | p1 §1.2A | Location id/code/zone/type | PASS | Migration/seed tests; Stock lookup includes discovered Location data |
| MF-03 | p1 §1.2A | Stock id/item_id/location_id/qty/updated_at | PASS | Real receive/lookup persistence tests |
| MF-04 | p2 §1.2B | POST Item | PASS | API T-01 and browser actual create/pending/success |
| MF-05 | p2 §1.2B | List with category/pagination | PASS | API T-08–10; actual browser filter/pagination across 26 records |
| MF-06 | p2 §1.2B | Item detail | PASS | API T-05; edit fetch and direct-route reload |
| MF-07 | p2 §1.2B | Item update | PASS | API T-06; browser edits all fields and verifies original created_at |
| MF-08 | p2 §1.2B | Item soft delete | PASS | T-07 physical records retained; deleted Item/Stock views unavailable |
| MF-09 | p2 §1.2B | Stock receive | PASS | T-12–20/23–24 and real container smoke |
| MF-10 | p2 §1.2B | Stock all-location lookup | PASS | T-22 and two-location container smoke |
| MF-11 | p2 §1.2C | Unique SKU, duplicate 409 | PASS | Named constraint/concurrent API tests; browser duplicate field/message/toast |
| MF-12 | p2 §1.2C | Nonnegative stock, service validation | PASS | Stock service unit rejection and PostgreSQL named CHECK tests |
| MF-13 | p2 §1.2C | Application stock mutation log | PASS | Exactly one committed log per confirmed receive; rollback/unknown tests and smoke inspect fields |
| MF-14 | p3 §2.1 | SKU/name/category/unit table | PASS | Actual Chrome at 1280px/768px, screenshots inspected |
| MF-15 | p3 §2.1 | Debounced SKU/name search ≥300ms | PASS | Unit 299ms/300ms boundary; browser finds off-page SKU/name |
| MF-16 | p3 §2.1 | Category dropdown | PASS | Complete meta.categories, including off-page category; browser filtering |
| MF-17 | p3 §2.1 | Pagination | PASS | Server meta.total; disjoint pages, filter reset, URL history, invalid-page recovery |
| MF-18 | p3 §2.1 | Create/Edit navigation | PASS | Browser links, direct navigation and SPA reload |
| MF-19 | p4 §2.1 | Client required fields | PASS | Unit/Chrome validation prevents empty submission; Unicode/control/bounds checks |
| MF-20 | p4 §2.1 | Form calls own create/update API | PASS | Actual browser HTTP 201/200 and database-backed detail comparison |
| MF-21 | p4 §2.1 | Pending form state | PASS | Browser latency gate verifies disabled inputs/button; unit prevents concurrent submission |
| MF-22 | p4 §2.1 | Show API success/error messages | PASS | Actual backend messages in success/409 toast and form alert |
| MF-23 | p4 global | Fetch skeleton/spinner | PASS | Browser holds initial real GET, observes skeleton then real rows |
| MF-24 | p4 global | Success/error toasts | PASS | Actual create/update/409; duplicate toast count exactly one |
| MF-25 | p4 global | Desktop 1280px+, tablet 768px | PASS | Actual viewports; document-width assertions and screenshot inspection |

## PDF technical requirements

| ID | PDF | Requirement | Status | Evidence |
|---|---|---|---|---|
| MT-01 | p1/p2 | Go backend | PASS | Existing pinned Go source and full regression |
| MT-02 | p2 §1.3 | PostgreSQL | PASS | Real PostgreSQL 17.11, no SQLite substitution |
| MT-03 | p2 §1.3 | Docker and Compose | PASS | Both fullstack projects built/started with readiness checks |
| MT-04 | p2 §1.3 | OpenAPI 3.0 | PASS | Valid 3.0.3, seven mandatory + eight documented additional operations |
| MT-05 | p2 architecture | Handler → service → repository | PASS | Existing module ownership and import inspection; app constructors wire dependencies |
| MT-06 | p2 architecture | Request logging middleware | PASS | Regression tests and actual container method/route/status/duration/request_id logs |
| MT-07 | p2 architecture | Central error handler | PASS | App-injected error mapper; validation/domain/SQL/panic HTTP regression |
| MT-08 | p2/p3 | Standard response envelopes | PASS | API regression covers success/error/empty/unknown paths/methods/body cap |
| MT-09 | p3 | Handler validation before service | PASS | Item/Stock handler spies prove no service calls for invalid requests |
| MT-10 | p3 | Env configuration | PASS | Templates, config validation, no secret files in source; local demo defaults labeled |
| MT-11 | p3 | Migration scripts | PASS | Actual Goose up/down/reapply/refusal tests; fresh-volume migration |
| MT-12 | p4 §2.2 | React 18/19 + TypeScript | PASS | React 19.3.1; strict tsc and Vite build |
| MT-13 | p4 §2.2 | Router v6, list/create/edit | PASS | v6.30.6 pin; nine browser-tested application routes |
| MT-14 | p4 §2.2 | TanStack Query | PASS | Queries/mutations/providers, key parameters, invalidation, real UI flows |
| MT-15 | p4 quality | Custom hooks | PASS | useItems/useItem/useItemMutations/useItemFilters/useDebouncedValue |
| MT-16 | p4 quality | Modular components | PASS | Table, filters, form, empty/loading/error/pagination/toast components separated |
| MT-17 | p4 quality | TypeScript API types | PASS | Shared envelopes and Item/list/write/meta types with runtime response checks; Stock/Location/Inventory/Summary/Transfer runtime response checks |

## PDF deliverables and submission constraints

| ID | PDF | Requirement | Status | Evidence or required action |
|---|---|---|---|---|
| MD-01 | p3 §1.5 | GitHub source repository | PASS | Source pushed to the user-authorized farhanpamungk4as/warehouse-management-system repository |
| MD-02 | p3/p5 | docker compose up --build | PASS | Complete five-service fresh-volume and retained-volume startup |
| MD-03 | p3 | OpenAPI file or docs route | PASS | backend/docs/openapi.yaml is committed; interactive Swagger UI optional |
| MD-04 | p3 | Migration scripts | PASS | backend/migrations contains both versioned SQL files |
| MD-05 | p3 | .env.example | PASS | backend and frontend templates included and ignored real env files |
| MD-06 | p3/p5 | README setup/architecture | PASS | Concise root README includes frontend/backend/Docker/env/tests |
| MD-07 | p5 §3.2 | ≥2 technical decisions/trade-offs | PASS | README has five concrete decisions with limitations |
| MD-08 | p1/p5 | Honest AI disclosure | PASS | Tools/contributions/verification named; no invented manual authorship |
| MD-09 | p4 §2.3 | Frontend in same repository | PASS | frontend source and lockfile included in local final commit |
| MD-10 | p4 §2.3 | npm install && npm run dev | PASS | Actual install/dev launch plus HTTP HTML/proxy/direct-route checks |
| MD-11 | p4/p5 §3.1 | Required repository structure | PASS | cmd/internal/migrations/docs/Dockerfile/env; frontend required folders; root Compose/README |
| SC-01 | p5 §3.3 | ≤2 calendar days after receipt | NOT VERIFIED | Actual receipt timestamp/cutoff interpretation unknown; do not infer it from file times |
| SC-02 | p5 §3.3 | Public repo or invited reviewer | NOT VERIFIED | Needs authorized publication and reviewer access check |
| SC-03 | p5 §3.3 | Required email subject/submission | NOT VERIFIED | Subject template preserved; actual candidate name/recipient and authorization absent |
| EG-01 | p1/p5 guidance | Candidate understands generated code | NOT VERIFIED | Requires candidate review/explanation; automated checks cannot establish human understanding |
| EG-02 | p5 guidance | Error handling/docs/non-obvious comments | PASS | Central errors, clear README and comments for transaction locks, proxy and async UI behaviors |
| EG-03 | p1 guidance | Structured maintainable implementation | PASS within local scope | Modular code, pinned builds and stated limits; no claim of operational production certification |

No mandatory local implementation feature is MISSING or left FAIL. Repository visibility/reviewer access, external deployment/submission and human review remain explicitly unverified. The new enterprise request explicitly authorizes the bonus implementations below; Location CRUD is added by the subsequent user request; a durable movement ledger remains outside scope.

## Bonus and enterprise requirements

| Requirement | Status | Evidence |
|---|---|---|
| Actual shadcn/ui and Lucide/Sonner | PASS | Registry Button/Card/Input/Table/Badge/Command/Popover/Dialog/AlertDialog/Sheet/Separator/Skeleton/Sidebar/Sonner/Tooltip source, components.json, real rendered workflows |
| Shared shell and navigation | PASS | Nine routes, root redirect, desktop collapse, mobile/tablet Sheet and breadcrumbs |
| Real dashboard metrics/distribution | PASS | One PostgreSQL aggregate statement, zero/active/soft-deleted cases and browser API comparison |
| Item detail and per-location stock | PASS | Existing detail + stock endpoints, BigInt total, real timestamps/location info and actions |
| Delete confirmation | PASS | Browser cancel/confirm, 404 active visibility and retained physical records |
| Global Inventory | PASS | Efficient joined pagination, literal search, category/location filters, repeatable-read metadata; real receive refresh |
| Receive page | PASS | Searchable active item selector, discovered locations, bounded integer input, pending gate, committed result, invalidation and reconciliation warning |
| Atomic Transfer API and UI | PASS | All required real-PG concurrency/failure cases and Chrome confirmation/balance/conservation/409 flows |
| Locations CRUD page/API | PASS | Actual data, create/edit/delete dialogs, cached list invalidation, live stock menus, duplicate-code errors and stock-protected deletion; no hardcoded seed IDs |
| Global Error Boundary | PASS | Unit failure and actual browser render failure with safe recovery actions |
| Real Redis integration | PASS | Short-TTL location-only cache, actual hit/miss/expiry, bounded no-retry fallback and stopped-container Chrome check |
| Responsive desktop/tablet/mobile | PASS | Four requested widths, actual assertions and captured/inspected images |
| Production-compatible preparation | PASS locally | Authenticated HTTPS edge, managed PostgreSQL, same-origin Nginx, optional Redis, env templates, migration/health runbook and config validation |
| Hosted functional demo | BLOCKED / NOT VERIFIED | Hosting/domain/credentials and explicit publication approval absent; see deployment.md |


## Limits and decisions for review

- React Router v6 is mandated. npm audit reports two moderate package entries in react-router/react-router-dom (open redirect with untrusted navigation and SSR hydration deserialization; advisories GHSA-wrjc-x8rr-h8h6 and GHSA-337j-9hxr-rhxg). Available audit fixes require v7. The app is browser-only, has no SSR hydration, and routes only to constants or validated numeric Item IDs; no untrusted redirect destination is used. Advisories remain visible; a future major migration requires contract approval. Updated Vitest 4.1.11 removes the initially observed high-impact test-tool advisories.
- Browser acceptance was executed in installed Google Chrome. Safari/Firefox and production hosting were not verified. No lint configuration was introduced.
- Writes are not idempotent. Last-committed Item edits win; offset pages may shift across requests; console logs have a crash/audit gap. These are the approved scope limitations.
- Original assessment read locally; Firecrawl parse unavailable; no original PDF or extracted assessment text is included in Git.
- See [backend verification history](backend-verification-history.md) for prior Phase 02–05 evidence. The enterprise design addendum supersedes earlier deferred bonus scope; earlier phase evidence remains historical.

## Publication handoff

Source has been pushed to the user-authorized GitHub repository. After candidate review, ensure it is public or the reviewer has access. No visibility change, email or submission has been performed. Confirm the actual receipt date and the reviewer address; use email subject `[Technical Test] Fullstack – [Nama Kandidat]`. Include the repository URL and README, add a live demo only if one is later explicitly deployed, and retain the honest AI disclosure.

## UI revision based on supplied admin reference — 16 October 2026

The previous navy/teal presentation and visible native selects were replaced with a light sidebar, neutral surfaces, black primary actions, a compact list panel, category chips, blue badges and locally bundled Inter. Item, inventory, receive, transfer, zone and page-size selectors use actual shadcn Select components; row actions and density use DropdownMenu. Ctrl/⌘ K opens a working page-search command dialog. Create/edit headings follow the same hierarchy. No fake notifications, profile authentication, sorting or bulk actions were added.

All 24 Chrome flows and 18 frontend tests passed; strict TypeScript and production Docker build passed. Responsive screenshots were inspected at 1440/1280/768/390, including open menus. A focused UI rerun checks the final table widths so long category fixtures do not inflate mobile rows. Dropdown captures use viewport screenshots because full-page capture can temporarily reposition fixed Radix portals in Chrome; actual menu dimensions, hit targets and selection were inspected. The backend is unchanged in this UI revision; its earlier PostgreSQL/Redis/race evidence remains applicable and is not reported as a new run. All fixture writes used the isolated warehouse-ui-acceptance project; the retained database was not reset or seeded.

The follow-up Card/Button consistency pass migrates the remaining list/form/empty/error surfaces to shadcn Card and custom action buttons to shadcn Button, including category chips, module search, retry, back/cancel links and dashboard shortcuts. Item forms use CardHeader, CardTitle, CardDescription, CardContent and CardFooter while retaining native form semantics and pending locks. Legacy standalone button styles were removed. All 18 frontend tests and 24 Chrome flows passed again, along with strict TypeScript and the production Docker build; form and mobile list captures were visually inspected. Only the retained frontend container was rebuilt.

## Location CRUD, list-card actions and demo seed follow-up

Location GET detail/POST/PUT/DELETE now complement GET list. No schema or migration changes were needed. Strict code/zone/type validation, unique-code errors, 404s and FK-protected hard deletion preserve stock references (including zero balances). The cache key is invalidated after successful writes; local locking prevents reader refill races, and failed invalidation bypasses the key for its TTL without reporting a committed write as failed. Multi-backend cache coherence is TTL-bounded; this deployment uses one backend.

Create item, Receive stock and Create location actions live inside their list cards. Shadcn location create/edit dialogs preserve input, disable pending writes and focus API validation errors; delete requires confirmation. Receive, transfer and inventory menus read GET /locations and refresh after mutation/navigation.

Verification: all 28 Chrome flows passed, all 18 frontend tests passed, production Docker build/strict TypeScript passed, and the full Go race suite ran against disposable PostgreSQL/Redis. Three new integration tests cover CRUD validation, both receive/delete orderings, real Redis invalidation and purge-failure fallback. The demo script was run twice on the isolated empty database: first run created two locations, ten items and 900 stock units; second run added zero items/units. Postman CLI asserted the resulting dashboard totals with three passing assertions and event reporting disabled. Retained database data was not seeded or reset.
