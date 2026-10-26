// This file holds the go:generate directive that regenerates the typed HTTP
// client + wire DTOs (internal/api/api.gen.go) from the EzClassify OpenAPI
// spec via oapi-codegen. The package doc lives in doc.go.
//
// To regenerate after an engine release, run `make regen` (preferred) or:
//
//	go generate ./...
//
// The directive below uses the same checked OpenAPI 3.1 adapter and pinned
// generator as make regen. No prior generator install is needed. The manifest
// records the generator version and module hash, and CI verifies both.
//
//go:generate bash scripts/regen.sh

package ezclassify
