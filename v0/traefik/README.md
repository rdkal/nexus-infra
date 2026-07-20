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
redeploy of traefik itself needed. To expose a service:

1. Drop a `<your-project>.yml` file into `~/.nexus/volumes/traefik/dynamic/` (this is a
   fixed host path today — see "Known limitation" below).
2. Define a `router` (host rule, entrypoint `websecure`, `certResolver: letsencrypt`) and
   a `service` pointing at `http://127.0.0.1:<your-port>`.
3. If the route needs auth, add the `authelia` forward-auth middleware — see
   `../authelia/README.md` for its address and how to reference it.

This mirrors what `~/.config/traefik/dynamic/routes.yml` did in the pre-Nexus setup —
that file's routers are a working reference for the shape a fragment should take.

## Services & ports

| Service | Binds |
|---|---|
| `traefik` | `:443` (public) |

## Dependencies

- None required to start. Individual routes in `dynamic/` will 502 until their target
  service is actually listening — that's expected, not an error state for traefik itself.
- Commonly paired with `../authelia` for routes that need auth.

## Required environment (secrets — see "Known limitation")

- `ACME_EMAIL` — Let's Encrypt account email.
- `CF_DNS_API_TOKEN` — Cloudflare API token, scoped to DNS edit, for the DNS-01 challenge.
  Read directly by the traefik/lego process, not by this repo.

## Known limitation: secrets and cross-project paths are both out-of-band

Nexus has no built-in secret management or cross-project volume-path injection yet (see
nexus's own DESIGN.md, "Explicitly deferred" / "Open Questions"). Until that lands:

- `ACME_EMAIL` and `CF_DNS_API_TOKEN` must be present in the **nexus daemon's own
  environment** so the `traefik` service inherits them (e.g.
  `systemctl --user edit nexus.service` and add `Environment=` lines, then
  `systemctl --user restart nexus`). They are not stored in this repo.
- The `dynamic` volume's absolute path (`~/.nexus/volumes/traefik/dynamic`) is a
  convention documented here, not something Nexus resolves for you — other projects'
  build/deploy steps must know this literal path.

## Deploy / rollback notes

- `acme` is the only volume that must be backed up — losing it means re-issuing every
  certificate (rate-limited by Let's Encrypt).
- `bin` and `config` can be wiped safely; they're rebuilt on the next deploy.
- Rolling back: `nexus project add` a previous tag/ref, or use the web dashboard's
  redeploy-at-SHA control once available.
