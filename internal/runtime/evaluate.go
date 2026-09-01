package runtime

import (
	"fmt"
	"strings"
)

type evalIssue struct {
	status  Status
	reason  string
	unknown *UnknownRecord
}

type evalState struct {
	semantics Semantics
	options   Options
	bindings  map[string]Value
	trace     []EffectEvent
}

func EvaluateReference(program Program, semantics Semantics, options Options) Outcome {
	return evaluate(program, semantics, options, "none")
}

func EvaluateGenerated(program Program, semantics Semantics, options Options, variant string) Outcome {
	outcome := evaluate(program, semantics, options, variant)
	if variant == "trace-divergence" && outcome.Status == StatusClosed && len(outcome.Trace) > 0 {
		outcome.Trace[0].Effect = outcome.Trace[0].Effect + ":candidate"
		finalizeOutcome(&outcome)
	}
	return outcome
}

func evaluate(program Program, semantics Semantics, options Options, variant string) Outcome {
	state := &evalState{semantics: semantics, options: options, bindings: map[string]Value{}, trace: make([]EffectEvent, 0)}
	value, issue := state.runStatements(program.Body, nil)
	outcome := Outcome{Schema: "gooo.execution/v1", Trace: state.trace}
	if issue != nil {
		outcome.Status = issue.status
		outcome.Reason = issue.reason
		outcome.Unknown = issue.unknown
	} else if value == nil {
		outcome.Status = StatusRefuted
		outcome.Reason = "program did not produce a result"
	} else {
		outcome.Status = StatusClosed
		copy := value
		outcome.Value = &copy
	}
	finalizeOutcome(&outcome)
	return outcome
}

func (s *evalState) runStatements(statements []Stmt, blockResult *Expr) (*Value, *evalIssue) {
	for _, statement := range statements {
		switch statement.Kind {
		case "let":
			value, issue := s.evalExpr(statement.Expr)
			if issue != nil {
				return nil, issue
			}
			if value.Type != statement.DeclType {
				return nil, refutedIssue(fmt.Sprintf("let %s declares %s but expression has type %s", statement.Name, statement.DeclType, value.Type))
			}
			s.bindings[statement.Name] = value
		case "effect":
			if issue := s.execEffect(statement); issue != nil {
				return nil, issue
			}
		case "if":
			condition, issue := s.evalExpr(statement.Expr)
			if issue != nil {
				return nil, issue
			}
			if condition.Type != "bool" || condition.Bool == nil {
				return nil, refutedIssue(fmt.Sprintf("if condition has type %s, want bool", condition.Type))
			}
			branch := statement.Else
			if *condition.Bool {
				branch = statement.Then
			}
			if branch == nil {
				return nil, refutedIssue("conditional branch is missing")
			}
			value, issue := s.runStatements(branch.Statements, branch.Result)
			if issue != nil || value != nil {
				return value, issue
			}
		case "result":
			value, issue := s.evalExpr(statement.Expr)
			if issue != nil {
				return nil, issue
			}
			return &value, nil
		default:
			return nil, refutedIssue(fmt.Sprintf("unknown statement kind %q", statement.Kind))
		}
	}
	if blockResult != nil {
		value, issue := s.evalExpr(blockResult)
		if issue != nil {
			return nil, issue
		}
		return &value, nil
	}
	return nil, nil
}

func (s *evalState) execEffect(statement Stmt) *evalIssue {
	value, issue := s.evalExpr(statement.Expr)
	if issue != nil {
		return issue
	}
	rule, ok := s.semantics.Effects[statement.Effect]
	if !ok {
		return refutedIssue(fmt.Sprintf("effect %q is not declared by .gooo semantics", statement.Effect))
	}
	if value.Type != rule.Arg {
		return refutedIssue(fmt.Sprintf("effect %s expects %s but received %s", statement.Effect, rule.Arg, value.Type))
	}
	if !s.options.Grants[statement.Effect] {
		return unknownIssue(s.semantics, "missing-effect-grant", statement.Effect)
	}
	s.trace = append(s.trace, EffectEvent{Ordinal: len(s.trace), Effect: rule.Trace, Value: value})
	return nil
}

