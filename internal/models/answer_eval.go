package models

type AnswerEval struct {
	Feedback     string  `json:"feedback"`
	Accuracy     float64 `json:"accuracy"`
	Completeness float64 `json:"completeness"`
	Relevance    float64 `json:"relevance"`
}
