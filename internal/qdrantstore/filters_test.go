package qdrantstore

import (
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// buildSchemaPrompt
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildSchemaPrompt_ContainsSchemaName(t *testing.T) {
	prompt := buildSchemaPrompt(ElogSchema, "what did Jane do last week")
	if !contains(prompt, "elog") {
		t.Error("prompt should mention the schema name")
	}
}

func TestBuildSchemaPrompt_ContainsAllFields(t *testing.T) {
	prompt := buildSchemaPrompt(ElogSchema, "test query")
	for key := range ElogSchema.Fields {
		if !contains(prompt, key) {
			t.Errorf("prompt missing field %q", key)
		}
	}
}

func TestBuildSchemaPrompt_ContainsAliases(t *testing.T) {
	prompt := buildSchemaPrompt(ElogSchema, "test")
	if !contains(prompt, "ops") || !contains(prompt, "Operations") {
		t.Error("prompt should show aliases")
	}
}

func TestBuildSchemaPrompt_DifferentSchemas(t *testing.T) {
	elogPrompt := buildSchemaPrompt(ElogSchema, "q")
	imgPrompt := buildSchemaPrompt(ImageSchema, "q")
	if elogPrompt == imgPrompt {
		t.Error("different schemas must produce different prompts")
	}
	if !contains(imgPrompt, "photographer") {
		t.Error("image prompt should mention photographer field")
	}
	if !contains(imgPrompt, "camera_model") {
		t.Error("image prompt should mention camera_model field")
	}
}

func TestBuildSchemaPrompt_SPARQLSchema(t *testing.T) {
	prompt := buildSchemaPrompt(SPARQLNodeSchema, "find Person nodes")
	if !contains(prompt, "rdf_type") {
		t.Error("SPARQL prompt should mention rdf_type")
	}
	if !contains(prompt, "predicate") {
		t.Error("SPARQL prompt should mention predicate")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// BuildDynamicFilter
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildDynamicFilter_NilWhenEmpty(t *testing.T) {
	f := BuildDynamicFilter(ElogSchema, QueryParams{})
	if f != nil {
		t.Error("empty params should return nil filter")
	}
}

func TestBuildDynamicFilter_NilWhenOnlyQueryKey(t *testing.T) {
	// The "query" key is semantic text, not a filter condition.
	f := BuildDynamicFilter(ElogSchema, QueryParams{"query": "beam degraded"})
	if f != nil {
		t.Error("only the query key should return nil filter")
	}
}

func TestBuildDynamicFilter_AuthorCondition(t *testing.T) {
	f := BuildDynamicFilter(ElogSchema, QueryParams{"author": "Jane Smith"})
	if f == nil {
		t.Fatal("expected non-nil filter for author")
	}
	if len(f.Must) != 1 {
		t.Fatalf("expected 1 must condition, got %d", len(f.Must))
	}
	field := f.Must[0].GetField()
	if field == nil || field.Key != "author" {
		t.Errorf("expected field key 'author', got %+v", field)
	}
}

func TestBuildDynamicFilter_CategoryCondition(t *testing.T) {
	f := BuildDynamicFilter(ElogSchema, QueryParams{"category": "LINAC"})
	if f == nil || len(f.Must) != 1 {
		t.Fatal("expected 1 condition for category")
	}
	if f.Must[0].GetField().Key != "category" {
		t.Error("wrong field key for category")
	}
}

func TestBuildDynamicFilter_DateRange(t *testing.T) {
	f := BuildDynamicFilter(ElogSchema, QueryParams{
		"date_from": "2021-10-01T00:00:00Z",
		"date_to":   "2021-11-01T00:00:00Z",
	})
	if f == nil {
		t.Fatal("expected non-nil filter for date range")
	}
	if len(f.Must) != 1 {
		t.Fatalf("expected 1 range condition, got %d", len(f.Must))
	}
	r := f.Must[0].GetField().Range
	if r == nil {
		t.Fatal("expected range condition")
	}
	if r.Gte == nil || r.Lt == nil {
		t.Error("both Gte and Lt should be set for a full date range")
	}
	if *r.Gte >= *r.Lt {
		t.Error("Gte should be less than Lt")
	}
}

func TestBuildDynamicFilter_OpenDateRange_FromOnly(t *testing.T) {
	f := BuildDynamicFilter(ElogSchema, QueryParams{
		"date_from": "2021-10-01T00:00:00Z",
	})
	if f == nil {
		t.Fatal("expected filter for date_from only")
	}
	r := f.Must[0].GetField().Range
	if r.Gte == nil {
		t.Error("Gte should be set")
	}
	if r.Lt != nil {
		t.Error("Lt should be nil for open upper bound")
	}
}

func TestBuildDynamicFilter_MultipleConditions(t *testing.T) {
	f := BuildDynamicFilter(ElogSchema, QueryParams{
		"author":    "Jane Smith",
		"category":  "LINAC",
		"date_from": "2021-10-01T00:00:00Z",
		"date_to":   "2021-11-01T00:00:00Z",
	})
	if f == nil {
		t.Fatal("expected non-nil filter")
	}
	if len(f.Must) != 3 {
		t.Errorf("expected 3 conditions (author + category + date range), got %d", len(f.Must))
	}
}

func TestBuildDynamicFilter_ImageSchema(t *testing.T) {
	f := BuildDynamicFilter(ImageSchema, QueryParams{
		"photographer": "Ansel Adams",
		"camera_model": "Linhof",
	})
	if f == nil || len(f.Must) != 2 {
		t.Fatalf("expected 2 conditions for image schema, got %v", f)
	}
}

func TestBuildDynamicFilter_SPARQLSchema(t *testing.T) {
	f := BuildDynamicFilter(SPARQLNodeSchema, QueryParams{
		"rdf_type": "http://schema.org/Person",
		"graph":    "http://example.org/graph1",
	})
	if f == nil || len(f.Must) != 2 {
		t.Fatalf("expected 2 conditions for SPARQL schema, got %v", f)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Alias normalisation
// ─────────────────────────────────────────────────────────────────────────────

func TestExtractQueryParamsNormalisesAliases(t *testing.T) {
	// Simulate the alias normalisation that happens after LLM extraction
	// without making a real LLM call.
	raw := QueryParams{"category": "ops"}
	schema := ElogSchema

	for jsonKey, spec := range schema.Fields {
		if v, ok := raw[jsonKey]; ok && len(spec.Aliases) > 0 {
			lower := v
			if canonical, found := spec.Aliases[lower]; found {
				raw[jsonKey] = canonical
			}
		}
	}

	if raw["category"] != "Operations" {
		t.Errorf("alias 'ops' should normalise to 'Operations', got %q", raw["category"])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// parseDate
// ─────────────────────────────────────────────────────────────────────────────

func TestParseDate_RFC3339(t *testing.T) {
	ts := parseDate("2021-10-15T11:42:53Z")
	if ts == 0 {
		t.Error("expected non-zero Unix timestamp for valid RFC3339")
	}
}

func TestParseDate_Empty(t *testing.T) {
	if parseDate("") != 0 {
		t.Error("empty string should return 0")
	}
}

func TestParseDate_Invalid(t *testing.T) {
	if parseDate("not-a-date") != 0 {
		t.Error("invalid date should return 0")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// MatchString / MatchBool / MatchInt / DateRangeTS
// ─────────────────────────────────────────────────────────────────────────────

func TestMatchString_FieldKey(t *testing.T) {
	c := MatchString("author", "Jane")
	f := c.GetField()
	if f == nil || f.Key != "author" {
		t.Errorf("expected field key 'author', got %+v", f)
	}
	if f.Match.GetKeyword() != "Jane" {
		t.Errorf("expected keyword 'Jane', got %q", f.Match.GetKeyword())
	}
}

func TestMatchBool(t *testing.T) {
	c := MatchBool("has_plot", true)
	f := c.GetField()
	if !f.Match.GetBoolean() {
		t.Error("expected boolean true")
	}
}

func TestMatchInt(t *testing.T) {
	c := MatchInt("chunk_index", 7)
	f := c.GetField()
	if f.Match.GetInteger() != 7 {
		t.Errorf("expected integer 7, got %d", f.Match.GetInteger())
	}
}

func TestDateRangeTS_BothBounds(t *testing.T) {
	c := DateRangeTS("date_ts", 1000, 2000)
	r := c.GetField().Range
	if r == nil || r.Gte == nil || r.Lt == nil {
		t.Fatal("expected both bounds set")
	}
	if *r.Gte != 1000 || *r.Lt != 2000 {
		t.Errorf("got Gte=%v Lt=%v", *r.Gte, *r.Lt)
	}
}

func TestDateRangeTS_OpenBounds(t *testing.T) {
	c := DateRangeTS("date_ts", 0, 0)
	r := c.GetField().Range
	if r.Gte != nil || r.Lt != nil {
		t.Error("zero bounds should produce nil pointers (open range)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// helper
// ─────────────────────────────────────────────────────────────────────────────

func contains(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
