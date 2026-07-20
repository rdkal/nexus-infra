# authelia

## Purpose

Authentication/forward-auth gateway. Sits behind Traefik as a `forwardAuth` middleware
target — see `../traefik/README.md` for how a project wires a route through it. This
project only runs Authelia itself; it carries no opinions about which domains it protects
(that's `access_control`, deliberately left for you to fill in — see below).

## Volumes

| Volume | Path (`$NEXUS_VOLUME_<NAME>`) | Contents | Extension point? |
|---|---|---|---|
| `bin` | `~/.nexus/volumes/authelia/bin` | downloaded `authelia` binary | no |
| `config` | `~/.nexus/volumes/authelia/config` | `configuration.yml`, `users_database.yml` | **yes, by hand** |
| `data` | `~/.nexus/volumes/authelia/data` | sqlite session/history db, notification file | no, but must be backed up |

### `config` is seeded once, then yours to edit

`configuration.yml` and `users_database.yml` are rendered/copied on the **first** deploy
only — later deploys never overwrite them (see the `nexus.yaml` build step). After first
deploy:

1. Edit `~/.nexus/volumes/authelia/config/configuration.yml` — add your `access_control`
   rules (which domains, which policy). The template ships `default_policy: deny` and an
   empty rule list, so nothing is reachable until you add rules.
2. Edit `~/.nexus/volumes/authelia/config/users_database.yml` — replace the example user.
   Generate a hash with `authelia crypto hash generate argon2 --password '...'`.
3. Restart the service (`nexus` will pick up config changes on next redeploy/restart —
   Authelia itself doesn't hot-reload `access_control` changes without a restart).

## Services & ports

| Service | Binds |
|---|---|
| `authelia` | `127.0.0.1:9091` (behind Traefik only, never exposed directly) |

## Dependencies

- **Valkey/Redis** on `127.0.0.1:6379` for session storage. Not part of this project —
  expected to already be running on the host.
- Commonly paired with `../traefik`, which forwards auth-required routes to this service.

## Required environment (secrets — see "Known limitation")

- `AUTHELIA_JWT_SECRET` — reset-password JWT signing secret.
- `AUTHELIA_SESSION_SECRET` — session cookie secret.
- `AUTHELIA_STORAGE_ENCRYPTION_KEY` — sqlite storage encryption key.
- `AUTHELIA_COOKIE_DOMAIN` — the root domain the session cookie applies to (e.g. `example.com`).
- `AUTHELIA_SUBDOMAIN` — the subdomain Authelia itself is served on (e.g. `auth` for `auth.example.com`).

## Known limitation: secrets are out-of-band (same as traefik)

Nexus has no built-in secret management yet. The four `AUTHELIA_*` values above must be
present in the **nexus daemon's own environment** (see `../traefik/README.md`'s "Known
limitation" section for the exact mechanism) so the `authelia` service inherits them at
build/run time. They are not stored in this repo.

## Deploy / rollback notes

- `data` must be backed up — it's session history and the encrypted storage db.
- `config` should be backed up too, once you've added real `access_control` rules and
  users — it will NOT be regenerated for you.
- `bin` can be wiped safely; rebuilt on next deploy if missing.
