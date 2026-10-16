# Deployment preparation

**LIVE DEPLOYMENT: BLOCKED / NOT VERIFIED.** No host, managed PostgreSQL account, domain, reviewer credentials or publication authorization has been supplied. No external deployment, Git push, DNS change or live URL exists from this work.

## Selected strategy

A small Docker VM runs Caddy, the existing Nginx frontend and the Go backend; a managed PostgreSQL service provides durable storage, backups and TLS. Optional Redis runs privately beside the app. This uses the exact tested frontend/backend images and one HTTPS origin: browser → authenticated Caddy → Nginx → `/api/` → Go. Nginx preserves API paths and handles every SPA deep link. No CORS configuration is needed. Frontend builds with the default `/api/v1`; no browser credentials or database connection strings belong in VITE variables.

This strategy has recurring VM, managed database and domain costs. It is straightforward to review but requires host updates, monitoring, backups and a single-host availability trade-off. A static Vercel/Netlify frontend alone would not supply the Go API/database, so it is not the chosen functional demo strategy. It remains a demo preparation, not an operational production certification.

## Access control is required

The application itself has no user authentication or authorization. The prepared [Caddyfile](../deploy/Caddyfile) protects **all** frontend and API paths with HTTPS Basic Authentication using a hashed password. The [deployment Compose file](../deploy/compose.yml) publishes only Caddy's 80/443; backend, frontend, database and Redis have no public port mapping. Do not expose backend 8080 directly. This shared demo gate is sufficient for a restricted assessment demonstration; it does not provide application roles, per-user auditing or a multi-tenant security model. Use only non-sensitive demo data.

Caddy requires hashed passwords; generate one interactively with `caddy hash-password`. Store the resulting hash and actual connection secrets outside Git, using the host's secret manager or a protected deploy/.env. Keep bcrypt hashes single-quoted in .env so `$` is literal. The configuration follows the [official Caddy basic_auth reference](https://caddyserver.com/docs/caddyfile/directives/basic_auth).

## Authorized operator runbook

1. Obtain explicit authorization to publish the demo; choose the hosting account, domain, budget, reviewer user and password delivery method. Provision managed PostgreSQL with TLS verification and backups. Create a migration-capable database role; use a more restricted runtime role if the environment permits separate credentials.
2. Point the chosen domain's DNS to the VM. Restrict its firewall to SSH administration and HTTPS/ACME ports. Copy reviewed source or approved images to the host. The prepared Caddy image is pinned by digest; review image updates before release.
3. Copy `deploy/.env.example` to `deploy/.env` on that host and fill actual secrets. Set DATABASE_URL to a properly URL-encoded managed PostgreSQL connection with `sslmode=verify-full`, installing the provider CA when required. Never reuse local demo credentials. REDIS_URL stays empty to disable caching, or `redis://redis:6379/0` with the cache profile. External Redis should use a TLS `rediss://` URL and protected credentials.
4. From repository root, inspect the configuration without starting it: `docker compose --env-file deploy/.env -f deploy/compose.yml config --quiet`. Avoid printing resolved secret values. Build and validate the Caddyfile in an isolated local container before release.
5. After approval, `docker compose --env-file deploy/.env -f deploy/compose.yml up --build -d --wait` (add `--profile cache` before `up` when Redis is enabled). The one-shot Goose migration must exit 0 before backend startup. Existing migrations are unchanged; future schema changes must use forward migrations. Never run destructive Down against retained data.
6. Verify internal backend `/healthz`, database TLS, migration version, restart behavior and same-origin API access. Verify unauthenticated frontend and mutation requests receive 401 at Caddy; authenticated requests must work over HTTPS. Test create, edit, receive, transfer, soft delete and direct SPA refresh using disposable demo records. Confirm Redis outage does not block operations.
7. Configure monitoring for health/error rates, protected application logs and managed DB backups; test restoration. For release rollback, retain prior images and use compatible forward schema changes. Console stock logs are best effort and are not a durable warehouse transaction ledger.

## What was verified locally

The production frontend bundle and API run through the existing Nginx same-origin proxy. All nine SPA routes survive direct navigation/reload in real Chrome. Local runtime containers are non-root; PostgreSQL migrations and readiness gates passed on a fresh volume and the retained developer volume. Redis-enabled runtime and actual stopped-Redis fallback passed. Deployment Compose structure and Caddyfile validation are checked separately; live DNS, certificates, hosting resources, managed database TLS/backups, public 401 protection and external availability remain **NOT VERIFIED** until an authorized deployment is executed.

Required handoff inputs: hosting/account access and budget, managed PostgreSQL connection/CA, domain/DNS control, demo access credentials, authorization to publish, and (separately) GitHub remote/reviewer details if repository publication is desired. No live URL should be listed before those checks complete.
