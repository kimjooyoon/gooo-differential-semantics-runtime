package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	runtime "github.com/kimjooyoon/gooo-differential-semantics-runtime/internal/runtime"
)

func main() {
	if len(os.Args) < 2 {
		fail(errors.New("command is required: reference, emit, compare, or corpus"))
	}
	switch os.Args[1] {
	case "reference":
		runReference(os.Args[2:])
	case "emit":
		runEmit(os.Args[2:])
	case "compare":
		runCompare(os.Args[2:])
	case "corpus":
		runCorpus(os.Args[2:])
	default:
		fail(fmt.Errorf("unknown command %q", os.Args[1]))
	}
}

type stringList []string

func (values *stringList) String() string {
	return fmt.Sprint([]string(*values))
}

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runReference(args []string) {
	flags := flag.NewFlagSet("reference", flag.ContinueOnError)
	semanticsPath := flags.String("semantics", ".gooo/semantics.gooo", "authoritative semantics .gooo")
	programPath := flags.String("program", "", "Gooo program")
	outputPath := flags.String("output", "", "JSON output")
	var runtimeArgs stringList
	flags.Var(&runtimeArgs, "grant", "granted effect name")
	var externalArgs stringList
	flags.Var(&externalArgs, "external", "external value name=value")
	if err := flags.Parse(args); err != nil {
		fail(err)
	}
	if *programPath == "" {
		fail(errors.New("--program is required"))
	}
	semantics, _, err := runtime.LoadSemantics(*semanticsPath)
	if err != nil {
		fail(err)
	}
	program, _, err := runtime.ParseProgram(*programPath)
	if err != nil {
		fail(err)
	}
	options, err := runtime.ParseOptionsArgs(runtimeOptions(runtimeArgs, externalArgs))
	if err != nil {
		fail(err)
	}
	outcome := runtime.EvaluateReference(program, semantics, options)
	writeJSON(*outputPath, outcome)
}

func runEmit(args []string) {
	flags := flag.NewFlagSet("emit", flag.ContinueOnError)
	semanticsPath := flags.String("semantics", ".gooo/semantics.gooo", "authoritative semantics .gooo")
	programPath := flags.String("program", "", "Gooo program")
	outputPath := flags.String("output", "", "generated Go source")
	variant := flags.String("variant", "none", "candidate generation variant")
	if err := flags.Parse(args); err != nil {
		fail(err)
	}
	if *programPath == "" || *outputPath == "" {
		fail(errors.New("--program and --output are required"))
	}
	semantics, _, err := runtime.LoadSemantics(*semanticsPath)
	if err != nil {
		fail(err)
	}
	program, _, err := runtime.ParseProgram(*programPath)
	if err != nil {
		fail(err)
	}
	source, err := runtime.EmitGoSource(program, semantics, *variant)
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(filepath.Dir(*outputPath), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(*outputPath, source, 0o644); err != nil {
		fail(err)
	}
}

func runCompare(args []string) {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	caseID := flags.String("case-id", "", "corpus case id")
	expected := flags.String("expected", "", "expected status")
	referencePath := flags.String("reference", "", "reference outcome")
	generatedPath := flags.String("generated", "", "generated outcome")
	outputPath := flags.String("output", "", "comparison report")
	if err := flags.Parse(args); err != nil {
		fail(err)
	}
	if *caseID == "" || *referencePath == "" || *generatedPath == "" || *outputPath == "" {
		fail(errors.New("--case-id, --reference, --generated, and --output are required"))
	}
	reference, err := runtime.LoadOutcome(*referencePath)
	if err != nil {
		fail(err)
	}
	generated, err := runtime.LoadOutcome(*generatedPath)
	if err != nil {
		fail(err)
	}
	expectedStatus := runtime.Status(*expected)
	if expectedStatus != runtime.StatusClosed && expectedStatus != runtime.StatusUnknown && expectedStatus != runtime.StatusRefuted {
		fail(fmt.Errorf("invalid expected status %q", *expected))
	}
	report := runtime.Compare(*caseID, expectedStatus, reference, generated)
	writeJSON(*outputPath, report)
	if err := runtime.ValidateExpected(report); err != nil {
		fail(err)
	}
}

func runCorpus(args []string) {
	flags := flag.NewFlagSet("corpus", flag.ContinueOnError)
	path := flags.String("path", ".gooo/corpus.gooo", "corpus .gooo")
	outputPath := flags.String("output", "", "summary JSON")
	if err := flags.Parse(args); err != nil {
		fail(err)
	}
	corpus, err := runtime.LoadCorpus(*path)
	if err != nil {
		fail(err)
	}
	summary := struct {
		Schema string         `json:"schema"`
		Cases  int            `json:"cases"`
		Counts map[string]int `json:"counts"`
	}{Schema: "gooo.corpus-validation/v1", Cases: len(corpus.Cases), Counts: corpus.Counts}
	writeJSON(*outputPath, summary)
}

func runtimeOptions(grants, externals stringList) []string {
	args := make([]string, 0, len(grants)*2+len(externals)*2)
	for _, grant := range grants {
		args = append(args, "--grant", grant)
	}
	for _, external := range externals {
		args = append(args, "--external", external)
	}
	return args
}

func writeJSON(path string, value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fail(err)
	}
	encoded = append(encoded, '\n')
	if path == "" {
		fmt.Print(string(encoded))
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
