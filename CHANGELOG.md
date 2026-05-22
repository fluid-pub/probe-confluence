# Changelog — probe-confluence

All notable changes to **fluid-pub/probe-confluence** are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Tag naming: `0.y.z` (no `v` prefix). Align `cmd/version.go` with the tag before release.

## [Unreleased]

## [0.1.3] - 2026-05-22

### Changed

- Control plane transport is HTTP only: **`POST /probes/register`**, **`/probes/ping`**, **`/probes/v1/ingest`** via **`probe-core`** (compatible with control plane 0.5+). Legacy `controlplane.websocket_url` / `FLUID_CONTROLPLANE_WEBSOCKET_URL` is still accepted; **`probe-core`** derives the HTTP base URL when `base_url` is unset.
- Documentation and examples prefer `FLUID_CONTROLPLANE_HTTP_BASE` / `controlplane.base_url`; empty optional `base_url` env no longer disables the whole control plane block.

### Fixed

- `internal/config`: resolving optional `base_url` from the environment no longer clears control plane config when the variable is unset (WebSocket URL fallback remains valid).

## [0.1.2] - 2026-05-21

### Changed

- Confluence REST access uses the standard library **`net/http`** client (removed dependency on a separate Confluence SDK).
- **`probe-core`** bump: reliable **`schema.yml`** resolution at `/etc/fluid/config/schema.yml` in cluster deployments.

## [0.1.1] - 2026-05-21

### Added

- Ship **`config/schema.yml`** in the container image so Helm can use **`fluid-workload`** `config.schemaInImage` (ConfigMap subPath on `config.yaml` only).

## [0.1.0] - 2026-05-21

### Added

- Initial Confluence Cloud probe: collect wiki **pages**, persist local state, connect to the Fluid control plane.
- Semver release workflow publishing **`ghcr.io/fluid-pub/probe-confluence:<tag>`** and GitHub Release binary **`confluence-probe-linux-amd64`** (+ `SHA256SUMS.txt`).
- Configuration templates, entity schema, and Dependabot / CodeQL automation.
