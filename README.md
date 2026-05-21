# fluid-pub/probe-confluence

Fluid probe for Atlassian Confluence Cloud: collects wiki **pages** and pushes entities to the control plane over WebSocket.

## Repository layout

| Path | Role |
|------|------|
| `core/` | Git submodule → [`fluid-pub/probe-core`](https://github.com/fluid-pub/probe-core) |
| `cmd/` | Entrypoint and `cmd/version.go` (semver for releases) |
| `internal/` | Confluence API client, entities, config |
| `config/probe.example.yml` | Configuration template |
| `config/schema.yml` | Entity schema (shipped in the Docker image; pushed to the control plane on connect) |
| `.github/workflows/` | CI and release via [`fluid-pub/actions`](https://github.com/fluid-pub/actions) |

## Local development

One-time per clone, enable the same **`gofmt`** check as CI:

```bash
./scripts/install-git-hooks.sh
```

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

Enroll as a probe with `agent_type: confluence`. Operational tuning (`data.entities`, `fields.*.rag`, intervals) belongs in **runtime_config** on the probe record. The schema contract is **`config/schema.yml`** (image semver); Kubernetes/GitOps should mount only bootstrap YAML (see **`fluid-workload`** `config.schemaInImage`).

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting. Repository automation includes Dependabot (`.github/dependabot.yml`) and CodeQL (`.github/workflows/codeql.yml`).
