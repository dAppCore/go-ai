// SPDX-Licence-Identifier: EUPL-1.2

package lab

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	core "dappco.re/go"
	coreapi "dappco.re/go/api"
	"github.com/gin-gonic/gin"
)

// Group is the coreapi.RouteGroup that exposes LabService over HTTP
// under /v1/ml-lab/*. Reachable from the WebView via apiFetch and
// from any OpenAPI consumer via the gateway. Wails bindings cover
// in-process direct access from the WebView; this group covers
// out-of-process consumers (CLI clients, plugins, remote nodes).
//
// Mounted by RegisterHTTPRoutes(c) which looks up both the api
// Service and the lab Service from core and attaches the group to
// the gin engine. Returns Ok with nil when the api Service isn't
// registered on this Core (headless build, no HTTP surface).
type Group struct {
	svc *Service
}

// NewGroup builds a Group bound to svc. svc MUST be non-nil — the
// handlers would otherwise panic on first call.
//
// Usage example:
//
//	g := lab.NewGroup(svc)
//	engine.Register(g)
func NewGroup(svc *Service) *Group {
	return &Group{svc: svc}
}

// Name satisfies coreapi.RouteGroup. Surfaces in the OpenAPI tag
// list + admin diagnostics.
func (g *Group) Name() string { return "ml-lab" }

// BasePath satisfies coreapi.RouteGroup. Versioned under /v1 so
// breaking changes can stage behind /v2.
func (g *Group) BasePath() string { return "/v1" }

// RegisterRoutes mounts every ml-lab endpoint onto rg. Called
// once at boot by coreapi.Engine when the group is registered.
func (g *Group) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/ml-lab/models", g.handleListModels)
	rg.GET("/ml-lab/runs", g.handleListRuns)
	rg.POST("/ml-lab/runs/start", g.handleStartRun)
	rg.POST("/ml-lab/runs/:id/control", g.handleControlRun)
	rg.GET("/ml-lab/runs/:id/events", g.handleRunEvents)
	rg.POST("/ml-lab/inspect", g.handleInspect)
	rg.POST("/ml-lab/lql", g.handleLQL)
	rg.POST("/ml-lab/influx", g.handleInflux)
	rg.POST("/ml-lab/duckdb", g.handleDuckDB)
	rg.POST("/ml-lab/ask", g.handleAsk)
	rg.GET("/ml-lab/saved-queries", g.handleListSaved)
	rg.POST("/ml-lab/saved-queries", g.handleSaveQuery)
	rg.DELETE("/ml-lab/saved-queries/:id", g.handleDeleteSaved)
}

// Describe satisfies coreapi.DescribableGroup. Per-route shapes
// follow the same dictionary pattern runner_group.Describe uses so
// the OpenAPI generator builds a consistent spec. Kept minimal —
// expansion happens as routes settle.
func (g *Group) Describe() []coreapi.RouteDescription {
	return []coreapi.RouteDescription{
		{Method: "GET", Path: "/ml-lab/models", Summary: "List local .model / .train / .vindex artefacts", Tags: []string{"ml-lab"}, StatusCode: 200},
		{Method: "GET", Path: "/ml-lab/runs", Summary: "List LoRA training runs (active + history)", Tags: []string{"ml-lab"}, StatusCode: 200},
		{Method: "POST", Path: "/ml-lab/runs/start", Summary: "Start a new LoRA training run", Tags: []string{"ml-lab"}, StatusCode: 200},
		{Method: "POST", Path: "/ml-lab/runs/:id/control", Summary: "Pause / resume / cancel / checkpoint a run", Tags: []string{"ml-lab"}, StatusCode: 200},
		{Method: "GET", Path: "/ml-lab/runs/:id/events", Summary: "SSE stream of run events (tick / complete / error / checkpoint)", Tags: []string{"ml-lab"}, StatusCode: 200},
		{Method: "POST", Path: "/ml-lab/inspect", Summary: "Inspect a .model / .train / .vindex pack", Tags: []string{"ml-lab"}, StatusCode: 200},
		{Method: "POST", Path: "/ml-lab/lql", Summary: "Run one LQL statement", Tags: []string{"ml-lab"}, StatusCode: 200},
		{Method: "POST", Path: "/ml-lab/influx", Summary: "Run a Flux / InfluxQL query", Tags: []string{"ml-lab"}, StatusCode: 200},
		{Method: "POST", Path: "/ml-lab/duckdb", Summary: "Run a DuckDB SELECT", Tags: []string{"ml-lab"}, StatusCode: 200},
		{Method: "POST", Path: "/ml-lab/ask", Summary: "Rewrite natural-language ASK into a target-tab editor body", Tags: []string{"ml-lab"}, StatusCode: 200},
		{Method: "GET", Path: "/ml-lab/saved-queries", Summary: "List saved queries (optionally filtered by tab)", Tags: []string{"ml-lab"}, StatusCode: 200},
		{Method: "POST", Path: "/ml-lab/saved-queries", Summary: "Save / upsert a query", Tags: []string{"ml-lab"}, StatusCode: 200},
		{Method: "DELETE", Path: "/ml-lab/saved-queries/:id", Summary: "Delete a saved query by id", Tags: []string{"ml-lab"}, StatusCode: 200},
	}
}