func (s *evalState) evalExpr(expression *Expr) (Value, *evalIssue) {
	if expression == nil {
		return Value{}, refutedIssue("missing expression")
	}
	switch expression.Kind {
	case "int":
		return IntValue(expression.Int), nil
	case "bool":
		return BoolValue(expression.Bool), nil
	case "string":
		return StringValue(expression.Text), nil
	case "var":
		value, ok := s.bindings[expression.Name]
		if !ok {
			return Value{}, refutedIssue(fmt.Sprintf("variable %q is not bound", expression.Name))
		}
		return value, nil
	case "call":
		return s.evalCall(expression)
	default:
		return Value{}, refutedIssue(fmt.Sprintf("unknown expression kind %q", expression.Kind))
	}
}

func (s *evalState) evalCall(expression *Expr) (Value, *evalIssue) {
	rule, ok := s.semantics.PureFunctions[expression.Name]
	if !ok {
		return Value{}, refutedIssue(fmt.Sprintf("pure function %q is not declared by .gooo semantics", expression.Name))
	}
	if len(expression.Args) != len(rule.Args) {
		return Value{}, refutedIssue(fmt.Sprintf("pure function %s expects %d arguments, got %d", expression.Name, len(rule.Args), len(expression.Args)))
	}
	arguments := make([]Value, len(expression.Args))
	for index, argument := range expression.Args {
		value, issue := s.evalExpr(argument)
		if issue != nil {
			return Value{}, issue
		}
		if value.Type != rule.Args[index] {
			return Value{}, refutedIssue(fmt.Sprintf("pure function %s argument %d expects %s but received %s", expression.Name, index+1, rule.Args[index], value.Type))
		}
		arguments[index] = value
	}
	switch rule.Operation {
	case "add":
		return IntValue(*arguments[0].Int + *arguments[1].Int), nil
	case "not":
		return BoolValue(!*arguments[0].Bool), nil
	case "is_positive":
		return BoolValue(*arguments[0].Int > 0), nil
	case "external_int", "external_bool":
		name := *arguments[0].String
		value, found := s.options.Externals[name]
		if !found {
			return Value{}, unknownIssue(s.semantics, "external-value", name)
		}
		if value.Type != rule.Returns {
			return Value{}, refutedIssue(fmt.Sprintf("external %s returned %s, want %s", name, value.Type, rule.Returns))
		}
		return value, nil
	default:
		return Value{}, refutedIssue(fmt.Sprintf("pure function %s has unsupported operation %q", expression.Name, rule.Operation))
	}
}

func refutedIssue(reason string) *evalIssue {
	return &evalIssue{status: StatusRefuted, reason: reason}
}

func unknownIssue(semantics Semantics, key, subject string) *evalIssue {
	template, ok := semantics.Unknowns[key]
	if !ok {
		return refutedIssue(fmt.Sprintf(".gooo semantics has no UNKNOWN template %q", key))
	}
	record := template
	record.Reason = strings.ReplaceAll(record.Reason, "%s", subject)
	record.Step = strings.ReplaceAll(record.Step, "%s", subject)
	record.NextOperation = strings.ReplaceAll(record.NextOperation, "%s", subject)
	record.BlockedBy = strings.ReplaceAll(record.BlockedBy, "%s", subject)
	return &evalIssue{status: StatusUnknown, reason: record.Reason, unknown: &record}
}

func finalizeOutcome(outcome *Outcome) {
	payload := struct {
		Status  Status         `json:"status"`
		Value   *Value         `json:"value,omitempty"`
		Trace   []EffectEvent  `json:"trace"`
		Reason  string         `json:"reason,omitempty"`
		Unknown *UnknownRecord `json:"unknown,omitempty"`
	}{Status: outcome.Status, Value: outcome.Value, Trace: outcome.Trace, Reason: outcome.Reason, Unknown: outcome.Unknown}
	outcome.TerminalExplanationDigest = DigestJSON(payload)
}
