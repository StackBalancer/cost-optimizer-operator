# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Resource usage and recommendations are now tracked per container instead of
  per pod, so sidecars no longer distort a workload's sizing.
- `ResourceOptimizer` status reports one recommendation per container
  (`status.recommendations`) instead of a single combined recommendation.

### Fixed
- Devcontainer now provisions a k3d cluster for local development, so
  metrics-server is available out of the box.
- Collected CPU and memory usage per pod no longer accumulates across pods
  in a deployment, which previously scaled recommendations with replica count.
