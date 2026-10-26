# go-classify

A runnable Go CLI that scans a directory of text files for marking strings,
validates each marking against your runtime, and produces a per-file
classification report.

Each file goes through two Context-bound calls: a scan that extracts and
validates every marking string, and an aggregate that computes the file's
overall sensitivity level.

## You need a runtime

This example sends the text of your files, which the Cedar Valley replay server
has no recording for. Against the replay server a scan returns a 501
diagnostic, so use a runtime that serves the Context you scan with. To see the
Go client working before you have one, run [`examples/basic`](../basic/README.md)
or [`examples/server-tour`](../server-tour/README.md) against the replay server.

The Context is `cedar-valley-protection`, a community-content Context. You need
Go 1.26.10 or newer.

## Run it against your runtime

Your runtime administrator gives you the runtime URL and, for a secured
deployment, two credentials: a caller token and a signed Context token. A
runtime started on your machine listens on `http://localhost:3000`, which is
the default for `--server`. Export the tokens if your deployment asks for them:

```sh
export EZC_ACCESS_TOKEN=<caller identity token>
export EZC_CONTEXT_TOKEN=<signed Context token>
```

From the repository root, scan the sample directory:

```sh
go run ./examples/go-classify --server https://runtime.example.com --dir ./examples/go-classify/sample
```

Scan your own directory, or bind a different published Context:

```sh
go run ./examples/go-classify --server https://runtime.example.com --dir /path/to/docs
go run ./examples/go-classify --server https://runtime.example.com --ctx <your-context-id> --dir ./examples/go-classify/sample
```

## Example output

```
=== EzClassify File Scan Report ===
Server: http://localhost:3000  Context: cedar-valley-protection@latest  Dir: ./examples/go-classify/sample

FILE                                    MATCHES  VALID  INVALID  AGGREGATE
----                                    -------  -----  -------  ---------
examples/go-classify/sample/README.md   3        3      0        RESTRICTED
examples/go-classify/sample/sample.txt  5        5      0        RESTRICTED

Summary: 2 file(s) scanned, 8 marking(s) found (8 valid, 0 invalid)
```

## Flags

| Flag        | Default                   | Description                        |
|-------------|---------------------------|------------------------------------|
| `--server`  | `http://localhost:3000`   | EzClassify runtime base URL        |
| `--ctx`     | `cedar-valley-protection` | Context ID to use for scanning     |
| `--ctx-version` | `latest`               | Context version to bind            |
| `--dir`     | `.`                       | Directory to scan (`.txt`/`.md`)   |
| `--timeout` | `30s`                     | HTTP request timeout (positive)    |
