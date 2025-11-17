package generate

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/lechgu/tichy/internal/fetchers"
	"github.com/lechgu/tichy/internal/injectors"
	"github.com/lechgu/tichy/internal/models"
	testgen "github.com/lechgu/tichy/internal/tests"
	"github.com/samber/do/v2"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var (
	docType string
	source  string
	output  string
	num     int
)

var Cmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate test cases from knowledge base",
	RunE:  runGenerate,
}

func init() {
	Cmd.Flags().StringVarP(&docType, "mode", "m", "", "Document fetch mode")
	Cmd.Flags().StringVarP(&source, "source", "s", "", "Source")
	Cmd.Flags().StringVarP(&output, "output", "o", "tests.json", "Output file")
	Cmd.Flags().IntVarP(&num, "num", "n", 100, "Number of test cases")

	_ = Cmd.MarkFlagRequired("mode")
	_ = Cmd.MarkFlagRequired("source")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	if docType != "text" {
		return fmt.Errorf("unsupported type: %s", docType)
	}

	ctx := cmd.Context()

	generator, err := testgen.NewGenerator(injectors.Default)
	if err != nil {
		return fmt.Errorf("generator error: %w", err)
	}

	fetcher, err := do.InvokeNamed[fetchers.Fetcher](injectors.Default, docType)
	if err != nil {
		return fmt.Errorf("fetcher error: %w", err)
	}
	documents, err := fetcher.Fetch(ctx, source)
	if err != nil {
		return fmt.Errorf("fetch error: %w", err)
	}

	bar := progressbar.NewOptions(num,
		progressbar.OptionSetDescription("Generating tests"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionClearOnFinish(),
	)

	genConfig := testgen.GeneratorConfig{
		QuestionsPerDoc: 3,
		MinConfidence:   0.7,
		ContextSize:     1500,
		ContextOverlap:  300,
		Categories:      []string{},
		MaxTests:        num,
		OnProgress: func() {
			_ = bar.Add(1)
		},
	}

	testCases, err := generator.Generate(ctx, documents, genConfig)
	if err != nil {
		return fmt.Errorf("generation error: %w", err)
	}

	testFile := struct {
		Tests []models.TestQuestion `json:"tests"`
	}{
		Tests: testCases,
	}

	data, err := json.MarshalIndent(testFile, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON encoding error: %w", err)
	}

	if err := os.WriteFile(output, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("\nGenerated %d test cases, written to %s\n", len(testCases), output)

	printCategorySummary(testCases)

	return nil
}

func printCategorySummary(tests []models.TestQuestion) {
	categoryCount := make(map[string]int)
	for _, test := range tests {
		categoryCount[test.Category]++
	}

	fmt.Println("\nCategory breakdown:")
	for category, count := range categoryCount {
		fmt.Printf("  %-15s %d\n", category+":", count)
	}
}
