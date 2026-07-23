# traefik

## Purpose

Single edge reverse proxy for the host. Terminates TLS on `:443` via Let's Encrypt
(Cloudflare DNS-01 challenge) and routes to backend services on `127.0.0.1` using
file-provider dynamic config. This project only sets up the proxy itself — it carries
no routes for any particular app.

## Volumes

| Volume | Path (`$NEXUS_VOLUME_<NAME>`) | Contents | Extension point? |
|---|---|---|---|
| `bin` | `~/.nexus/volumes/traefik/bin` | downloaded `traefik` binary | no |
| `config` | `~/.nexus/volumes/traefik/config` | rendered `traefik.yml` (static config) | no |
| `dynamic` | `~/.nexus/volumes/traefik/dynamic` | **routers/services fragments, one file per project** | **yes** |
| `acme` | `~/.nexus/volumes/traefik/acme` | `acme.json` — Let's Encrypt cert storage | no, but must be backed up |

### How another project gets routed through Traefik (the extension point)

Traefik watches the `dynamic` volume (`watch: true`) and hot-reloads on any change — no
redeploy of traefik itself needed. To expose a service, from another project's
`nexus.yaml` build step:

```yaml
environment:
  TRAEFIK_DYNAMIC_DIR: ${NEXUS_TRAEFIK_DYNAMIC}   # resolved by nexus, no hardcoded path
build: |
  envsubst < config/traefik-route.yml.tmpl > "$TRAEFIK_DYNAMIC_DIR/<your-project>.yml"
```

`NEXUS_TRAEFIK_DYNAMIC` is injected automatically for every project once traefik is
deployed (nexus's `NEXUS_<PROJECT>_<VOLUME>` convention — see nexus's DESIGN.md). No
literal path to get wrong, and no manual coordination with whoever named the traefik
project. Note: **traefik must be deployed before the consumer project**, or the variable
won't exist yet.

In the dropped fragment, define a `router` (host rule, entrypoint `websecure`,
`certResolver: letsencrypt`) and a `service` pointing at `http://127.0.0.1:<your-port>`.
If the route needs auth, reference the `authelia@file` middleware — see
`../authelia/README.md`, which publishes that middleware into this same volume as a
worked example of this exact handshake.

## Services & ports

| Service | Binds |
|---|---|
| `traefik` | `:443` (public) |

## Dependencies

- None required to start. Individual routes in `dynamic/` will 502 until their target
  service is actually listening — that's expected, not an error state for traefik itself.
- Commonly paired with `../authelia` for routes that need auth.

## Required environment

Set these in `$NEXUS_HOME/env/traefik.env` (operator file — not in git, persists across
deploys). `nexus.yaml` declares both as required: the deploy fails loudly at build time if
either is missing, rather than silently rendering a broken config.

```sh
# ~/.nexus/env/traefik.env
ACME_EMAIL=you@example.com
CF_DNS_API_TOKEN=your-cloudflare-token-scoped-to-dns-edit
```

These are read directly by the traefik/lego process for the Let's Encrypt DNS-01
challenge. Isolation is automatic: only this project's build/service processes ever see
them (nexus's per-project environment, not the daemon's full environment).

## Deploy / rollback notes

- `acme` is the only volume that must be backed up — losing it means re-issuing every
  certificate (rate-limited by Let's Encrypt).
- `bin` and `config` can be wiped safely; they're rebuilt on the next deploy.
- Rolling back: `nexus project add` a previous tag/ref, or use the web dashboard's
  redeploy-at-SHA control once available.
