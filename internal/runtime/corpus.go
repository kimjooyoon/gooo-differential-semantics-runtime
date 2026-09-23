package runtime

import (
	"fmt"
	"os"
)

type Corpus struct {
	Schema    string         `json:"schema"`
	Authority string         `json:"authority"`
	Counts    map[string]int `json:"counts"`
	Cases     []CorpusCase   `json:"cases"`
}

type CorpusCase struct {
	ID               string            `json:"id"`
	Program          string            `json:"program"`
	ExpectedStatus   Status            `json:"expected_status"`
	Grants           []string          `json:"grants"`
	Externals        map[string]string `json:"externals"`
	CandidateVariant string            `json:"candidate_variant"`
	Replay           bool              `json:"replay"`
}

func LoadCorpus(path string) (Corpus, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Corpus{}, err
	}
	var corpus Corpus
	if err := decodeStrictJSON(raw, &corpus, "corpus .gooo"); err != nil {
		return Corpus{}, err
	}
	if err := corpus.Validate(); err != nil {
		return Corpus{}, err
	}
	return corpus, nil
}

func (c Corpus) Validate() error {
	if c.Schema != "gooo.corpus/v1" || c.Authority != "metacode" {
		return fmt.Errorf("corpus .gooo must be authoritative gooo.corpus/v1")
	}
	if len(c.Cases) != 9 {
		return fmt.Errorf("corpus must contain exactly 9 cases")
	}
	seen := map[string]bool{}
	counts := map[string]int{"normal": 0, "unknown": 0, "refuted": 0, "replay": 0}
	for _, testCase := range c.Cases {
		if testCase.ID == "" || seen[testCase.ID] {
			return fmt.Errorf("corpus case id is empty or duplicated: %q", testCase.ID)
		}
		seen[testCase.ID] = true
		if testCase.Program == "" || testCase.CandidateVariant == "" {
			return fmt.Errorf("corpus case %q is incomplete", testCase.ID)
		}
		if testCase.ExpectedStatus != StatusClosed && testCase.ExpectedStatus != StatusUnknown && testCase.ExpectedStatus != StatusRefuted {
			return fmt.Errorf("corpus case %q has invalid expected status %q", testCase.ID, testCase.ExpectedStatus)
		}
		if testCase.Replay {
			counts["replay"]++
		} else {
			switch testCase.ExpectedStatus {
			case StatusClosed:
				counts["normal"]++
			case StatusUnknown:
				counts["unknown"]++
			case StatusRefuted:
				counts["refuted"]++
			}
		}
	}
	for key, expected := range map[string]int{"normal": 4, "unknown": 2, "refuted": 2, "replay": 1} {
		if c.Counts[key] != expected || counts[key] != expected {
			return fmt.Errorf("corpus count %s must be exactly %d", key, expected)
		}
	}
	return nil
}
