# duckdb-ui

## Purpose

Hosts DuckDB's built-in browser SQL console (`duckdb -ui`) as a long-lived Nexus
service, opened read-only against a directory of your choice — so anyone with access to
the console can run ad-hoc SQL over Parquet (or CSV, JSON, ...) files there without a
terminal session on the host. Carries no opinions about which domain it's routed on or
which directory it opens (that's the consumer's job — see "Required environment" below,
and `../traefik/README.md` + `../authelia/README.md` for wiring up the route). This
project only runs the console itself.

**Requires nothing else deployed first** — it doesn't touch Traefik's `dynamic` volume
itself; a consumer project publishes its own route the same way `nexus-web-route`/
`retu-route` do in `retu`'s own `nexus.yaml` (see that repo for a worked example).

## Volumes

| Volume | Path (`$NEXUS_VOLUME_<NAME>`) | Contents | Extension point? |
|---|---|---|---|
| `bin` | `~/.nexus/volumes/duckdb-ui/bin` | downloaded `duckdb` binary, its extension cache (`ui`), and a throwaway empty `catalog.duckdb` | no |

Nothing here is worth backing up — everything in `bin` is either re-downloadable or, for
`catalog.duckdb`, deliberately empty (see "Why `-readonly` needs a real file" below).

## Why `-readonly`, and why it needs a real file

The service runs with `-readonly`, which blocks any catalog write (`CREATE TABLE`,
`COPY ... TO`, etc.) through the console — `SELECT`/`read_parquet(...)` querying is
unaffected. Confirmed live: `-readonly` only has an effect against a real database
*file*; `duckdb -readonly` with no filename (DuckDB's in-memory default) errors outright
("Cannot launch in-memory database in read-only mode!"). The build step creates one
empty `catalog.duckdb` purely so `-readonly` has something to attach to — queries
against `DUCKDB_UI_DATA_DIR` never touch it.

## Services & ports

| Service | Binds |
|---|---|
| `duckdb-ui` | `[::1]:4213` — **IPv6 loopback only, confirmed live, not `127.0.0.1`.** DuckDB's `ui` extension has a `ui_local_port` setting but no bind-host setting; a consumer's Traefik route must target `http://[::1]:4213`. |

The `-ui` web server's lifetime is tied to the CLI's own stdin loop — it exits the
instant stdin hits EOF (there is no separate daemon/headless flag). `run:` in
`nexus.yaml` pipes `sleep infinity` into it to keep stdin open indefinitely, which is
what makes this work as a long-lived service at all.

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
- The `ui` extension bundles its own frontend assets — after the first successful build,
  the service itself needs no further network access to serve the console.
