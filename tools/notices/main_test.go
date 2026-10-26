package main

import (
	"strings"
	"testing"
)

func TestRenderIncludesDependencyNotices(t *testing.T) {
	mods := []module{
		{
			path: "github.com/oapi-codegen/runtime", version: "v1.7.0",
			license: "Apache-2.0", licenseFile: "vendor/github.com/oapi-codegen/runtime/LICENSE",
			copyright: "Copyright the oapi-codegen authors",
		},
		{
			path: "example.com/unnoted", version: "v0.1.0",
			license: "ISC", licenseFile: "vendor/example.com/unnoted/LICENSE",
			copyright: "Copyright the unnoted authors",
		},
	}
	got := render(mods)
	for _, want := range []string{
		"Apache License 2.0 (see `LICENSE`).",
		"| `github.com/oapi-codegen/runtime` | v1.7.0 | Apache-2.0 | Copyright the oapi-codegen authors |",
		"| `example.com/unnoted` | v0.1.0 | ISC | Copyright the unnoted authors |",
		"### github.com/oapi-codegen/runtime\n\nThe generated HTTP client (`internal/api`) uses this module",
		"Licensed under the **Apache License, Version 2.0** (see `vendor/github.com/oapi-codegen/runtime/LICENSE`).",
		"### example.com/unnoted\n\nProduction dependency of the SDK.",
		"Licensed under the **ISC License** (see `vendor/example.com/unnoted/LICENSE`).",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered notices missing %q", want)
		}
	}
}

// TestParseClosureWellFormed pins the happy path: `path version` lines are
// deduplicated, the own module is excluded, and the result is sorted by path.
func TestParseClosureWellFormed(t *testing.T) {
	out := ownModule + "\n" +
		"github.com/google/uuid v1.5.0\n" +
		"github.com/oapi-codegen/runtime v1.1.1\n" +
		"github.com/google/uuid v1.5.0\n"
	mods, err := parseClosure(out)
	if err != nil {
		t.Fatalf("parseClosure: %v", err)
	}
	if len(mods) != 2 {
		t.Fatalf("got %d modules, want 2 (deduplicated, own module excluded)", len(mods))
	}
	if mods[0].path != "github.com/google/uuid" || mods[1].path != "github.com/oapi-codegen/runtime" {
		t.Errorf("modules not sorted by path: %v, %v", mods[0].path, mods[1].path)
	}
	if mods[0].version != "v1.5.0" {
		t.Errorf("version = %q, want v1.5.0", mods[0].version)
	}
}

// TestParseClosureVersionlessModuleFails pins the fail-closed contract: a
// dependency line without a version (what a local-path replace produces) must
// error so the module cannot escape the license gate.
func TestParseClosureVersionlessModuleFails(t *testing.T) {
	out := "github.com/google/uuid v1.5.0\n" +
		"example.com/local-replace\n"
	_, err := parseClosure(out)
	if err == nil {
		t.Fatal("expected an error for a version-less module line, got nil")
	}
	if !strings.Contains(err.Error(), "example.com/local-replace") {
		t.Errorf("error %q does not name the offending line", err)
	}
}

// TestParseClosureOwnModuleIsExempt pins that the audited module's own
// version-less line stays exempt from the shape check.
func TestParseClosureOwnModuleIsExempt(t *testing.T) {
	mods, err := parseClosure(ownModule + "\n")
	if err != nil {
		t.Fatalf("parseClosure: %v", err)
	}
	if len(mods) != 0 {
		t.Errorf("got %d modules, want 0", len(mods))
	}
}
