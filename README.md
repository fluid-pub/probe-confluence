# fluid-pub/probe-confluence

Fluid probe for Atlassian Confluence Cloud: collects wiki **pages** and pushes entities to the control plane over WebSocket.

## Repository layout

| Path | Role |
|------|------|
| `core/` | Git submodule → [`fluid-pub/probe-core`](https://github.com/fluid-pub/probe-core) |
| `cmd/` | Entrypoint and `cmd/version.go` (semver for releases) |
| `internal/` | Confluence API client, entities, config |
| `config/probe.example.yml` | Configuration template |
| `.github/workflows/` | CI and release via [`fluid-pub/actions`](https://github.com/fluid-pub/actions) |

## Local development

```bash
git submodule update --init --recursive
cp config/probe.example.yml config/probe.yml
cp env.secrets.example env.secrets
# Set your Atlassian and control plane values in env.secrets (never commit that file).
# Optionally adjust URLs in config/probe.yml (also gitignored).
source env.secrets
make dev
```

All credentials and tenant URLs come from **your** `env.secrets` and local `config/probe.yml` only — nothing customer-specific is stored in this repository.

`make dev` runs `go run ./cmd` with `-config config/probe.yml`. Runtime snapshots go under `state/` (gitignored).

## Releases

Push a semver tag **without** `v` (e.g. `0.1.0`) matching `var Version` in `cmd/version.go`. The release workflow publishes:

- `ghcr.io/fluid-pub/probe-confluence:<tag>`
- GitHub Release asset `confluence-probe-linux-amd64` and `SHA256SUMS.txt`

Tag creation on this public repository is restricted to the org **`release-managers`** team (see [`infrastructure/github`](https://github.com/fluid-pub/infrastructure)).

## Control plane

Enroll as a probe with `agent_type: confluence`. Set `wiki_base_url` in runtime config when the control plane should build wiki links for indexed pages.

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting. Repository automation includes Dependabot (`.github/dependabot.yml`) and CodeQL (`.github/workflows/codeql.yml`).
