# Engineering Standards

Design, code quality, testing, and decision-making rules for this project. All contributors follow these.

---

## Design

- **Readability first; performance when it matters.** If a less-readable path has significant performance gains, take it, but document why with a comment explaining the tradeoff.
- **Purist design over quick fixes.** Invest in correct abstractions upfront. Architectural debt that compounds should be prevented, not queued.
- **Accept interfaces, hand in implementations.** Depend on the smallest interface that does the job. `New` takes its dependencies from the caller (`WithHTTPClient` hands in a configured `*http.Client`); it does not construct them across layers.
- **Own the blast radius of your changes.** When you change something, know the full ripple and address it in the same PR. Don't leave broken dependents "to fix later" without an explicit decision to do so.

## Code Quality

- **Godoc on all exports.** Every exported function, type, method, and constant gets a doc comment that states purpose and contract, not a walkthrough of the code. Start with the identifier name and use `[Symbol]` links so it renders well on pkg.go.dev. Exactly one package comment per package, in [`doc.go`](doc.go).

  ```go
  // WithTimeout bounds each request to d. It panics on a duration of zero or
  // less, which is a programming error.
  func WithTimeout(d time.Duration) Option
  ```

- **Comments explain WHY, not WHAT.** Only comment when the next developer might not understand why you didn't do the obvious thing. Explain constraints, workarounds, hidden dependencies, not logic flow.

  ```go
  // Clone before mutating: RoundTrip must not modify the caller's request.
  r = r.Clone(r.Context())
  ```

- **Zero trust at public boundaries.** Validate inputs on the public API and responses from external services. Trust calls between internal packages. Fail fast with clear errors at boundaries.
- **Errors carry context and wrap their cause.** Return errors; reserve panics for documented programmer errors (a malformed constant baseURL in `New`, for example). Wrap with `fmt.Errorf("...: %w", err)` so callers can use `errors.Is` and `errors.As`. Error strings are lowercase with no trailing punctuation.
- **gofmt and go vet, nothing else.** `gofmt -l .` must report no first-party files and `go vet ./...` must pass; CI gates both. The project deliberately adds no third-party linter, so don't hand-roll style rules beyond what those tools enforce.

## Testing

- **Order: Contract → Tests → Implementation.** Write the doc comment first, tests second, implementation third. Tests are specs, not afterthoughts.
- **One responsibility per function.** Single purpose, a contract stated in its doc comment, no side effects unless that is the function's entire point.
- **Test behavior, not coverage percentages.** Each test verifies a specific path: happy path, bad path, or defensive guard. Skip tests that add no value.
- **Separate test types strictly.**
  - **Unit:** in-package tests; exercise functions in isolation
  - **Integration:** external test packages (`package ezclassify_test`) that import only the public API; real types, no stubbing of internal packages
  - **Live-server:** run against a real rest-api (`make smoke`, `make conformance-server`), never against the in-process replayer
- **Bug fixes require regression tests.** Every fix needs a test that fails before the fix and passes after. No exceptions.

## Decision-Making

- **Stable contracts, incremental changes.** Preserve published API compatibility. Prefer incremental, reversible changes and follow semantic versioning for breaking changes.
- **Inline small debt, backlog large debt.** Found debt under 30 min effort: fix inline. Larger refactors, dependency upgrades, or architectural changes: log to backlog, don't auto-execute.
- **Don't let tasks balloon.** If you discover work that significantly exceeds the original scope, stop, log it, and stay on scope. Surface it for explicit prioritization.
