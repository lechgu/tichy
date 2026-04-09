package qdrantstore

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/qdrant/go-client/qdrant"
)

// helper function to build LLM prompt from user userQuery
// suitable to extract various attributes
func buildPrompt(userQuery string) string {
	return `
You are a query parser for an elog search system.

Extract structured filters from the user query.

Return ONLY valid JSON.

Fields:
- query: main semantic search text
- author: person name if mentioned
- category: one of [Operations, Vacuum, RF, LINAC, Ring]
- date_from: ISO8601 (YYYY-MM-DDTHH:MM:SSZ)
- date_to: ISO8601

Examples:

Input: "find logs from ops from april 2025"
Output:
{
  "query": "logs",
  "category": "Operations",
  "date_from": "2025-04-01T00:00:00Z",
  "date_to": "2025-05-01T00:00:00Z"
}

Input: "what did Maxwell do last week"
Output:
{
  "query": "activities",
  "author": "Maxwell",
  "date_from": "...",
  "date_to": "..."
}

Rules:
- "ops" = "Operations"
- Convert natural dates (e.g. "April 2025") into proper ranges
- If no filter, omit the field

User query:
` + userQuery
}

func ExtractQueryParams(ctx context.Context, llmURL, userQuery string) (QueryParams, error) {
	client := openai.NewClient(
		option.WithBaseURL(llmURL+"/v1"),
		option.WithAPIKey("not-needed"),
	)

	userPrompt := buildPrompt(userQuery)
	systemPrompt := "You extract structured JSON from user queries."

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4o,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Temperature: openai.Float(0.), // Some creativity but not too much
	})
	if err != nil {
		return QueryParams{}, err
	}

	content := resp.Choices[0].Message.Content
	content = extractJSON(content)

	var qp QueryParams
	err = json.Unmarshal([]byte(content), &qp)
	return qp, err
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end <= start {
		return s
	}
	return s[start : end+1]
}

func MatchString(key, value string) *qdrant.Condition {
	return &qdrant.Condition{
		ConditionOneOf: &qdrant.Condition_Field{
			Field: &qdrant.FieldCondition{
				Key: key,
				Match: &qdrant.Match{
					MatchValue: &qdrant.Match_Keyword{
						Keyword: value,
					},
				},
			},
		},
	}
}

func MatchBool(key string, val bool) *qdrant.Condition {
	return &qdrant.Condition{
		ConditionOneOf: &qdrant.Condition_Field{
			Field: &qdrant.FieldCondition{
				Key: key,
				Match: &qdrant.Match{
					MatchValue: &qdrant.Match_Boolean{
						Boolean: val,
					},
				},
			},
		},
	}
}

type QueryParams struct {
	Query    string `json:"query"`
	Author   string `json:"author,omitempty"`
	Category string `json:"category,omitempty"`
	DateFrom string `json:"date_from,omitempty"`
	DateTo   string `json:"date_to,omitempty"`
}

// parseDate tries a few common RFC formats used in elog headers.
func parseDate(s string) int64 {
	formats := []string{
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 -0700",
		"2023-01-31T23:35:43-05:00",
		time.RFC3339,
		time.RFC1123Z,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.Unix()
		}
	}
	return 0
}

func BuildDynamicFilter(p QueryParams) *qdrant.Filter {
	var must []*qdrant.Condition

	if p.Author != "" {
		must = append(must, MatchString("author", p.Author))
	}
	if p.Category != "" {
		must = append(must, MatchString("category", p.Category))
	}
	if p.DateFrom != "" || p.DateTo != "" {
		must = append(must, DateRangeTS("date_ts", parseDate(p.DateFrom), parseDate(p.DateTo)))
	}

	if len(must) == 0 {
		return nil
	}

	return &qdrant.Filter{Must: must}
}

func DateRangeTS(key string, from, to int64) *qdrant.Condition {
	fromF := float64(from)
	toF := float64(to)

	return &qdrant.Condition{
		ConditionOneOf: &qdrant.Condition_Field{
			Field: &qdrant.FieldCondition{
				Key: key,
				Range: &qdrant.Range{
					Gte: &fromF,
					Lt:  &toF,
				},
			},
		},
	}
}
