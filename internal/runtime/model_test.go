package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSemanticsRejectsTrailingJSON(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", ".gooo", "semantics.gooo"))
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, []byte("\n{}\n")...)
	if _, err := ParseSemantics(raw); err == nil {
		t.Fatal("ParseSemantics accepted trailing JSON")
	}
}
