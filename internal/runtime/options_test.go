package runtime

import "testing"

func TestParseOptionsRejectsDuplicateNames(t *testing.T) {
	if _, err := ParseOptionsArgs([]string{"--grant", "OUTPUT", "--grant", "OUTPUT"}); err == nil {
		t.Fatal("duplicate --grant was accepted")
	}
	if _, err := ParseOptionsArgs([]string{"--external", "flag=true", "--external", "flag=false"}); err == nil {
		t.Fatal("duplicate --external was accepted")
	}
}
