# Agent Instructions

## Standards

Read [`STANDARDS.md`](./STANDARDS.md) before any feature, refactor, architectural decision, or task > 30 minutes. These engineering standards apply to you as they do to any contributor. Skip for: quick bug fixes, typos, documentation-only changes.

Read [`ARCHITECTURE.md`](./ARCHITECTURE.md) before any architectural work. It is
the repository's ownership contract for generated code, handwritten seams,
dependency direction, and required boundary gates.

## Examples

Runnable examples live under [`examples/`](examples/) and are part of this module (`go build ./...` covers them).

- **[`examples/basic/`](examples/basic/)**: demonstrates validate, aggregate, and access-check through one published Context. Defaults to the Cedar Valley replay server (`--server http://127.0.0.1:4010 --ctx cedar-valley --ctx-version 1.0.0`). Run: `go run ./examples/basic`
- **[`examples/go-classify/`](examples/go-classify/)**: scans text files through a Context and produces a per-file classification report. Needs a runtime, since the replay server has no recording for arbitrary file text. Run: `go run ./examples/go-classify --server http://localhost:3000 --ctx cedar-valley-protection --dir ./examples/go-classify/sample`
- **[`examples/server-tour/`](examples/server-tour/)**: discovers available Contexts and drives a Context-bound process call. Defaults to the Cedar Valley replay server. Run: `go run ./examples/server-tour --marking "INTERNAL"`

Each example has a README with the replay server start command, expected output, and the switch to a runtime. All three read optional `EZC_ACCESS_TOKEN` and `EZC_CONTEXT_TOKEN` for a secured runtime.

Examples have no tests. They are built by `make build` and verified to compile on every CI run.
