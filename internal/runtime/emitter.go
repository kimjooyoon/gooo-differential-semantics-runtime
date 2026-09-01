package runtime

import (
	"encoding/json"
	"fmt"
	"strconv"
)

const runtimeImportPath = "github.com/kimjooyoon/gooo-differential-semantics-runtime/internal/runtime"

func EmitGoSource(program Program, semantics Semantics, variant string) ([]byte, error) {
	programJSON, err := json.Marshal(program)
	if err != nil {
		return nil, err
	}
	semanticsJSON, err := json.Marshal(semantics)
	if err != nil {
		return nil, err
	}
	if variant == "" {
		variant = "none"
	}
	knownVariant := false
	for _, candidate := range semantics.Generation.CandidateVariants {
		if candidate == variant {
			knownVariant = true
			break
		}
	}
	if !knownVariant {
		return nil, fmt.Errorf("candidate variant %q is not declared by .gooo semantics", variant)
	}
	return []byte(fmt.Sprintf(`package main

import (
	"encoding/json"
	"fmt"
	"os"

	goooruntime %q
)

const semanticsJSON = %s
const programJSON = %s
const candidateVariant = %s

func main() {
	options, err := goooruntime.ParseOptionsArgs(os.Args[1:])
	if err != nil {
		fatal(err)
	}
	semantics, err := goooruntime.ParseSemantics([]byte(semanticsJSON))
	if err != nil {
		fatal(err)
	}
	var program goooruntime.Program
	if err := json.Unmarshal([]byte(programJSON), &program); err != nil {
		fatal(err)
	}
	outcome := goooruntime.EvaluateGenerated(program, semantics, options, candidateVariant)
	encoded, err := json.MarshalIndent(outcome, "", "  ")
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(encoded))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
`, runtimeImportPath, strconv.Quote(string(semanticsJSON)), strconv.Quote(string(programJSON)), strconv.Quote(variant))), nil
}
