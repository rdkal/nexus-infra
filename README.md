# nexus-infra

Platform infrastructure for this host, deployed via [Nexus](https://github.com/rdkal/nexus)
instead of hand-maintained systemd units. This repo is scoped to the pieces that make the
*server itself* work (edge proxy, auth gateway, ...) — not application-level projects.

## Layout

```
v0/
  traefik/
  authelia/
```

Each top-level project lives under a `v0/<name>/` directory and is added independently:

```sh
nexus project add github.com/rdkal/nexus-infra/v0/traefik
nexus project add github.com/rdkal/nexus-infra/v0/authelia
```

### Why the `v0/` prefix

Nexus itself is pre-1.0 and this repo's design (how volumes are laid out, how projects
compose, how services find each other) is still being worked out. `v0/` marks "current
approach, expect breaking changes." If the design needs a breaking change later, it moves
to `v1/traefik` etc. alongside — `v0` keeps running untouched on whatever ref currently
tracks it until you deliberately migrate. Nothing about a project's name lives inside its
`nexus.yaml` (Nexus assigns names at `add`/nest time), so this is purely a path convention,
not something Nexus needs to know about.

## Documentation standard

Every project directory has a `README.md` with these sections:

- **Purpose** — one paragraph, what it does and why it's here.
- **Volumes** — each volume's name, on-disk path, what belongs in it, and — critically —
  whether *other* projects are expected to write into it (an "extension point"). If so,
  say exactly what to drop there and in what format.
- **Services & ports** — every long-running process the project starts and what port(s)
  it binds.
- **Dependencies** — other projects/services this one expects to be running, and why.
- **Deploy / rollback notes** — anything non-obvious about bringing it up, tearing it
  down, or rolling back a bad deploy.

See `v0/traefik/README.md` for a worked example of documenting an extension point (its
`dynamic` volume, where other projects add their own routes).
