package evaluate

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/lechgu/tichy/internal/evaluators"
	"github.com/lechgu/tichy/internal/injectors"
	"github.com/lechgu/tichy/internal/models"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var (
	input string
)

var Cmd = &cobra.Command{
	Use:   "evaluate",
	Short: "Evaluate RAG system using test cases",
	RunE:  runEvaluate,
}

func init() {
	Cmd.Flags().StringVarP(&input, "input", "i", "tests.json", "Test cases file")
	_ = Cmd.MarkFlagRequired("input")
}

func runEvaluate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("failed to read test file: %w", err)
	}

	var testData struct {
		Tests []models.TestQuestion `json:"tests"`
	}
	if err := json.Unmarshal(data, &testData); err != nil {
		return fmt.Errorf("failed to parse test file: %w", err)
	}

	evaluator, err := evaluators.New(injectors.Default)
	if err != nil {
		return fmt.Errorf("evaluator error: %w", err)
	}

	var totalMRR, totalNDCG, totalKeywordCoverage float64
	var totalAccuracy, totalCompleteness, totalRelevance float64
	successCount := 0

	bar := progressbar.NewOptions(len(testData.Tests),
		progressbar.OptionSetDescription("Evaluating tests"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionClearOnFinish(),
	)

	for _, test := range testData.Tests {
		retrieval, err := evaluator.EvaluateRetrieval(ctx, test)
		if err != nil {
			_ = bar.Add(1)
			continue
		}

		answer, _, _, err := evaluator.EvaluateAnswer(ctx, test)
		if err != nil {
			_ = bar.Add(1)
			continue
		}

		totalMRR += retrieval.MRR
		totalNDCG += retrieval.NDCG
		totalKeywordCoverage += retrieval.KeywordCoverage
		totalAccuracy += answer.Accuracy
		totalCompleteness += answer.Completeness
		totalRelevance += answer.Relevance
		successCount++
		_ = bar.Add(1)
	}

	if successCount == 0 {
		return fmt.Errorf("all evaluations failed")
	}

	fmt.Printf("\n=== Summary (%d tests) ===\n", successCount)
	fmt.Printf("Retrieval Metrics:\n")
	fmt.Printf("  Avg MRR:              %.3f\n", totalMRR/float64(successCount))
	fmt.Printf("  Avg NDCG:             %.3f\n", totalNDCG/float64(successCount))
	fmt.Printf("  Avg Keyword Coverage: %.1f%%\n", totalKeywordCoverage/float64(successCount))
	fmt.Printf("\nAnswer Metrics:\n")
	fmt.Printf("  Avg Accuracy:         %.2f/5\n", totalAccuracy/float64(successCount))
	fmt.Printf("  Avg Completeness:     %.2f/5\n", totalCompleteness/float64(successCount))
	fmt.Printf("  Avg Relevance:        %.2f/5\n", totalRelevance/float64(successCount))

	return nil
}
