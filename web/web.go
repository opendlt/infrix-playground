// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

// Package web embeds the hosted-playground browser app (adoption-09): the
// landing page and SPA that drive the run/verify/replay/share experience. The
// shared modules it imports (the proof-receipt card, the client-side verifier,
// the Cinema core, the Nexus design tokens) are NOT duplicated here — the
// server serves those from the embedded Nexus asset tree, so there is one
// canonical implementation of each.
package web

import (
	_ "embed"
)

//go:embed index.html
var index []byte

//go:embed playground.js
var playgroundJS []byte

//go:embed playground.css
var playgroundCSS []byte

//go:embed bundleScene.js
var bundleSceneJS []byte

// File is one embedded playground asset.
type File struct {
	Body        []byte
	ContentType string
}

// Assets maps the request path to the embedded playground asset. The server
// serves these directly; everything else (/, SPA routes) falls back to
// index.html, and shared modules (/lib, /components, /cinema-core, /styles.css)
// are served from the Nexus asset tree.
func Assets() map[string]File {
	return map[string]File{
		"/playground.js":  {Body: playgroundJS, ContentType: "application/javascript; charset=utf-8"},
		"/playground.css": {Body: playgroundCSS, ContentType: "text/css; charset=utf-8"},
		"/bundleScene.js": {Body: bundleSceneJS, ContentType: "application/javascript; charset=utf-8"},
	}
}

// Index returns the SPA shell HTML.
func Index() []byte { return index }
