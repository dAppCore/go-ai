// SPDX-License-Identifier: EUPL-1.2

package mcp

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	api "forge.lthn.ai/core/go-api"
)

// BridgeToAPI populates a go-api ToolBridge from recorded MCP tools.
// Each tool becomes a POST endpoint that reads a JSON body, dispatches
// to the tool's RESTHandler (which knows the concrete input type), and
// wraps the result in the standard api.Response envelope.
func BridgeToAPI(svc *Service, bridge *api.ToolBridge) {
	for _, rec := range svc.Tools() {
		desc := api.ToolDescriptor{
			Name:         rec.Name,
			Description:  rec.Description,
			Group:        rec.Group,
			InputSchema:  rec.InputSchema,
			OutputSchema: rec.OutputSchema,
		}

		// Capture the handler for the closure.
		handler := rec.RESTHandler

		bridge.Add(desc, func(c *gin.Context) {
			var body []byte
			if c.Request.Body != nil {
				var err error
				body, err = io.ReadAll(c.Request.Body)
				if err != nil {
					c.JSON(http.StatusBadRequest, api.Fail("invalid_request", "Failed to read request body"))
					return
				}
			}

			result, err := handler(c.Request.Context(), body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, api.Fail("tool_error", err.Error()))
				return
			}

			c.JSON(http.StatusOK, api.OK(result))
		})
	}
}