// RegisterHTTPRoutes builds the Group bound to the named lab
// Service and attaches it to the coreapi engine. Returns Ok with
// nil when either the api Service or the lab Service isn't
// registered on this Core — those are valid configurations
// (headless build / partial composition) that callers shouldn't
// have to gate against.
//
// Called from pkg/desktop.mountSubsystems after the engine + lab
// Service are both registered. Idempotent w.r.t. duplicate calls
// in the same gin engine (gin allows route overrides; the second
// registration shadows the first).
//
// Usage example:
//
//	if rr := lab.RegisterHTTPRoutes(c); !rr.OK { return rr }
func RegisterHTTPRoutes(c *core.Core) core.Result {
	if c == nil {
		return core.Fail(core.E("lab.RegisterHTTPRoutes", "core is nil", nil))
	}
	apiSvc, ok := core.ServiceFor[*coreapi.Service](c, "api")
	if !ok || apiSvc == nil || apiSvc.Engine == nil {
		return core.Ok(nil)
	}
	labSvc, ok := core.ServiceFor[*Service](c, "lab")
	if !ok || labSvc == nil {
		return core.Ok(nil)
	}
	apiSvc.Engine.Register(NewGroup(labSvc))
	return core.Ok(nil)
}

// --- handlers ---

func (g *Group) handleListModels(c *gin.Context) {
	r := g.svc.ListModels()
	g.write(c, r)
}

func (g *Group) handleListRuns(c *gin.Context) {
	r := g.svc.ListRuns()
	g.write(c, r)
}

func (g *Group) handleStartRun(c *gin.Context) {
	var req StartRunRequest
	if !g.bind(c, &req) {
		return
	}
	g.write(c, g.svc.StartRun(req))
}

func (g *Group) handleControlRun(c *gin.Context) {
	id := c.Param("id")
	var req RunControl
	if !g.bind(c, &req) {
		return
	}
	if req.RunID == "" {
		req.RunID = id
	}
	g.write(c, g.svc.ControlRun(req))
}

