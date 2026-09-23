package runtime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type Status string

const (
	StatusClosed  Status = "CLOSED"
	StatusUnknown Status = "UNKNOWN"
	StatusRefuted Status = "REFUTED"
)

type Semantics struct {
	Schema          string                    `json:"schema"`
	Authority       string                    `json:"authority"`
	Language        string                    `json:"language"`
	Version         string                    `json:"version"`
	Types           []string                  `json:"types"`
	Grammar         map[string]any            `json:"grammar"`
	ExpressionRules map[string]map[string]any `json:"expression_rules"`
	PureFunctions   map[string]FunctionRule   `json:"pure_functions"`
	Effects         map[string]EffectRule     `json:"effects"`
	Unknowns        map[string]UnknownRecord  `json:"unknowns"`
	Generation      GenerationRule            `json:"generation"`
	Comparison      ComparisonRule            `json:"comparison"`
}

type FunctionRule struct {
	Args      []string `json:"args"`
	Returns   string   `json:"returns"`
	Operation string   `json:"operation"`
}

type EffectRule struct {
	Arg   string `json:"arg"`
	Trace string `json:"trace"`
}

type GenerationRule struct {
	Target            string   `json:"target"`
	BinaryRequired    bool     `json:"binary_required"`
	CandidateVariants []string `json:"candidate_variants"`
}

type ComparisonRule struct {
	Fields           []string `json:"fields"`
	StatusPrecedence []Status `json:"status_precedence"`
}

type UnknownRecord struct {
	Stage         string `json:"stage"`
	Step          string `json:"step"`
	Reason        string `json:"reason"`
	UnknownClass  string `json:"unknown_class"`
	NextOperation string `json:"next_operation"`
	BlockedBy     string `json:"blocked_by"`
}

type Program struct {
	Name string `json:"name"`
	Body []Stmt `json:"body"`
}

type Block struct {
	Statements []Stmt `json:"statements"`
	Result     *Expr  `json:"result"`
}

type Stmt struct {
	Kind     string `json:"kind"`
	Name     string `json:"name,omitempty"`
	DeclType string `json:"decl_type,omitempty"`
	Expr     *Expr  `json:"expr,omitempty"`
	Effect   string `json:"effect,omitempty"`
	Then     *Block `json:"then,omitempty"`
	Else     *Block `json:"else,omitempty"`
}

type Expr struct {
	Kind string  `json:"kind"`
	Int  int64   `json:"int,omitempty"`
	Bool bool    `json:"bool,omitempty"`
	Text string  `json:"text,omitempty"`
	Name string  `json:"name,omitempty"`
	Args []*Expr `json:"args,omitempty"`
}

type Value struct {
	Type   string  `json:"type"`
	Int    *int64  `json:"int,omitempty"`
	Bool   *bool   `json:"bool,omitempty"`
	String *string `json:"string,omitempty"`
}

type EffectEvent struct {
	Ordinal int    `json:"ordinal"`
	Effect  string `json:"effect"`
	Value   Value  `json:"value"`
}

type Outcome struct {
	Schema                    string         `json:"schema"`
	Status                    Status         `json:"status"`
	Value                     *Value         `json:"value,omitempty"`
	Trace                     []EffectEvent  `json:"trace"`
	TerminalExplanationDigest string         `json:"terminal_explanation_digest"`
	Reason                    string         `json:"reason,omitempty"`
	Unknown                   *UnknownRecord `json:"unknown,omitempty"`
}

type Options struct {
	Grants    map[string]bool
	Externals map[string]Value
}

func LoadSemantics(path string) (Semantics, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Semantics{}, nil, err
	}
	semantics, err := ParseSemantics(raw)
	return semantics, raw, err
}

func ParseSemantics(raw []byte) (Semantics, error) {
	var semantics Semantics
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&semantics); err != nil {
		return Semantics{}, fmt.Errorf("parse .gooo semantics: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Semantics{}, errors.New("parse .gooo semantics: trailing JSON value")
		}
		return Semantics{}, fmt.Errorf("parse .gooo semantics: trailing input: %w", err)
	}
	if err := semantics.Validate(); err != nil {
		return Semantics{}, err
	}
	return semantics, nil
}

