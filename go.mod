// infrix-playground: the hosted playground extracted from the Infrix monorepo
// (docs/extraction-plan). It is a THIN CLIENT — it asks an Infrix node to run a
// golden governed flow over /v4/playground/runFlow, then re-verifies the
// returned portable evidence package OFFLINE with the published verifier (no
// node trust). It depends only on the published infrix-schema / infrix-verify /
// infrix-nexus-web modules plus the Go standard library: no monorepo pkg/*, no
// live-node compile-time dependency.
module github.com/opendlt/infrix-playground

go 1.25.7

require (
	github.com/opendlt/infrix-nexus-web v0.1.0
	github.com/opendlt/infrix-schema v0.2.0
	github.com/opendlt/infrix-verify v0.2.0
)
