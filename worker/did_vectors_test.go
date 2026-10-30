// Copyright 2026 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// THE SHARED did:infrix VECTOR SET (W.DID.CROSSSURFACE.PARITY), pinned in the playground's CI.
//
// testdata/did-vectors-v1.json is the exact output of `infrix did vectors --out` in infrix core. The same
// file (the same SHA-256) is pinned in infrix-sdk-js, infrix-schema and infrix-verify, and infrix core pins
// the digest of what it exports, so a change to the DID rules is a visible, simultaneous change in every
// repository.
//
// The playground is a thin client with no DID parser: it relays whatever the node answers and DISPLAYS example
// identifiers. Parity for such a surface is two things, both enforced here: the vector file is the one core
// exports, and every did:infrix identifier the playground shows or sends is one the vector set accepts (the
// canonical v1 grammar), never the retired acc:// spelling the node refuses (the playground used to show
// did:infrix:acc://alice.acme in three places).

const pinnedDIDVectorsSHA256 = "e25a465a066cbdbf746ca7abefed35d6d39e908120a2925b1844d761fc5e63cb"

type didVectorSet struct {
	Spec      string `json:"spec"`
	NetworkID string `json:"networkId"`
	Parse     []struct {
		Name      string `json:"name"`
		Input     string `json:"input"`
		Valid     bool   `json:"valid"`
		Canonical string `json:"canonical"`
	} `json:"parse"`
	Construct []struct {
		Name string `json:"name"`
	} `json:"construct"`
}

func loadDIDVectors(t *testing.T) (didVectorSet, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "did-vectors-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var set didVectorSet
	if err := json.Unmarshal(raw, &set); err != nil {
		t.Fatal(err)
	}
	return set, raw
}

func TestDIDVectorFileIsTheOneInfrixCoreExports(t *testing.T) {
	set, raw := loadDIDVectors(t)
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != pinnedDIDVectorsSHA256 {
		t.Fatalf("did vectors sha256 = %s, pinned %s: re-export with `infrix did vectors --out` and update every repository together", got, pinnedDIDVectorsSHA256)
	}
	if set.Spec != "did-infrix-v1" || len(set.Parse) < 26 || len(set.Construct) < 3 {
		t.Fatalf("vector set shape changed: spec %q, %d parse, %d construct", set.Spec, len(set.Parse), len(set.Construct))
	}
	edited := append(append([]byte(nil), raw...), ' ')
	if s := sha256.Sum256(edited); hex.EncodeToString(s[:]) == pinnedDIDVectorsSHA256 {
		t.Fatal("PLANTED DEFECT NOT DETECTED: an edited vector file kept the pinned digest")
	}
}

var didLiteral = regexp.MustCompile(`did:(?:infrix|lattica):[^\s'"` + "`" + `<>)]*`)

// canonicalInfrixDID is the v1 grammar's three registered profiles; a literal that is not in this shape and is not a
// vector the set accepts is not an identifier the node resolves.
var canonicalInfrixDID = regexp.MustCompile(`^did:infrix:v1:(acc|anon|chain):`)

// Every did:infrix identifier written in the playground's sources is either a valid canonical vector shape bound to
// the vector set's network id, or the bare method name in prose (no identifier after it). The retired spellings fail.
func TestEveryIdentifierThePlaygroundShowsIsCanonical(t *testing.T) {
	set, _ := loadDIDVectors(t)
	var files []string
	for _, pattern := range []string{"../web/*.js", "../web/*.html", "*.go", "../api/*.go"} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	seen := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, lit := range didLiteral.FindAllString(string(body), -1) {
			if lit == "did:infrix" || lit == "did:lattica" || strings.HasSuffix(lit, ":") && strings.Count(lit, ":") == 2 {
				continue // the method name in prose
			}
			seen++
			if strings.HasPrefix(lit, "did:lattica") {
				t.Errorf("%s shows the retired method: %s", f, lit)
				continue
			}
			if !canonicalInfrixDID.MatchString(lit) {
				t.Errorf("%s shows %q, which is not the canonical did:infrix:v1 grammar (the retired acc:// spelling is refused by the node)", f, lit)
				continue
			}
			if strings.HasPrefix(lit, "did:infrix:v1:acc:") && !strings.Contains(lit, ":"+set.NetworkID+":") {
				t.Errorf("%s shows %q, which is not bound to the vector set's network id %s", f, lit, set.NetworkID)
			}
		}
	}
	if seen == 0 {
		t.Fatal("the playground shows no did:infrix identifier at all; the sample scan is vacuous")
	}
}

// PLANTED DEFECT: the retired spelling is caught by the same scan.
func TestRetiredSpellingIsCaught(t *testing.T) {
	if canonicalInfrixDID.MatchString("did:infrix:acc://alice.acme") || canonicalInfrixDID.MatchString("did:lattica:acc://alice.acme") {
		t.Fatal("PLANTED DEFECT NOT DETECTED: the retired acc:// spelling matches the canonical grammar")
	}
}
