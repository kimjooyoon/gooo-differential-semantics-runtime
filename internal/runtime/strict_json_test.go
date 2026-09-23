package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeJSONBoundariesRejectUnknownAndTrailingValues(t *testing.T) {
	cases := map[string]string{
		"unknown":  `{"unexpected":true}`,
		"trailing": `{} {"extra":true}`,
	}
	for name, payload := range cases {
		if _, err := ParseSemantics([]byte(payload)); err == nil {
			t.Fatalf("%s semantics input was accepted", name)
		}
		corpusPath := filepath.Join(t.TempDir(), name+"-corpus.json")
		if err := os.WriteFile(corpusPath, []byte(payload), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadCorpus(corpusPath); err == nil {
			t.Fatalf("%s corpus input was accepted", name)
		}
		outcomePath := filepath.Join(t.TempDir(), name+"-outcome.json")
		if err := os.WriteFile(outcomePath, []byte(payload), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadOutcome(outcomePath); err == nil {
			t.Fatalf("%s outcome input was accepted", name)
		}
	}
}
