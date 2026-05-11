// SPDX-License-Identifier: EUPL-1.2

package ai

import (
	"context"

	core "dappco.re/go"
)

func ExampleNewProviderRouter() {
	routerResult := NewProviderRouter(ProviderRoute{
		Name:    "local",
		ModelID: "gemma-test",
		Model:   &routerFakeModel{modelType: "mlx", output: "hello from local"},
	})
	router := routerResult.Value.(*ProviderRouter)

	chatResult := router.Chat(context.Background(), ProviderChatRequest{Prompt: "hello"})
	response := chatResult.Value.(ProviderChatResponse)

	core.Println(response.Provider)
	core.Println(response.Text)
	// Output:
	// local
	// hello from local
}
