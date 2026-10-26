# Contributing to EzClassify Go SDK

Thanks for your interest in contributing. The short version: sign off your
commits, fork, open an issue before anything non-trivial, and bring tests. Environment
setup and build mechanics live in [DEVELOPMENT.md](./DEVELOPMENT.md): the
toolchain, the build and test commands, the regen and spec-vendoring workflow,
and running conformance.

## Before You Start

1. **Sign off your commits.** Contributions come in under the Developer
   Certificate of Origin, so every commit carries a `Signed-off-by` line.
   `git commit -s` adds it. There is no agreement to sign and nothing to send:
   see the [DCO text](https://github.com/ezclassify/ezclassify/blob/main/DCO.md) for what the line certifies.
2. **Fork the repository.** All contributions come through pull requests from
   forks.
3. **Open an issue first** for non-trivial changes, so we can agree on the
   approach before you invest the work.

## Pull Requests

- **Bring tests.** New behavior needs tests; bug fixes need a regression test
  that fails without the fix. `go test ./...` runs the suite and CI runs the
  same gate.
- **Follow the engineering standards** in [STANDARDS.md](STANDARDS.md) for
  design, code quality, and testing. Read it before significant work.
- Keep pull requests focused; a reviewer should be able to hold the whole change
  in their head.

## Commit Messages

Conventional commits (`fix:`, `feat:`, `docs:`). Go releases are git tags
(`v<version>`); [`go.mod`](go.mod) carries no version field.

## Reporting Security Issues

Report vulnerabilities privately per the [security policy](https://github.com/ezclassify/ezclassify/blob/main/SECURITY.md), never in a
public issue.

## Code of Conduct

This project follows our [Code of Conduct](https://github.com/ezclassify/ezclassify/blob/main/CODE_OF_CONDUCT.md). By
participating, you are expected to uphold it.
