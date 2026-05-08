// SPDX-License-Identifier: EUPL-1.2

package openai

import (
	"context"

	core "dappco.re/go"
	"dappco.re/go/inference"
)

func ExampleNewBackend() {
	backend := NewBackend(Config{
		Name:         "openai",
		BaseURL:      "https://api.openai.com",
		DefaultModel: "gpt-4o-mini",
	})

	core.Println(backend.Name())
	core.Println(backend.Available())

	// Output:
	// openai
	// true
}

func ExampleContextAssemblerFunc() {
	assembler := ContextAssemblerFunc(func(ctx context.Context, messages []inference.Message) (string, error) {
		return "retrieved context", nil
	})
	contextText, _ := assembler.AssembleContext(context.Background(), nil)

	core.Println(contextText)

	// Output:
	// retrieved context
}
