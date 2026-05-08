// SPDX-License-Identifier: EUPL-1.2

package ai

import (
	"context"

	core "dappco.re/go"
	"dappco.re/go/inference"
)

func ExampleRAGContextAssembler() {
	assembler := RAGContextAssembler{
		Query: func(task TaskInfo) core.Result {
			return core.Ok(core.Concat("context for ", task.Title))
		},
	}

	contextText, _ := assembler.AssembleContext(context.Background(), []inference.Message{
		{Role: "user", Content: "build failure"},
	})
	core.Println(contextText)

	// Output:
	// context for build failure
}
