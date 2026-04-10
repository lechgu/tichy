package qdrantstore

// filters.go — generic, schema-driven filter construction for Qdrant.
//
// Design goals
// ============
//  1. Schema-agnostic: the filter builder knows nothing about elogs, images,
//     or SPARQL.  Callers describe their payload schema via FilterSchema and
//     the builder does the rest.
//  2. LLM-driven extraction: a single ExtractQueryParams call works for any
//     domain.  The prompt is generated from the schema, so adding a new use-
//     case only requires defining a new FilterSchema — no code changes needed.
//  3. Open for extension: adding a new field type (e.g. geo, nested object)
//     means adding a FieldKind constant and one case in BuildDynamicFilter.
//  4. No elog hard-coding: the collection-name string-match in retriever.go
//     is replaced by attaching a FilterSchema to the retriever at wire-up
//     time.
//
// Use-case examples
// =================
//
//   ElogSchema        — author, category, date range (elog domain)
//   ImageSchema       — photographer, camera model, date taken, tags
//   SPARQLNodeSchema  — rdf:type, subject URI, predicate, graph name
//   CustomSchema      — any map[string]FieldSpec you define at runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/qdrant/go-client/qdrant"
)

// ─────────────────────────────────────────────────────────────────────────────
// Schema definition types
// ─────────────────────────────────────────────────────────────────────────────

// FieldKind describes how a payload field is stored in Qdrant and therefore
// how it must appear in a filter condition.
type FieldKind int

const (
	// KindKeyword — exact string match (MERGE on keyword index).
	// Use for categorical fields: author, category, type, tag, …
	KindKeyword FieldKind = iota

	// KindDateRange — a pair of ISO8601 strings that map to a Unix-timestamp
	// range condition.  The LLM is asked to emit date_from / date_to.
	// The payload field name is expected to be a Unix int64 (e.g. "date_ts").
	KindDateRange

	// KindBool — boolean match.
	KindBool

	// KindInteger — exact integer match (e.g. chunk_index, version).
	KindInteger
)

// FieldSpec describes one filterable field in the payload schema.
type FieldSpec struct {
	// PayloadKey is the exact key stored in the Qdrant point payload.
	PayloadKey string

	// Kind determines which Qdrant condition is generated.
	Kind FieldKind

	// Description is included verbatim in the LLM prompt so the model
	// understands what values are expected.  Be specific:
	//   "person's full name as it appears in log author field"
	//   "one of [Operations, Vacuum, RF, LINAC, Ring]"
	Description string

	// Aliases maps natural-language shortcuts to canonical values.
	// E.g. {"ops": "Operations", "vac": "Vacuum"}
	// The LLM-extracted value is normalised through this map before filtering.
	Aliases map[string]string
}

// FilterSchema is the complete description of one collection's filterable
// payload.  Attach one to a QdrantRetriever at construction time.
type FilterSchema struct {
	// Name is a short human-readable label used in prompts and logs.
	Name string

	// Fields defines every filterable payload field.
	// Map key = the JSON key the LLM should emit for this field.
	// (May differ from PayloadKey when the LLM-facing name is more natural.)
	Fields map[string]FieldSpec
}

// ─────────────────────────────────────────────────────────────────────────────
// Pre-built schemas
// ─────────────────────────────────────────────────────────────────────────────

// ElogSchema covers the CESR elog payload produced by cesr-elog-parser.
var ElogSchema = FilterSchema{
	Name: "elog",
	Fields: map[string]FieldSpec{
		"author": {
			PayloadKey:  "author",
			Kind:        KindKeyword,
			Description: "full name of the person who wrote the log entry",
		},
		"category": {
			PayloadKey:  "category",
			Kind:        KindKeyword,
			Description: `one of [Operations, Vacuum, RF, LINAC, Ring, BPM, Cryo, Magnet]`,
			Aliases:     map[string]string{"ops": "Operations", "vac": "Vacuum"},
		},
		"date_from": {
			PayloadKey:  "date_ts",
			Kind:        KindDateRange,
			Description: "start of date range, ISO8601 (YYYY-MM-DDTHH:MM:SSZ)",
		},
		"date_to": {
			PayloadKey:  "date_ts",
			Kind:        KindDateRange,
			Description: "end of date range, ISO8601 (YYYY-MM-DDTHH:MM:SSZ)",
		},
	},
}

