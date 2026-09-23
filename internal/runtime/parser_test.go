package runtime

import "testing"

func TestParseProgramRejectsNonTerminalResult(t *testing.T) {
	cases := []string{
		`program invalid { result 1; effect emit(1); result 1; }`,
		`program invalid { if true { result 1; let later: int = 2; result later; } else { result 1; } }`,
	}
	for _, source := range cases {
		if _, err := ParseProgramBytes([]byte(source)); err == nil {
			t.Fatalf("ParseProgramBytes accepted non-terminal result: %s", source)
		}
	}
}

