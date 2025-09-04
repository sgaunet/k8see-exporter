[![GitHub release](https://img.shields.io/github/release/sgaunet/k8see-exporter.svg)](https://github.com/sgaunet/k8see-exporter/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/sgaunet/k8see-exporter)](https://goreportcard.com/report/github.com/sgaunet/k8see-exporter)
![GitHub Downloads](https://img.shields.io/github/downloads/sgaunet/k8see-exporter/total)
![Coverage Badge](https://raw.githubusercontent.com/wiki/sgaunet/k8see-exporter/coverage-badge.svg)
[![License](https://img.shields.io/github/license/sgaunet/k8see-exporter.svg)](LICENSE)

# k8see (Kubernetes Events Exporter)

Kubernetes Events Exporter is a suit of three tools to export kubernertes events in an external database. The goal is to get events in an SQL DB to be able to analyze what happened.

The 3 tools are :

* [k8see-exporter](https://github.com/sgaunet/k8see-exporter) : Deployment inside the kubernetes cluster to export events in a redis stream
* [k8see-importer](https://github.com/sgaunet/k8see-importer) : Tool that read the redis stream to import events in a database (PostGreSQL)
* [k8see-webui](https://github.com/sgaunet/k8see-webui) : Web interface to query the database
* [k8see-deploy](https://github.com/sgaunet/k8see-deploy) : kubernetes manifests to deploy k8see-exporter and also the whole procedure to deploy a full test environment in a k8s cluster (with kind).

# k8see-exporter

The image is on docker hub: sgaunet/k8see-exporter:**version**

The manifests to deploy the application are in [k8see-deploy](https://github.com/sgaunet/k8see-deploy/tree/main/manifests/k8see-exporter)

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