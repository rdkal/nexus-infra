# authelia

## Purpose

Authentication/forward-auth gateway. Sits behind Traefik as a `forwardAuth` middleware
target. This project runs Authelia itself *and* publishes the wiring Traefik needs to
reach it — its own login-page route plus a reusable `authelia@file` middleware — into
Traefik's `dynamic` volume on every deploy (see "Publishes into traefik's dynamic
volume" below). It carries no opinions about which domains it protects (that's
`access_control`, deliberately left for you to fill in — see below).

**Requires `../traefik` to already be deployed** — this project's build step needs
`$NEXUS_TRAEFIK_DYNAMIC`, which only exists once traefik has deployed at least once.

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

### Publishes into traefik's `dynamic` volume (the extension-point handshake)

Every deploy renders `config/traefik-dynamic.yml.tmpl` to
`$NEXUS_TRAEFIK_DYNAMIC/authelia.yml`, which defines:

- a `router` for `${AUTHELIA_SUBDOMAIN}.${AUTHELIA_COOKIE_DOMAIN}` (Authelia's own
  login/portal page), and
- the `authelia` forward-auth `middleware`, reusable by any other project's route.

Other projects needing auth on a route just add `middlewares: [authelia@file]` to their
own dropped fragment — they never need to know Authelia's port or address.

## Services & ports

| Service | Binds |
|---|---|
| `authelia` | `127.0.0.1:9091` (behind Traefik only, never exposed directly) |

## Dependencies

- **Valkey/Redis** on `127.0.0.1:6379` for session storage. Not part of this project —
  expected to already be running on the host.
- Commonly paired with `../traefik`, which forwards auth-required routes to this service.

## Required environment

Set these in `$NEXUS_HOME/env/authelia.env` (operator file — not in git, persists across
deploys). `nexus.yaml` declares all of them as required: the deploy fails loudly at build
time if any is missing.

```sh
# ~/.nexus/env/authelia.env
AUTHELIA_JWT_SECRET=...              # reset-password JWT signing secret
AUTHELIA_SESSION_SECRET=...          # session cookie secret
AUTHELIA_STORAGE_ENCRYPTION_KEY=...  # sqlite storage encryption key
AUTHELIA_COOKIE_DOMAIN=example.com   # root domain the session cookie applies to
AUTHELIA_SUBDOMAIN=auth              # subdomain authelia is served on (auth.example.com)
```

Isolation is automatic: only this project's build/service processes ever see these
(nexus's per-project environment, not the daemon's full environment).

## Deploy / rollback notes

- `data` must be backed up — it's session history and the encrypted storage db.
- `config` should be backed up too, once you've added real `access_control` rules and
  users — it will NOT be regenerated for you.
- `bin` can be wiped safely; rebuilt on next deploy if missing.
