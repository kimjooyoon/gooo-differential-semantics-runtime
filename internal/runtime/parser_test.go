package runtime

import "testing"

func TestParserRejectsStatementsAfterTerminalResult(t *testing.T) {
	for _, source := range []string{
		`program Example { result 1; result 2; }`,
		`program Example { if true { result 1; } else { result 2; } result 3; }`,
	} {
		if _, err := newParser(source).parseProgram(); err == nil {
			t.Fatalf("expected unreachable terminal statement to be rejected: %s", source)
		}
	}
}

func TestParserAcceptsTerminalBranchAsFinalStatement(t *testing.T) {
	program, err := newParser(`program Example { if true { result 1; } else { result 2; } }`).parseProgram()
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Body) != 1 || program.Body[0].Kind != "if" {
		t.Fatalf("unexpected parsed program: %#v", program)
	}
}