// handleRunEvents streams RunEvents as Server-Sent Events. The
// frontend opens an EventSource on /v1/ml-lab/runs/:id/events and
// renders each frame as it lands.
func (g *Group) handleRunEvents(c *gin.Context) {
	id := c.Param("id")
	ch, err := g.svc.SubscribeRunEvents(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	// Open with a comment frame so the client knows the stream is
	// alive even before the first real event.
	_, _ = c.Writer.Write([]byte(":ok\n\n"))
	c.Writer.Flush()

	clientGone := c.Request.Context().Done()
	// Heartbeat every 15s so intermediaries don't time the stream
	// out on quiet runs (paused, between checkpoints, etc).
	heart := time.NewTicker(15 * time.Second)
	defer heart.Stop()

	c.Stream(func(w io.Writer) bool {
		select {
		case <-clientGone:
			return false
		case <-heart.C:
			_, err := w.Write([]byte(":hb\n\n"))
			if err != nil {
				return false
			}
			return true
		case ev, ok := <-ch:
			if !ok {
				// Driver closed the channel — emit a terminal frame
				// and stop.
				_, _ = w.Write([]byte("event: end\ndata: {}\n\n"))
				return false
			}
			buf, mErr := json.Marshal(ev)
			if mErr != nil {
				return false
			}
			if _, wErr := w.Write([]byte("data: ")); wErr != nil {
				return false
			}
			if _, wErr := w.Write(buf); wErr != nil {
				return false
			}
			if _, wErr := w.Write([]byte("\n\n")); wErr != nil {
				return false
			}
			return true
		}
	})
}

func (g *Group) handleInspect(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if !g.bind(c, &req) {
		return
	}
	g.write(c, g.svc.InspectModelPack(req.Path))
}

func (g *Group) handleLQL(c *gin.Context) {
	var req LQLRequest
	if !g.bind(c, &req) {
		return
	}
	g.write(c, g.svc.RunLQL(req))
}

func (g *Group) handleInflux(c *gin.Context) {
	var req InfluxQuery
	if !g.bind(c, &req) {
		return
	}
	g.write(c, g.svc.QueryInflux(req))
}

// handleDuckDB rejects mutating SQL at the entry — read-only
// enforcement before the request reaches the QueryDuckDB method.
func (g *Group) handleDuckDB(c *gin.Context) {
	var req DuckDBQuery
	if !g.bind(c, &req) {
		return
	}
	if !isReadOnlySQL(req.SQL) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "ml-lab/duckdb endpoint accepts SELECT/WITH/EXPLAIN only"})
		return
	}
	g.write(c, g.svc.QueryDuckDB(req))
}

func (g *Group) handleAsk(c *gin.Context) {
	var req AskRequest
	if !g.bind(c, &req) {
		return
	}
	g.write(c, g.svc.RewriteAsk(req))
}

func (g *Group) handleListSaved(c *gin.Context) {
	tab := c.Query("tab")
	g.write(c, g.svc.ListSavedQueries(tab))
}

func (g *Group) handleSaveQuery(c *gin.Context) {
	var req SavedQuery
	if !g.bind(c, &req) {
		return
	}
	g.write(c, g.svc.SaveQuery(req))
}

func (g *Group) handleDeleteSaved(c *gin.Context) {
	id := c.Param("id")
	g.write(c, g.svc.DeleteSavedQuery(id))
}

// --- helpers ---

// bind reads + decodes the JSON body into dst. Writes a 400 with
// an error envelope on failure and returns false so the handler
// short-circuits.
func (g *Group) bind(c *gin.Context, dst any) bool {
	if c.Request == nil || c.Request.Body == nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "missing request body"})
		return false
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 8<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "read body: " + err.Error()})
		return false
	}
	if len(body) == 0 {
		// Empty body is OK — caller passes zero-value DTO.
		return true
	}
	if err := json.Unmarshal(body, dst); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "decode body: " + err.Error()})
		return false
	}
	return true
}

// write serialises a core.Result into the canonical {ok, value} /
// {ok: false, error} JSON envelope every ml-lab handler returns.
func (g *Group) write(c *gin.Context, r core.Result) {
	if !r.OK {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": r.Error()})
		return
	}
	c.JSON(http.StatusOK, r.Value)
}

// isReadOnlySQL allows SELECT / WITH / EXPLAIN / SHOW / DESCRIBE.
// Cheap prefix check rather than a real parser — sufficient for
// gating; deeper validation lives in pkg/ml when the real DuckDB
// handle backs the query.
func isReadOnlySQL(sql string) bool {
	t := core.Trim(sql)
	if t == "" {
		return true
	}
	u := core.Upper(t)
	return core.HasPrefix(u, "SELECT") ||
		core.HasPrefix(u, "WITH") ||
		core.HasPrefix(u, "EXPLAIN") ||
		core.HasPrefix(u, "SHOW") ||
		core.HasPrefix(u, "DESCRIBE")
}