func (s Semantics) Validate() error {
	if s.Schema != "gooo.semantics/v1" || s.Authority != "metacode" || s.Language != "Gooo" {
		return errors.New(".gooo semantics must be authoritative Gooo v1 metacode")
	}
	if len(s.Types) != 3 || s.Types[0] != "int" || s.Types[1] != "bool" || s.Types[2] != "string" {
		return errors.New("semantics must declare int, bool, and string values")
	}
	if len(s.PureFunctions) == 0 || len(s.Effects) == 0 {
		return errors.New("semantics must declare pure functions and effects")
	}
	for name, fn := range s.PureFunctions {
		if name == "" || fn.Operation == "" || fn.Returns == "" {
			return fmt.Errorf("pure function %q is incomplete", name)
		}
	}
	for name, effect := range s.Effects {
		if name == "" || effect.Arg == "" || effect.Trace == "" {
			return fmt.Errorf("effect %q is incomplete", name)
		}
	}
	for key, unknown := range s.Unknowns {
		if key == "" || unknown.Validate() != nil {
			return fmt.Errorf("UNKNOWN record %q is incomplete", key)
		}
	}
	if !equalStatuses(s.Comparison.StatusPrecedence, []Status{StatusRefuted, StatusUnknown, StatusClosed}) {
		return errors.New("comparison precedence must be REFUTED > UNKNOWN > CLOSED")
	}
	if !equalStrings(s.Comparison.Fields, []string{"typed_value", "ordered_effect_trace", "terminal_explanation_digest"}) {
		return errors.New("comparison must name the three required differential fields")
	}
	if s.Generation.Target != "go1.27" || !s.Generation.BinaryRequired || len(s.Generation.CandidateVariants) != 2 || s.Generation.CandidateVariants[0] != "none" || s.Generation.CandidateVariants[1] != "trace-divergence" {
		return errors.New("generation must require Go 1.27 binaries and the fixed variants")
	}
	return nil
}

func (u UnknownRecord) Validate() error {
	if u.Stage == "" || u.Step == "" || u.Reason == "" || u.UnknownClass == "" || u.NextOperation == "" || u.BlockedBy == "" {
		return errors.New("UNKNOWN must preserve stage, step, reason, unknown_class, next_operation, and blocked_by")
	}
	return nil
}

func ParseProgram(path string) (Program, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Program{}, nil, err
	}
	program, err := ParseProgramBytes(raw)
	return program, raw, err
}

func ParseProgramBytes(raw []byte) (Program, error) {
	program, err := newParser(string(raw)).parseProgram()
	if err != nil {
		return Program{}, fmt.Errorf("parse .gooo program: %w", err)
	}
	return program, nil
}

func ParseOptionsArgs(args []string) (Options, error) {
	options := Options{Grants: map[string]bool{}, Externals: map[string]Value{}}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--grant":
			if index+1 >= len(args) || args[index+1] == "" {
				return Options{}, errors.New("--grant requires an effect name")
			}
			options.Grants[args[index+1]] = true
			index++
		case "--external":
			if index+1 >= len(args) {
				return Options{}, errors.New("--external requires name=value")
			}
			name, value, ok := strings.Cut(args[index+1], "=")
			if !ok || name == "" {
				return Options{}, errors.New("--external requires name=value")
			}
			parsed, err := parseExternalValue(value)
			if err != nil {
				return Options{}, fmt.Errorf("external %q: %w", name, err)
			}
			options.Externals[name] = parsed
			index++
		default:
			return Options{}, fmt.Errorf("unknown runtime option %q", args[index])
		}
	}
	return options, nil
}

func parseExternalValue(raw string) (Value, error) {
	if raw == "true" || raw == "false" {
		value := raw == "true"
		return BoolValue(value), nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return Value{}, errors.New("value must be an integer or boolean")
	}
	return IntValue(value), nil
}

func IntValue(value int64) Value {
	return Value{Type: "int", Int: &value}
}

func BoolValue(value bool) Value {
	return Value{Type: "bool", Bool: &value}
}

func StringValue(value string) Value {
	return Value{Type: "string", String: &value}
}

func DigestJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal deterministic JSON: %v", err))
	}
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func equalStatuses(actual, expected []Status) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

func equalStrings[T ~string](actual, expected []T) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if string(actual[index]) != string(expected[index]) {
			return false
		}
	}
	return true
}
