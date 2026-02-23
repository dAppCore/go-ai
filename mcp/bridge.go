// SPDX-License-Identifier: EUPL-1.2

package mcp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	api "forge.lthn.ai/core/go-api"
)

// maxBodySize is the maximum request body size accepted by bridged tool endpoints.
const maxBodySize = 10 << 20 // 10 MB

// BridgeToAPI populates a go-api ToolBridge from recorded MCP tools.
// Each tool becomes a POST endpoint that reads a JSON body, dispatches
// to the tool's RESTHandler (which knows the concrete input type), and
// wraps the result in the standard api.Response envelope.
func BridgeToAPI(svc *Service, bridge *api.ToolBridge) {
	for rec := range svc.ToolsSeq() {
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
				body, err = io.ReadAll(io.LimitReader(c.Request.Body, maxBodySize))
				if err != nil {
					c.JSON(http.StatusBadRequest, api.Fail("invalid_request", "Failed to read request body"))
					return
				}
			}

			result, err := handler(c.Request.Context(), body)
			if err != nil {
				// Classify JSON parse errors as client errors (400),
				// everything else as server errors (500).
				var syntaxErr *json.SyntaxError
				var typeErr *json.UnmarshalTypeError
				if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
					c.JSON(http.StatusBadRequest, api.Fail("invalid_input", "Malformed JSON in request body"))
					return
				}
				c.JSON(http.StatusInternalServerError, api.Fail("tool_error", "Tool execution failed"))
				return
			}

			c.JSON(http.StatusOK, api.OK(result))
		})
	}
}
