# duckdb-ui

## Purpose

Hosts DuckDB's built-in browser SQL console (`duckdb -ui`) as a long-lived Nexus
service, opened against a directory of your choice — so anyone with access to the
console can run ad-hoc SQL over Parquet (or CSV, JSON, ...) files there without a
terminal session on the host. Carries no opinions about which domain it's routed on or
which directory it opens (that's the consumer's job — see "Required environment" below,
and `../traefik/README.md` + `../authelia/README.md` for wiring up the route). This
project only runs the console itself.

**Read-write, not read-only — this is not a bug.** `-readonly` was tried first and
confirmed live to break the `ui` extension's own initialization (it needs to write its
internal `_duckdb_ui` catalog — see "Volumes" below); there's no read-only mode for this
tool. Anyone with access to the console has the same effective trust level as shell
access to the host: they can `CREATE TABLE`/`COPY ... TO` into `DUCKDB_UI_DATA_DIR`. Gate
access accordingly (see "Dependencies").

**Requires nothing else deployed first** — it doesn't touch Traefik's `dynamic` volume
itself; a consumer project publishes its own route the same way `nexus-web-route`/
`retu-route` do in `retu`'s own `nexus.yaml` (see that repo for a worked example). A
consumer's route MUST target the proxy's port (`4214`), not `duckdb-ui`'s own port
(`4213`) — see "Services & ports" and "Why duckdb-ui-proxy exists" below.

## Volumes

| Volume | Path (`$NEXUS_VOLUME_<NAME>`) | Contents | Extension point? |
|---|---|---|---|
| `bin` | `~/.nexus/volumes/duckdb-ui/bin` | downloaded `duckdb` binary, its extension cache (`ui`), `stdin.fifo` (see "Services & ports"), and the built `duckdb-ui-proxy` binary | no |
| `data` | `~/.nexus/volumes/duckdb-ui/data` | `catalog.duckdb` — the `ui` extension's own state (saved notebooks/queries, its internal `_duckdb_ui` catalog) | no |

`bin` can be wiped safely (re-downloadable/rebuildable). `data` is worth backing up once
you've actually saved notebooks/queries in the console — it's the only state this project
itself owns (the Parquet tree it's querying, `DUCKDB_UI_DATA_DIR`, belongs to the
consumer, not this project).

## Services & ports

| Service | Binds |
|---|---|
| `duckdb-ui` | `[::1]:4213` — **IPv6 loopback only, confirmed live, not `127.0.0.1`.** DuckDB's `ui` extension has a `ui_local_port` setting but no bind-host setting. Not meant to be reached directly by a consumer's Traefik route — go through `duckdb-ui-proxy` instead (see below). |
| `duckdb-ui-proxy` | `[::1]:4214` — **this is what a consumer's Traefik route should target.** |

The `-ui` web server's lifetime is tied to the CLI's own stdin loop — it exits the
instant stdin hits EOF (there is no separate daemon/headless flag), so stdin has to stay
open forever for this to work as a service at all. `run:` does this with `exec duckdb
... <> stdin.fifo`: opening a FIFO for read+write blocks forever on read without needing
a second process to hold the write end open, and `exec` replaces the wrapping shell with
`duckdb` in place (same PID). Both matter — an earlier version used `sleep infinity |
duckdb -ui` instead, which is two processes; confirmed live, stopping that service only
signalled the shell Nexus had started, and `duckdb` (orphaned, reparented) kept running
and holding the port, requiring a manual kill to unstick the next deploy. A single exec'd
process has nothing for Nexus to lose track of.

### Why `duckdb-ui-proxy` exists

The `ui` extension's HTTP server hardcodes its CSRF guard (`src/http_server.cpp` in
`duckdb/duckdb-ui`, confirmed by reading the actual upstream source) to accept only
requests whose `Origin`/`Referer` is exactly `http://localhost:<port>` — deliberate, and
not configurable via any flag or environment variable. That's fundamentally incompatible
with reaching the console through a real domain behind Traefik: a browser's `Origin`
header can't be spoofed by page JS, so no client-side or hostname trick can satisfy it
(confirmed live — routing straight to `duckdb-ui` produces 401s from `/localToken` and
`/ddb/run` even once the frontend loads).

Patching and self-compiling the `ui` extension to relax this check was tried first and
abandoned: it vendors the entire DuckDB engine as a git submodule, so a from-source
rebuild is a large, slow, high-maintenance C++ build for what is fundamentally a
"rewrite two request headers" problem. `duckdb-ui-proxy` (`proxy/main.go`, a single file,
no external dependencies, builds in well under a second) is a small `net/http/httputil`
reverse proxy that sits between Traefik and `duckdb-ui`: it rewrites incoming
`Origin`/`Referer` to `duckdb-ui`'s own `local_url` before forwarding, so the extension's
check passes, while the actual access control (Authelia, in front of Traefik) still does
the job that check exists to approximate on a bare-localhost setup. The frontend's
separate hostname gate (`window.location.host.includes("localhost")`) is unrelated to
this and is satisfied purely by the consumer's route hostname (see `duckdb-route`'s
comment in `retu`'s `nexus.yaml`) — no frontend changes needed at all.

## Dependencies

- None to start. Commonly paired with `../traefik` (to expose it) and `../authelia` (to
  gate it) — this console has **no authentication of its own** and can read any path the
  host user can read, so routing it through the internet without `authelia@file` (or
  equivalent) in front is a real risk, not a hypothetical one.

## Required environment

Set per-consumer, in whatever project nests this one — not an operator `.env` file, since
it's not a secret. `nexus.yaml` declares it as required: the deploy fails loudly at build
time if it's missing.

```yaml
# in the consumer's own nexus.yaml, nesting this project:
duckdb-ui:
  src: github.com/rdkal/nexus-infra/v0/duckdb-ui
  ref: main
  environment:
    DUCKDB_UI_DATA_DIR: /path/to/whatever/directory/you/want/queryable
```

Relative-path queries in the console (`read_parquet('raw/kalshi/*.parquet')`) resolve
against this directory, exactly as if a human had `cd`'d there on a terminal.

## Deploy / rollback notes

- `bin` can be wiped safely; rebuilt (including a fresh `INSTALL ui`, which needs
  network access) on the next deploy.
- `data` should be backed up once it holds real saved notebooks/queries — losing it
  doesn't affect `DUCKDB_UI_DATA_DIR` at all, just the console's own saved state.
- The `ui` extension bundles its own frontend assets — after the first successful build,
  the service itself needs no further network access to serve the console.
