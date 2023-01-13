package event

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
	if err := ValidateSource(candidate.Source); err != nil {
		return err
	}

	if err := ValidateSubject(candidate.Subject); err != nil {
		return err
	}

	if err := ValidateType(candidate.Type); err != nil {
		return err
	}

	return nil
}
