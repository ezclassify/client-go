# server-tour

Lists the Contexts a server publishes, then processes a marking through one of
them and prints the result and the resolved version.

Cedar Valley is a fictional school district policy for teaching. Use only
synthetic content with it.

## Start with the Cedar Valley replay server

The replay server answers over real HTTP with responses recorded from a real
runtime for `cedar-valley@1.0.0`. You need Go 1.26.10 or newer, and Node.js 22
or 24 for the server only. You do not need a runtime, a license or a credential.

```sh
npx @ezclassify/conformance-server --scenario cedar-valley --port 4010
```

Leave the server running. In a second terminal, from the repository root:

```sh
go run ./examples/server-tour
```

The command defaults to `http://127.0.0.1:4010`, Context `cedar-valley`,
version `1.0.0`, and the marking `CONFIDENTIAL`. It prints the Context list,
then the process response as JSON:

```text
=== Available Contexts ===
  cedar-valley

=== cedar-valley@1.0.0 process("CONFIDENTIAL") ===
{
  ...
    "rendered": "CONFIDENTIAL",
    "renders": {},
    "valid": true,
    "warnings": []
  }
}
Resolved version: 1.0.0
```

The replay server answers a fixed set of recorded requests. `PUBLIC`,
`INTERNAL` and `CONFIDENTIAL` all work with `--marking`; any other marking gets
a 501 diagnostic, not an invented answer. If port 4010 is taken, start the
server on another port and pass `--server` to match.

## Switch to your runtime

Same code. Point it at your runtime with flags, and give it the credentials
your runtime administrator issued. `EZC_ACCESS_TOKEN` identifies the caller and
`EZC_CONTEXT_TOKEN` is the signed Context credential. Leave either out if your
deployment does not require it.

```sh
export EZC_ACCESS_TOKEN=<caller identity token>
export EZC_CONTEXT_TOKEN=<signed Context token>
go run ./examples/server-tour --server https://runtime.example.com --ctx cedar-valley --ctx-version 1.0.0 --marking "<a marking your Context accepts>"
```

A runtime evaluates any marking, so you are no longer limited to what the
replay server recorded. Keep tokens out of source control and shell history; in
your own application, read them from your secret store.

| Flag            | Default                 | Description                  |
|-----------------|-------------------------|------------------------------|
| `--server`      | `http://127.0.0.1:4010` | EzClassify REST API base URL |
| `--ctx`         | `cedar-valley`          | Context ID to use            |
| `--ctx-version` | `1.0.0`                 | Context version to bind      |
| `--marking`     | `CONFIDENTIAL`          | Marking string to process    |
