# Enterprise extension design

This addendum extends the retained [technical design](technical-design.md). The earlier phase's deferred bonus scope is superseded by the user's enterprise request. The original seven required operations, schema, Goose migrations, soft deletion and receive locking remain intact. Four APIs and their domain layers are added; there are no new tables or speculative transaction-history claims.

## Transaction ownership and ordering

Stock TransferService owns validation and use-case sequencing through consumer-owned ItemGuard, TransferLocations, TransferWriter and TransferRunner interfaces. The composition root binds Item and Location repositories to the **exact** pgx transaction created by the Stock transfer runner. No service imports concrete repositories or platform infrastructure. The runner alone begins/commits/rolls back; independent bounded cleanup handles canceled request contexts.

The order is active item share lock → lower location exclusive lock → higher location exclusive lock → conditional source write → bounded destination upsert → acknowledged COMMIT. Transfers A→B and B→A therefore queue on the same first location before holding stock locks. Receive locks only its destination location KEY SHARE before upsert, so it either precedes the transfer's location locks or waits without holding stock writes. Item soft delete takes a conflicting item update lock and rechecks visibility in PostgreSQL: receive/transfer-first complete before deletion; deletion-first makes later guards return 404.

Source UPDATE includes `qty >= requested`; a missing source row is insufficient stock. Destination upsert enforces `existing <= MaxInt32 - delta`. Capacity failure, injected errors after both writes, or known precommit cancellation roll back the entire operation. Stored zero source balances remain visible; dashboard occupied count excludes them. Total quantity is conserved by transfers. This conservative strategy serializes all transfers sharing either location even across unrelated items; finer stock locks would improve throughput but require more complex handling of missing destination rows.

Confirmed transfer logs contain request ID, item/source/destination IDs, moved quantity, resulting balances and UTC timestamp. Receive logs retain their original before/after fields. COMMIT transport failure is classified as unknown unless an explicit server rejection proves failure. Transfer annotates the unknown error for its own safe HTTP message; mandatory receive wording remains unchanged. Neither operation retries. Console events are best effort, and there is no durable ledger/outbox/idempotency key.

## Reads and caching

Inventory's read-only REPEATABLE READ transaction executes a bounded page join, filtered count and distinct active category query. SKU/name search escapes LIKE wildcards. Location filtering happens in SQL; no item-by-item stock requests exist. Dashboard aggregates and sorted distribution use one statement snapshot, with COALESCE to zero and an empty array for an empty warehouse. Integers use PostgreSQL bigint for aggregates; frontend parsers reject unsafe JavaScript integers rather than silently displaying rounded values. Item detail sums individual integer balances with BigInt. At extreme scale, aggregate API fields would require a decimal-string contract before totals exceed Number.MAX_SAFE_INTEGER.

LocationService alone owns cache policy; platform/cache is a Redis transport adapter with no domain imports. Only the actual read-only location list is cached, never balances or changing aggregates. Key `warehouse:v1:locations`, 30s default TTL (maximum 1m); per-command 100ms timeout, no transport retries. A cache error or invalid payload falls through to PostgreSQL; cache fill failures do not fail successful reads. No cache health dependency gates the backend. Cache disabled mode uses an empty REDIS_URL; optional runtime client is closed on shutdown. A small static seeded list does not need caching for speed; this is a deliberately limited, tested bonus integration.

## UI and verification boundaries

Feature pages compose shared shadcn registry primitives, Lucide icons and typed Query hooks. URL filters retain history-aware 300ms drafts; item writes invalidate list/dashboard/inventory views. Stock mutations invalidate stock/inventory/dashboard without awaiting refresh as proof of commit. On uncertain mutation results, resubmission stays disabled until a fresh balance check and cleared quantity permit a manual attempt. Read retries are bounded; mutation retries are disabled.

Rendering errors reach a root class ErrorBoundary with reload/dashboard actions. It logs component details only in development. Normal Query failures keep their ordinary retry UI. The production browser test deliberately rewrites one served JS response in an isolated browser to throw; no failure route or backdoor exists in shipped code.

No authentication, roles, durable audit ledger, history analytics or operational production certification is implied. Deployment preparation protects the demo at the HTTPS edge and publishes only that edge; authorization, hosting, DNS, managed database and externally verified access controls remain separate. Local item/stock data is retained, while destructive checks use dedicated disposable projects/databases.

## Location CRUD follow-up

LocationService now exposes Create/Get/Update/Delete through a consumer-owned Store interface, alongside cached List. The existing schema supports the API without a migration. The PostgreSQL locations_code_key produces a domain duplicate-code error. DELETE relies on the stocks_location_id_fkey RESTRICT constraint rather than a separate existence check; concurrent receive/delete operations serialize and stock records, including zero balances, prevent deletion.

Successful location writes purge the Redis list key. A per-service RWMutex holds readers through cache fill and serializes local writes/invalidation; if DEL fails the process bypasses the stale key for its TTL. Other backend replicas can still serve a bounded stale snapshot until TTL, so stronger multi-replica coherence would require a versioned/distributed invalidation policy. Frontend location mutations patch/invalidate the shared query and invalidate dashboard, inventory and stock. Location lists refetch on navigation/focus. Demo seeding uses real API writes with discovered IDs; it is an explicit operator command and is never run automatically on startup.
