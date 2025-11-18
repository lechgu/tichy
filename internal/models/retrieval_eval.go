package models

type RetrievalEval struct {
	MRR             float64 `json:"mrr"`
	NDCG            float64 `json:"ndcg"`
	KeywordCoverage float64 `json:"keyword_coverage"`
}
