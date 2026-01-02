[![GitHub release](https://img.shields.io/github/release/sgaunet/k8see-exporter.svg)](https://github.com/sgaunet/k8see-exporter/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/sgaunet/k8see-exporter)](https://goreportcard.com/report/github.com/sgaunet/k8see-exporter)
![GitHub Downloads](https://img.shields.io/github/downloads/sgaunet/k8see-exporter/total)
![Coverage Badge](https://raw.githubusercontent.com/wiki/sgaunet/k8see-exporter/coverage-badge.svg)
[![linter](https://github.com/sgaunet/k8see-exporter/actions/workflows/coverage.yml/badge.svg)](https://github.com/sgaunet/k8see-exporter/actions/workflows/coverage.yml)
[![coverage](https://github.com/sgaunet/k8see-exporter/actions/workflows/coverage.yml/badge.svg)](https://github.com/sgaunet/k8see-exporter/actions/workflows/coverage.yml)
[![Snapshot Build](https://github.com/sgaunet/k8see-exporter/actions/workflows/snapshot.yml/badge.svg)](https://github.com/sgaunet/k8see-exporter/actions/workflows/snapshot.yml)
[![Release Build](https://github.com/sgaunet/k8see-exporter/actions/workflows/release.yml/badge.svg)](https://github.com/sgaunet/k8see-exporter/actions/workflows/release.yml)
[![License](https://img.shields.io/github/license/sgaunet/k8see-exporter.svg)](LICENSE)

# k8see (Kubernetes Events Exporter)

Kubernetes Events Exporter is a suit of three tools to export kubernertes events in an external database. The goal is to get events in an SQL DB to be able to analyze what happened.

The 3 tools are :

* [k8see-exporter](https://github.com/sgaunet/k8see-exporter) : Deployment inside the kubernetes cluster to export events in a redis stream
* [k8see-importer](https://github.com/sgaunet/k8see-importer) : Tool that read the redis stream to import events in a database (PostGreSQL)
* [k8see-webui](https://github.com/sgaunet/k8see-webui) : Web interface to query the database
* [k8see-deploy](https://github.com/sgaunet/k8see-deploy) : kubernetes manifests to deploy k8see-exporter and also the whole procedure to deploy a full test environment in a k8s cluster (with kind).

# k8see-exporter

The image is available on GitHub Container Registry: ghcr.io/sgaunet/k8see-exporter:**version**

## Deployment

The application can be deployed using:

* **Helm Chart**: [helm-k8see](https://github.com/sgaunet/helm-k8see/)
* **Kubernetes Manifests**: [k8see-deploy](https://github.com/sgaunet/k8see-deploy/tree/main/manifests/k8see-exporter)

## RBAC Requirements

k8see-exporter requires **cluster-wide read permissions** for Kubernetes events.

### Required Permissions

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: k8see-exporter
  namespace: k8see
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8see-exporter
rules:
- apiGroups: [""]
  resources: ["events"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8see-exporter
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: k8see-exporter
subjects:
- kind: ServiceAccount
  name: k8see-exporter
  namespace: k8see
```

### Why Cluster-Wide?

k8see-exporter monitors events across **all namespaces** to provide comprehensive cluster observability. It uses a Kubernetes informer that watches the entire cluster.

### Security Considerations

- **Read-only access**: Only requires `get`, `list`, `watch` (no write permissions)
- **Events only**: No access to Pods, Secrets, or other sensitive resources
- **Standard pattern**: Same permissions as `kubectl get events --all-namespaces`

> **Note**: If using Helm deployment, RBAC resources are automatically created. Manual deployments require creating these resources.

# Development environment

[The repository k8see-deploy](https://github.com/sgaunet/k8see-deploy) contains the whole procedure to deploy a full test environment in a k8s cluster (with kind). The repository contains also themanifests to deploy k8see-exporter.

# Build

This project is using :

* golang 1.17+
* [task for development](https://taskfile.dev/#/)
* docker
* [docker buildx](https://github.com/docker/buildx)
* docker manifest
* [goreleaser](https://goreleaser.com/)


## Binary 

```
task
```

## Build the image

```
task image
```

# Make a release

```
task release
```

## Project Status

🟨 **Maintenance Mode**: This project is in maintenance mode.

While we are committed to keeping the project's dependencies up-to-date and secure, please note the following:

- New features are unlikely to be added
- Bug fixes will be addressed, but not necessarily promptly
- Security updates will be prioritized

## Issues and Bug Reports

We still encourage you to use our issue tracker for:

- 🐛 Reporting critical bugs
- 🔒 Reporting security vulnerabilities
- 🔍 Asking questions about the project

Please check existing issues before creating a new one to avoid duplicates.

## Contributions

🤝 Limited contributions are still welcome.

While we're not actively developing new features, we appreciate contributions that:

- Fix bugs
- Update dependencies
- Improve documentation
- Enhance performance or security

If you're interested in contributing, please read our [CONTRIBUTING.md](link-to-contributing-file) guide for more information on how to get started.

## Support

As this project is in maintenance mode, support may be limited. We appreciate your understanding and patience.

Thank you for your interest in our project!