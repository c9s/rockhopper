# Rockhopper Roadmap

A living checklist of where rockhopper is headed. Grouped by theme and roughly
ordered by priority. Checkboxes are for tracking — tick them as work lands.

**Framing:** rockhopper's unique selling point is *embeddable + AI-native*. The
**Correctness & Safety** items make it safe to recommend in production; the
**AI & Ecosystem** items make it the obvious choice for AI-assisted teams. Invest
in both; let the parity features follow demand.

## Suggested sequencing

1. **Quick wins** — small bug fixes and doc corrections (section F).
2. **The safety trio** — advisory locking → checksums → out-of-order policy (section A).
3. **Lean into the differentiator** — `--dry-run` + `validate`, then the MCP server and plugin packaging (sections C & D).
4. **Parity features** follow demand (section E).

---

## A. Correctness & safety (highest priority)

What separates "works on my machine" from "trusted in production".

- [ ] **Concurrency lock.** No serialization exists today (no advisory lock in the
      codebase). On a rolling deploy, multiple instances boot and call `Up()`
      simultaneously. Add per-dialect locking: Postgres `pg_advisory_lock`,
      MySQL `GET_LOCK`, etc. *Most important missing feature for real deployments.*
- [ ] **Checksum / drift detection.** Nothing hashes migration bodies, so editing
      an already-applied migration goes unnoticed. Store a content hash in
      `rockhopper_versions` and verify on load (Flyway / golang-migrate both do this).
- [x] **Out-of-order migration policy.** `up` now detects pending migrations whose
      version is below the highest applied version (`DB.InspectMigrations`) and
      **rejects by default** with an actionable `OutOfOrderError`. Opt in with
      `rockhopper up --allow-out-of-order` to apply them in place (with a warning).
      Follow-up: apply the same guard to the library `Upgrade()` path and surface
      out-of-order migrations in `status`.
- [ ] **Duplicate version handling.** `MigrationSlice.Less` (`migration.go`) panics on
      equal versions. Two branches generating migrations in the same second is
      realistic. Replace the panic with a graceful error; consider collision
      detection in `create`.

## B. Dialect coverage & testing

- [ ] **Decide MSSQL's fate.** Advertised in `CLAUDE.md` but not wired — there is no
      `DialectMSSQL`, and `Open()` only allows postgres/sqlite3/mysql. Either
      implement it or remove the claim (see section F).
- [ ] **Generalize DSN-from-env.** `BuildDSNFromEnvVars` only supports MySQL. Extend
      to a per-dialect DSN builder (builds on the recent `MYSQL8_` prefix work).
- [ ] **Real-database integration tests.** Tests are SQLite-in-memory only. Add
      container-based tests against real MySQL/Postgres — especially before adding
      locking, which is inherently dialect-specific.

## C. Operator / developer experience

- [ ] **`--dry-run`** for `up`/`down` — print the SQL it *would* run without executing.
      High value, and pairs naturally with the AI skills.
- [ ] **`validate` / `lint` command** — detect duplicate versions, missing `-- +down`,
      and parse errors *before* hitting them mid-deploy.
- [ ] **Machine-readable `status` (`--json`)** for CI gates.
- [ ] **Rollback ergonomics** — warn when a migration has no `-- +down` instead of
      silently no-op'ing.

## D. AI & ecosystem (widen the lead)

- [ ] **MCP server (`rockhopper mcp`)** — expose status/up/down/create/compile as
      structured tools so *any* agent can drive rockhopper, not just Claude Code.
      Safer than shell parsing.
- [ ] **Package skills as a Claude Code plugin / marketplace.** The `skills install`
      scaffolder is the project-local tier; a plugin is the global/discoverable tier.
- [ ] **Higher-value skills:**
  - [ ] "Diagnose a failed migration"
  - [ ] "Generate a migration from a schema diff"
  - [ ] "Review this migration for footguns (locking DDL, non-`CONCURRENTLY` indexes)"

## E. Parity features (lower priority)

- [ ] **Repeatable migrations** — re-run on checksum change for views / procedures /
      seed data (Flyway's `R__` concept).
- [ ] **Baseline** — adopt rockhopper on an existing database. `align` covers part of
      this but isn't framed as a baseline workflow.

## F. Quick wins (do first)

- [ ] **`config.TableName` is ignored.** It's parsed from YAML but `OpenWithConfig`
      (`db.go`) always passes the `TableName` constant. Either wire it through or
      drop the field. (Docs already note the CLI uses the default table name.)
- [ ] **MSSQL doc mismatch.** `CLAUDE.md` lists MSSQL as supported; the code doesn't
      wire it. Align docs with reality (or implement — see section B).
- [ ] **Remove leftover debug print.** `fmt.Println("preRunE")` in
      `cmd/rockhopper/main.go`.
