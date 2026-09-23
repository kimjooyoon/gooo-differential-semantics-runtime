package runtime

import (
	"fmt"
	"os"
	"reflect"
)

type ComparisonReport struct {
	Schema                  string         `json:"schema"`
	CaseID                  string         `json:"case_id"`
	ExpectedStatus          Status         `json:"expected_status"`
	Verdict                 Status         `json:"verdict"`
	Matched                 bool           `json:"matched"`
	Compared                []string       `json:"compared"`
	ReferenceStatus         Status         `json:"reference_status"`
	GeneratedStatus         Status         `json:"generated_status"`
	TypedValueEqual         bool           `json:"typed_value_equal"`
	OrderedEffectTraceEqual bool           `json:"ordered_effect_trace_equal"`
	TerminalDigestEqual     bool           `json:"terminal_explanation_digest_equal"`
	Reason                  string         `json:"reason,omitempty"`
	Unknown                 *UnknownRecord `json:"unknown,omitempty"`
}

func LoadOutcome(path string) (Outcome, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Outcome{}, err
	}
	var outcome Outcome
	if err := decodeStrictJSON(raw, &outcome, "execution outcome"); err != nil {
		return Outcome{}, err
	}
	if outcome.Schema != "gooo.execution/v1" || outcome.Status == "" || outcome.TerminalExplanationDigest == "" {
		return Outcome{}, fmt.Errorf("invalid execution outcome in %s", path)
	}
	return outcome, nil
}

func Compare(caseID string, expected Status, reference, generated Outcome) ComparisonReport {
	typedValueEqual := reflect.DeepEqual(reference.Value, generated.Value)
	traceEqual := reflect.DeepEqual(reference.Trace, generated.Trace)
	digestEqual := reference.TerminalExplanationDigest == generated.TerminalExplanationDigest
	matched := typedValueEqual && traceEqual && digestEqual
	verdict := StatusClosed
	reason := "reference and generated execution agree"
	if !matched {
		verdict = StatusRefuted
		switch {
		case !typedValueEqual:
			reason = "typed value divergence"
		case !traceEqual:
			reason = "ordered effect trace divergence"
		default:
			reason = "terminal explanation digest divergence"
		}
	} else if reference.Status == StatusRefuted || generated.Status == StatusRefuted {
		verdict = StatusRefuted
		reason = reference.Reason
		if reason == "" {
			reason = generated.Reason
		}
	} else if reference.Status == StatusUnknown || generated.Status == StatusUnknown {
		verdict = StatusUnknown
		reason = reference.Reason
		if reason == "" {
			reason = generated.Reason
		}
	}
	report := ComparisonReport{
		Schema:                  "gooo.differential-comparison/v1",
		CaseID:                  caseID,
		ExpectedStatus:          expected,
		Verdict:                 verdict,
		Matched:                 matched,
		Compared:                []string{"typed_value", "ordered_effect_trace", "terminal_explanation_digest"},
		ReferenceStatus:         reference.Status,
		GeneratedStatus:         generated.Status,
		TypedValueEqual:         typedValueEqual,
		OrderedEffectTraceEqual: traceEqual,
		TerminalDigestEqual:     digestEqual,
		Reason:                  reason,
	}
	if verdict == StatusUnknown {
		report.Unknown = reference.Unknown
	}
	return report
}

func ValidateExpected(report ComparisonReport) error {
	if report.Verdict != report.ExpectedStatus {
		return fmt.Errorf("case %s verdict %s, expected %s", report.CaseID, report.Verdict, report.ExpectedStatus)
	}
	if report.Verdict == StatusUnknown && (report.Unknown == nil || report.Unknown.Validate() != nil) {
		return fmt.Errorf("case %s UNKNOWN lost its six-field record", report.CaseID)
	}
	return nil
}
