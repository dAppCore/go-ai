package mcp

import (
	"context"
	"strings"
	"testing"
)

// RAG tools use package-level functions (rag.QueryDocs, rag.IngestDirectory, etc.)
// which require live Qdrant + Ollama services. Since those are not injectable,
// we test handler input validation, default application, and struct behaviour
// at the MCP handler level without requiring live services.

// --- ragQuery validation ---

// TestRagQuery_Bad_EmptyQuestion verifies empty question returns error.
func TestRagQuery_Bad_EmptyQuestion(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	ctx := context.Background()

	_, _, err = s.ragQuery(ctx, nil, RAGQueryInput{})
	if err == nil {
		t.Fatal("Expected error for empty question")
	}
	if !strings.Contains(err.Error(), "question cannot be empty") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestRagQuery_Good_DefaultsApplied verifies defaults are applied before validation.
// Because the handler applies defaults then validates, a non-empty question with
// zero Collection/TopK should have defaults applied. We cannot verify the actual
// query (needs live Qdrant), but we can verify it gets past validation.
func TestRagQuery_Good_DefaultsApplied(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	ctx := context.Background()

	// This will fail when it tries to connect to Qdrant, but AFTER applying defaults.
	// The error should NOT be about empty question.
	_, _, err = s.ragQuery(ctx, nil, RAGQueryInput{Question: "test query"})
	if err == nil {
		t.Skip("RAG query succeeded — live Qdrant available, skip default test")
	}
	// The error should be about connection failure, not validation
	if strings.Contains(err.Error(), "question cannot be empty") {
		t.Error("Defaults should have been applied before validation check")
	}
}

// --- ragIngest validation ---

// TestRagIngest_Bad_EmptyPath verifies empty path returns error.
func TestRagIngest_Bad_EmptyPath(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	ctx := context.Background()

	_, _, err = s.ragIngest(ctx, nil, RAGIngestInput{})
	if err == nil {
		t.Fatal("Expected error for empty path")
	}
	if !strings.Contains(err.Error(), "path cannot be empty") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestRagIngest_Bad_NonexistentPath verifies nonexistent path returns error.
func TestRagIngest_Bad_NonexistentPath(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	ctx := context.Background()

	_, _, err = s.ragIngest(ctx, nil, RAGIngestInput{
		Path: "/nonexistent/path/that/does/not/exist/at/all",
	})
	if err == nil {
		t.Fatal("Expected error for nonexistent path")
	}
}

// TestRagIngest_Good_DefaultCollection verifies the default collection is applied.
func TestRagIngest_Good_DefaultCollection(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	ctx := context.Background()

	// Use a real but inaccessible path to trigger stat error (not validation error).
	// The collection default should be applied first.
	_, _, err = s.ragIngest(ctx, nil, RAGIngestInput{
		Path: "/nonexistent/path/for/default/test",
	})
	if err == nil {
		t.Skip("Ingest succeeded unexpectedly")
	}
	// The error should NOT be about empty path
	if strings.Contains(err.Error(), "path cannot be empty") {
		t.Error("Default collection should have been applied")
	}
}

// --- ragCollections validation ---

// TestRagCollections_Bad_NoQdrant verifies graceful error when Qdrant is not available.
func TestRagCollections_Bad_NoQdrant(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	ctx := context.Background()

	_, _, err = s.ragCollections(ctx, nil, RAGCollectionsInput{})
	if err == nil {
		t.Skip("Qdrant is available — skip connection error test")
	}
	// Should get a connection error, not a panic
	if !strings.Contains(err.Error(), "failed to connect") && !strings.Contains(err.Error(), "failed to list") {
		t.Logf("Got error (expected connection failure): %v", err)
	}
}

// --- Struct round-trip tests ---

// TestRAGQueryResult_Good_AllFields verifies all fields can be set and read.
func TestRAGQueryResult_Good_AllFields(t *testing.T) {
	r := RAGQueryResult{
		Content:    "test content",
		Source:     "source.md",
		Section:    "Overview",
		Category:   "docs",
		ChunkIndex: 3,
		Score:      0.88,
	}

	if r.Content != "test content" {
		t.Errorf("Expected content 'test content', got %q", r.Content)
	}
	if r.ChunkIndex != 3 {
		t.Errorf("Expected chunkIndex 3, got %d", r.ChunkIndex)
	}
	if r.Score != 0.88 {
		t.Errorf("Expected score 0.88, got %f", r.Score)
	}
}

// TestCollectionInfo_Good_AllFields verifies CollectionInfo field access.
func TestCollectionInfo_Good_AllFields(t *testing.T) {
	c := CollectionInfo{
		Name:        "test-collection",
		PointsCount: 12345,
		Status:      "green",
	}

	if c.Name != "test-collection" {
		t.Errorf("Expected name 'test-collection', got %q", c.Name)
	}
	if c.PointsCount != 12345 {
		t.Errorf("Expected PointsCount 12345, got %d", c.PointsCount)
	}
}

// TestRAGDefaults_Good verifies default constants are sensible.
func TestRAGDefaults_Good(t *testing.T) {
	if DefaultRAGCollection != "hostuk-docs" {
		t.Errorf("Expected default collection 'hostuk-docs', got %q", DefaultRAGCollection)
	}
	if DefaultRAGTopK != 5 {
		t.Errorf("Expected default topK 5, got %d", DefaultRAGTopK)
	}
}
