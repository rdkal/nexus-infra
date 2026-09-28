# prometheus

## Purpose

Metrics for this host. Scrapes whatever the `targets` volume points it at, keeps
the series, and serves the query UI at `https://<SUBDOMAIN>.<COOKIE_DOMAIN>`
behind Authelia.

Nothing about *which* services exist lives in this project. That is deliberate
and is the whole reason it is generic: a project that wants to be scraped drops
one file into the `targets` volume and is picked up within thirty seconds, with
no redeploy of Prometheus and no edit to anything in this repo. It is the same
extension-point shape `traefik`'s `dynamic` volume already uses.

## Volumes

| Volume | Path (`$NEXUS_VOLUME_<NAME>`) | Contents | Extension point? |
|---|---|---|---|
| `bin` | `~/.nexus/volumes/prometheus/bin` | downloaded `prometheus` + `promtool` | no |
| `config` | `~/.nexus/volumes/prometheus/config` | rendered `prometheus.yml` | no — regenerated every deploy |
| `targets` | `~/.nexus/volumes/prometheus/targets` | **scrape target files, one per project** | **yes** |
| `data` | `~/.nexus/volumes/prometheus/data` | the TSDB | no |

`config` is regenerated on every deploy, unlike Authelia's `configuration.yml`
which is rendered once and then hand-edited. There is nothing here worth
hand-editing: the thing a person actually wants to change is which targets get
scraped, and that is the `targets` volume. Keeping the config generated means a
version bump or a changed default actually lands, instead of being pinned
forever to whatever the first deploy happened to write.

### How another project gets scraped (the extension point)

From the other project's `nexus.yaml`:

```yaml
environment:
  PROM_TARGETS_DIR: ${NEXUS_PROMETHEUS_TARGETS}   # resolved by nexus, no hardcoded path
build: |
  cat > "$PROM_TARGETS_DIR/my-project.yml" <<'EOF'
  - targets: ["127.0.0.1:8080"]
    labels:
      job: my-project
  EOF
```

`NEXUS_PROMETHEUS_TARGETS` is injected into every project once this one is
deployed (nexus's `NEXUS_<PROJECT>_<VOLUME>` convention). As with traefik,
**prometheus must be deployed before the consumer**, or the variable will not
exist yet. When this project is nested under a parent, the parent supplies the
nested address instead — `${NEXUS_<PARENT>_PROMETHEUS_TARGETS}`.

The file is Prometheus's own `file_sd` format: a list of target groups. Setting
`labels: {job: …}` names the job — **without it every drop-in piles up under
one job called `file_sd`**, which is almost never what you want. Any other
labels you set are attached to every metric from those targets.

Prometheus *watches* this directory. A new, changed or deleted file takes effect
within about thirty seconds — no redeploy, no restart, no reload signal. All
three claims here were checked against v3.15.0 rather than assumed: a dropped
file appeared as a live target, removing the file retired the target, and a
`job` label inside the file did override the `file_sd` job name. One
file per project reads best, because then removing the project removes its
targets. `~/.nexus/volumes/prometheus/targets/README.example` is written on
first deploy as a worked example (named so Prometheus's `*.yml`/`*.json` globs
skip it).

## Services & ports

| Service | Binds | Notes |
|---|---|---|
| `prometheus` | `127.0.0.1:9090` | loopback only — traefik fronts it, authelia guards it |

Loopback is not incidental. The Prometheus UI has no authentication of its own
and exposes every metric on the host, including anything a target leaks in a
label. Binding `0.0.0.0:9090` would publish all of it.

`--web.enable-lifecycle` is on, so `POST /-/reload` re-reads the config. The
`targets` volume does not need it (file_sd is watched), but a config change
does.

## Dependencies

- **traefik** — this project writes its own router fragment into traefik's
  `dynamic` volume, the handshake documented in `../traefik/README.md`. Without
  `TRAEFIK_DYNAMIC_DIR` set the build warns and skips the publish; Prometheus
  still runs, just only on loopback.
- **authelia** — the published route references the `authelia@file` middleware.
  If authelia is not deployed, traefik will reject the router as referencing an
  unknown middleware and the host will not serve. Deploy authelia first.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `COOKIE_DOMAIN` | — **required** | domain the UI is served under, e.g. `example.com` |
| `SUBDOMAIN` | — **required** | subdomain, e.g. `prometheus` |
| `PROM_RETENTION_TIME` | `90d` | how long series are kept |
| `PROM_RETENTION_SIZE` | `8GB` | hard cap on the TSDB |
| `PROM_SCRAPE_INTERVAL` | `15s` | default scrape and evaluation interval |

**Both retention limits are enforced, whichever binds first.** Prometheus will
not police the disk on your behalf: with neither set it keeps fifteen days and
whatever number of bytes that turns out to be. The size cap is the one that
stops a metrics volume from becoming a disk-full incident, so it has a default
rather than being optional.

## Deploy / rollback notes

- **The build validates before the service starts.** `promtool check config`
  runs at the end of the build step, so a malformed config fails the deploy
  instead of crash-looping the service.
- **The binary is downloaded once** and reused on later deploys — bumping
  `VERSION` in `nexus.yaml` is not enough on its own, because the build skips
  the download when `$NEXUS_VOLUME_BIN/prometheus` already exists. Delete that
  file to force a re-download.
- **The TSDB survives a redeploy** (it is in its own volume). To start clean,
  stop the service and empty `$NEXUS_VOLUME_DATA`.
- **Rolling back** is a redeploy at an older ref. The data volume format is
  stable within a Prometheus major version; going *backwards* across a major
  may refuse to open the TSDB, in which case the series have to be dropped.
- **Nothing here is a secret**, so there is no `prometheus.env` to lose. The
  two required variables are a hostname, and the deploy fails loudly if either
  is missing rather than rendering a config with an empty host in it.