// ImageSchema covers an image-metadata collection where each point stores
// EXIF-derived fields.
var ImageSchema = FilterSchema{
	Name: "image",
	Fields: map[string]FieldSpec{
		"photographer": {
			PayloadKey:  "photographer",
			Kind:        KindKeyword,
			Description: "name of the photographer or camera owner",
		},
		"camera_model": {
			PayloadKey:  "camera_model",
			Kind:        KindKeyword,
			Description: "camera model string from EXIF, e.g. 'Canon EOS R5'",
		},
		"tag": {
			PayloadKey:  "tag",
			Kind:        KindKeyword,
			Description: "one descriptive tag applied to the image, e.g. 'landscape', 'portrait'",
		},
		"date_from": {
			PayloadKey:  "date_ts",
			Kind:        KindDateRange,
			Description: "start of capture date range, ISO8601",
		},
		"date_to": {
			PayloadKey:  "date_ts",
			Kind:        KindDateRange,
			Description: "end of capture date range, ISO8601",
		},
	},
}

// SPARQLNodeSchema covers a knowledge-graph node collection where each point
// represents an RDF resource and its textual description.  This supports the
// Fabric node / SPARQL translation use-case.
var SPARQLNodeSchema = FilterSchema{
	Name: "sparql_node",
	Fields: map[string]FieldSpec{
		"rdf_type": {
			PayloadKey:  "rdf_type",
			Kind:        KindKeyword,
			Description: "RDF rdf:type URI of the resource, e.g. 'http://schema.org/Person'",
		},
		"graph": {
			PayloadKey:  "graph",
			Kind:        KindKeyword,
			Description: "named graph the triple belongs to",
		},
		"predicate": {
			PayloadKey:  "predicate",
			Kind:        KindKeyword,
			Description: "RDF predicate URI, e.g. 'http://schema.org/name'",
		},
		"date_from": {
			PayloadKey:  "date_ts",
			Kind:        KindDateRange,
			Description: "start of the assertion date range, ISO8601",
		},
		"date_to": {
			PayloadKey:  "date_ts",
			Kind:        KindDateRange,
			Description: "end of the assertion date range, ISO8601",
		},
	},
}

// ─────────────────────────────────────────────────────────────────────────────
// LLM-based parameter extraction
// ─────────────────────────────────────────────────────────────────────────────

// QueryParams is a generic string map that holds the LLM-extracted values.
// Keys match the map keys in FilterSchema.Fields.
type QueryParams map[string]string

// buildSchemaPrompt generates an LLM prompt from the schema so no manual
// prompt maintenance is needed when schemas change.
func buildSchemaPrompt(schema FilterSchema, userQuery string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(
		"You are a query parser for a %s search system.\n\n"+
			"Extract structured filters from the user query.\n\n"+
			"Return ONLY valid JSON with these optional fields:\n\n",
		schema.Name,
	))

	sb.WriteString("- query: the main semantic search text (always include this)\n")
	for jsonKey, spec := range schema.Fields {
		aliasNote := ""
		if len(spec.Aliases) > 0 {
			pairs := make([]string, 0, len(spec.Aliases))
			for k, v := range spec.Aliases {
				pairs = append(pairs, fmt.Sprintf("%q → %q", k, v))
			}
			aliasNote = fmt.Sprintf(" (aliases: %s)", strings.Join(pairs, ", "))
		}
		sb.WriteString(fmt.Sprintf("- %s: %s%s\n", jsonKey, spec.Description, aliasNote))
	}

	sb.WriteString("\nRules:\n")
	sb.WriteString("- Omit any field that cannot be inferred from the query.\n")
	sb.WriteString("- Convert natural language dates (e.g. 'last week', 'April 2025') to ISO8601 ranges.\n")
	sb.WriteString("- Return only the JSON object, no markdown fences, no explanation.\n")
	sb.WriteString("\nUser query:\n")
	sb.WriteString(userQuery)

	return sb.String()
}

// ExtractQueryParams calls the LLM with a schema-derived prompt and returns
// the extracted parameters.  Works for any FilterSchema.
func ExtractQueryParams(ctx context.Context, llmURL string, schema FilterSchema, userQuery string) (QueryParams, error) {
	client := openai.NewClient(
		option.WithBaseURL(llmURL+"/v1"),
		option.WithAPIKey("not-needed"),
	)

	prompt := buildSchemaPrompt(schema, userQuery)

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4o,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You extract structured JSON from user queries. Return only valid JSON."),
			openai.UserMessage(prompt),
		},
		Temperature: openai.Float(0),
	})
	if err != nil {
		return nil, err
	}

	content := extractJSON(resp.Choices[0].Message.Content)

	var raw map[string]string
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("filter extraction: unmarshal: %w", err)
	}

	// Normalise aliases defined in the schema.
	for jsonKey, spec := range schema.Fields {
		if v, ok := raw[jsonKey]; ok && len(spec.Aliases) > 0 {
			if canonical, found := spec.Aliases[strings.ToLower(v)]; found {
				raw[jsonKey] = canonical
			}
		}
	}

	return QueryParams(raw), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Filter construction
