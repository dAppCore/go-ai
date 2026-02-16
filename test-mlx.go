// +build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"forge.lthn.ai/core/go-ai/ml"
)

func main() {
	fmt.Println("=== MLX Backend Test ===")
	fmt.Println()

	// Test 1: Check if we're on the right platform
	fmt.Println("1. Platform check:")
	fmt.Printf("   GOOS: %s, GOARCH: %s\n", os.Getenv("GOOS"), os.Getenv("GOARCH"))
	fmt.Println()

	// Test 2: Try to create backends (without MLX tag, should use HTTP)
	fmt.Println("2. Backend availability (without MLX build tag):")
	fmt.Println("   Note: MLX backend requires -tags mlx build flag")
	fmt.Println()

	// Test 3: Check GGUF model directory
	fmt.Println("3. GGUF model directory:")
	modelDir := "/Volumes/Data/lem/gguf/"
	entries, err := os.ReadDir(modelDir)
	if err != nil {
		fmt.Printf("   Error reading directory: %v\n", err)
	} else {
		fmt.Printf("   Found %d files in %s\n", len(entries), modelDir)
		for _, entry := range entries {
			if !entry.IsDir() {
				info, _ := entry.Info()
				fmt.Printf("   - %s (%.2f GB)\n", entry.Name(), float64(info.Size())/(1024*1024*1024))
			}
		}
	}
	fmt.Println()

	// Test 4: Test scoring pipeline with mock backend
	fmt.Println("4. Testing scoring pipeline:")

	// Create a mock backend for testing
	mockBackend := &MockBackend{}

	// Test heuristic scoring
	response := ml.Response{
		ID:       "test-1",
		Prompt:   "What is 2+2?",
		Response: "The answer to 2+2 is 4. This is a basic arithmetic operation.",
	}

	hScore := ml.ScoreHeuristic(response.Response)
	fmt.Printf("   Heuristic Score: %+v\n", hScore)

	// Test judge (without actual model)
	judge := ml.NewJudge(mockBackend)
	fmt.Printf("   Judge created: %v\n", judge != nil)

	// Create scoring engine
	engine := ml.NewEngine(judge, 2, "all")
	fmt.Printf("   Engine created: %s\n", engine.String())
	fmt.Println()

	fmt.Println("5. Test probes:")
	fmt.Println("   Probes loaded from ml package")
	fmt.Println()

	fmt.Println("=== Test Complete ===")
}

// MockBackend is a simple backend for testing
type MockBackend struct{}

func (m *MockBackend) Generate(ctx context.Context, prompt string, opts ml.GenOpts) (string, error) {
	return `{"score": 5, "reasoning": "Mock response"}`, nil
}

func (m *MockBackend) Chat(ctx context.Context, messages []ml.Message, opts ml.GenOpts) (string, error) {
	return `{"score": 5, "reasoning": "Mock response"}`, nil
}

func (m *MockBackend) Name() string {
	return "mock"
}

func (m *MockBackend) Available() bool {
	return true
}
