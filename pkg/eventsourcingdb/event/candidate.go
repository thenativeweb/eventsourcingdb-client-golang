package event

import (
	"fmt"
	"net/url"
	"regexp"
)

var subjectRegex *regexp.Regexp
var typeRegex *regexp.Regexp

func init() {
	wordRegex := "[0-9A-Za-z_-]+"
	subjectRegex = regexp.MustCompile(fmt.Sprintf("^/(%[1]s/)*(%[1]s/?)?$", wordRegex))
	typeRegex = regexp.MustCompile("^[0-9A-Za-z_-]{2,}\\.([0-9A-Za-z_-]+\\.)+[0-9A-Za-z_-]+$")
}

type CandidateContext struct {
	Source  string `json:"source"`
	Subject string `json:"subject"`
	Type    string `json:"type"`
}

type Candidate struct {
	CandidateContext
	Data Data `json:"data"`
}

func NewCandidate(
	source string,
	subject string,
	eventType string,
	data Data,
) Candidate {
	return Candidate{
		CandidateContext{
			Source:  source,
			Subject: subject,
			Type:    eventType,
		},
		data,
	}
}

func (candidate Candidate) Validate() error {
	_, err := url.Parse(candidate.Source)
	if err != nil {
		return fmt.Errorf("malformed event source '%s', must be a valid URI", candidate.Source)
	}

	didMatch := subjectRegex.MatchString(candidate.Subject)
	if !didMatch {
		return fmt.Errorf("malformed event subject '%s', must be an absolute, slash-separated path", candidate.Subject)
	}

	didMatch = typeRegex.MatchString(candidate.Type)
	if !didMatch {
		return fmt.Errorf("malformed event type '%s', must be reverse domain name", candidate.Type)
	}

	return nil
}