// ─────────────────────────────────────────────────────────────────────────────

// BuildDynamicFilter translates QueryParams into a Qdrant filter using the
// field specifications from the supplied schema.  Returns nil when no
// filter conditions can be derived (caller should omit the filter entirely).
func BuildDynamicFilter(schema FilterSchema, params QueryParams) *qdrant.Filter {
	var must []*qdrant.Condition

	// Collect date_from / date_to values so they can be emitted as a single
	// range condition rather than two separate conditions.
	dateRangeFields := map[string]string{} // payloadKey → "from" or "to" accumulator

	for jsonKey, spec := range schema.Fields {
		val, ok := params[jsonKey]
		if !ok || val == "" || val == "query" {
			continue
		}

		switch spec.Kind {
		case KindKeyword:
			must = append(must, MatchString(spec.PayloadKey, val))

		case KindBool:
			b := strings.ToLower(val) == "true"
			must = append(must, MatchBool(spec.PayloadKey, b))

		case KindInteger:
			var n int64
			fmt.Sscan(val, &n)
			must = append(must, MatchInt(spec.PayloadKey, n))

		case KindDateRange:
			// Accumulate from/to by payload key; emit as one range at the end.
			// Convention: the JSON key must end in "_from" or "_to".
			if strings.HasSuffix(jsonKey, "_from") {
				dateRangeFields[spec.PayloadKey+"__from"] = val
			} else if strings.HasSuffix(jsonKey, "_to") {
				dateRangeFields[spec.PayloadKey+"__to"] = val
			}
		}
	}

	// Emit accumulated date range conditions.
	// A single from or to is fine — the other bound is treated as open.
	emitted := map[string]bool{}
	for jsonKey, spec := range schema.Fields {
		if spec.Kind != KindDateRange || emitted[spec.PayloadKey] {
			continue
		}
		_ = jsonKey
		fromVal := dateRangeFields[spec.PayloadKey+"__from"]
		toVal := dateRangeFields[spec.PayloadKey+"__to"]
		if fromVal == "" && toVal == "" {
			continue
		}
		must = append(must, DateRangeTS(spec.PayloadKey, parseDate(fromVal), parseDate(toVal)))
		emitted[spec.PayloadKey] = true
	}

	if len(must) == 0 {
		return nil
	}
	return &qdrant.Filter{Must: must}
}

// ─────────────────────────────────────────────────────────────────────────────
// Low-level Qdrant condition helpers
// ─────────────────────────────────────────────────────────────────────────────

// MatchString builds a keyword match condition.
func MatchString(key, value string) *qdrant.Condition {
	return &qdrant.Condition{
		ConditionOneOf: &qdrant.Condition_Field{
			Field: &qdrant.FieldCondition{
				Key: key,
				Match: &qdrant.Match{
					MatchValue: &qdrant.Match_Keyword{Keyword: value},
				},
			},
		},
	}
}

// MatchBool builds a boolean match condition.
func MatchBool(key string, val bool) *qdrant.Condition {
	return &qdrant.Condition{
		ConditionOneOf: &qdrant.Condition_Field{
			Field: &qdrant.FieldCondition{
				Key: key,
				Match: &qdrant.Match{
					MatchValue: &qdrant.Match_Boolean{Boolean: val},
				},
			},
		},
	}
}

// MatchInt builds an exact integer match condition.
func MatchInt(key string, val int64) *qdrant.Condition {
	return &qdrant.Condition{
		ConditionOneOf: &qdrant.Condition_Field{
			Field: &qdrant.FieldCondition{
				Key: key,
				Match: &qdrant.Match{
					MatchValue: &qdrant.Match_Integer{Integer: val},
				},
			},
		},
	}
}

// DateRangeTS builds a numeric range condition on a Unix-timestamp field.
// Pass 0 for from or to to leave that bound open.
func DateRangeTS(key string, from, to int64) *qdrant.Condition {
	r := &qdrant.Range{}
	if from != 0 {
		f := float64(from)
		r.Gte = &f
	}
	if to != 0 {
		t := float64(to)
		r.Lt = &t
	}
	return &qdrant.Condition{
		ConditionOneOf: &qdrant.Condition_Field{
			Field: &qdrant.FieldCondition{
				Key:   key,
				Range: r,
			},
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal utilities
// ─────────────────────────────────────────────────────────────────────────────

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end <= start {
		return s
	}
	return s[start : end+1]
}

// parseDate converts an ISO8601 or RFC1123 string to a Unix timestamp.
// Returns 0 when the string is empty or unparseable (treated as open bound).
func parseDate(s string) int64 {
	if s == "" {
		return 0
	}
	formats := []string{
		time.RFC3339,
		time.RFC1123Z,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 -0700",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.Unix()
		}
	}
	return 0
}
