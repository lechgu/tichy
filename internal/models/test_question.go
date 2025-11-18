package models

type TestQuestion struct {
	Question        string   `json:"question"`
	Category        string   `json:"category"`
	ReferenceAnswer string   `json:"reference_answer"`
	Keywords        []string `json:"keywords"`
	ExpectedSources []string `json:"expected_sources,omitempty"`
}
