package mcpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// emptySchema is a permissive object schema for test tools that
// accept arbitrary args. The SDK requires a non-nil InputSchema on
// AddTool; using `{"type":"object"}` makes the test server accept
// whatever the client sends.
func emptySchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object"}
}

// inMemoryServer spins up a real MCP server with the three tools
// tor-dice cares about, served over Streamable HTTP via the SDK's own
// handler. The client under test points at this server's URL — so
// the test exercises real Streamable-HTTP transport, real session
// init, real CallTool dispatch. We're testing the CLIENT here; the
// tool handlers just echo canned JSON.
func inMemoryServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-dice", Version: "v0.1.0"}, nil)

	server.AddTool(
		&mcp.Tool{Name: "roll", Description: "generic", InputSchema: emptySchema()},
		func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args struct {
				Spec string `json:"spec"`
			}
			_ = json.Unmarshal(req.Params.Arguments, &args)
			out, _ := json.Marshal(map[string]any{
				"spec":     args.Spec,
				"rolls":    []int{4, 6},
				"modifier": 0,
				"total":    10,
			})
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(out)}},
			}, nil
		},
	)

	server.AddTool(
		&mcp.Tool{Name: "roll_tor_check", Description: "tor check", InputSchema: emptySchema()},
		func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args struct {
				SkillRating  int    `json:"skill_rating"`
				TargetNumber int    `json:"target_number"`
				Format       string `json:"format"`
			}
			_ = json.Unmarshal(req.Params.Arguments, &args)
			body := map[string]any{
				"feat_die":      7,
				"success_dice":  []int{4, 6},
				"total":         17,
				"succeeds":      true,
				"margin":        3,
				"target_number": args.TargetNumber,
				"miserable_eye": false,
				"gandalf_rune":  false,
				"eye_of_sauron": false,
			}
			if args.Format != "" && args.Format != "none" {
				body["formatted"] = `<span class="fdie7">7</span>`
				body["format"] = args.Format
			}
			out, _ := json.Marshal(body)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(out)}},
			}, nil
		},
	)

	server.AddTool(
		&mcp.Tool{Name: "roll_tor_combat", Description: "tor combat", InputSchema: emptySchema()},
		func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			out, _ := json.Marshal(map[string]any{
				"feat_die":      8,
				"success_dice":  []int{3, 4},
				"total":         15,
				"hits":          true,
				"margin":        1,
				"defender_tn":   14,
				"miserable_eye": false,
				"gandalf_rune":  false,
				"eye_of_sauron": false,
			})
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(out)}},
			}, nil
		},
	)

	handler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	)
	httpSrv := httptest.NewServer(handler)
	t.Cleanup(httpSrv.Close)
	return httpSrv
}

// ---- Roll (generic) -----------------------------------------------

func TestRoll_RoundTrip(t *testing.T) {
	srv := inMemoryServer(t)
	c := New(srv.URL)
	defer c.Close()

	res, err := c.Roll(context.Background(), RollArgs{Spec: "2d6"})
	if err != nil {
		t.Fatalf("Roll: %v", err)
	}
	if res.Spec != "2d6" {
		t.Errorf("Spec=%q", res.Spec)
	}
	if res.Total != 10 || len(res.Rolls) != 2 {
		t.Errorf("Result=%+v", res)
	}
}

// ---- TOR check ----------------------------------------------------

func TestRollTORCheck_RoundTrip(t *testing.T) {
	srv := inMemoryServer(t)
	c := New(srv.URL)
	defer c.Close()

	res, err := c.RollTORCheck(context.Background(), TORCheckArgs{
		SkillRating:  2,
		TargetNumber: 14,
	})
	if err != nil {
		t.Fatalf("RollTORCheck: %v", err)
	}
	if res.FeatDie != 7 || res.Total != 17 || !res.Succeeds {
		t.Errorf("Result=%+v", res)
	}
	if res.TargetNumber != 14 {
		t.Errorf("TargetNumber lost in round-trip: %d", res.TargetNumber)
	}
}

func TestRollTORCheck_FormatHTMLTOR(t *testing.T) {
	srv := inMemoryServer(t)
	c := New(srv.URL)
	defer c.Close()

	res, err := c.RollTORCheck(context.Background(), TORCheckArgs{
		SkillRating:  2,
		TargetNumber: 14,
		Format:       "html_tor",
	})
	if err != nil {
		t.Fatalf("RollTORCheck: %v", err)
	}
	if res.Formatted == "" {
		t.Errorf("expected Formatted field when format=html_tor")
	}
	if res.Format != "html_tor" {
		t.Errorf("Format echo=%q want html_tor", res.Format)
	}
}

// ---- TOR combat ---------------------------------------------------

func TestRollTORCombat_RoundTrip(t *testing.T) {
	srv := inMemoryServer(t)
	c := New(srv.URL)
	defer c.Close()

	res, err := c.RollTORCombat(context.Background(), TORCombatArgs{
		AttackerSkill: 2,
		DefenderTN:    14,
	})
	if err != nil {
		t.Fatalf("RollTORCombat: %v", err)
	}
	if !res.Hits || res.Total != 15 {
		t.Errorf("Result=%+v", res)
	}
}

// ---- session reuse ------------------------------------------------

func TestSessionReuseAcrossCalls(t *testing.T) {
	// Two back-to-back calls should share the same MCP session — i.e.
	// the client.connect() lazy-init runs only once.
	srv := inMemoryServer(t)
	c := New(srv.URL)
	defer c.Close()

	_, err := c.Roll(context.Background(), RollArgs{Spec: "1d6"})
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	firstSession := c.session
	if firstSession == nil {
		t.Fatal("expected session after first call")
	}
	_, err = c.Roll(context.Background(), RollArgs{Spec: "1d8"})
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if c.session != firstSession {
		t.Error("session was re-created between calls (expected reuse)")
	}
}

// ---- close --------------------------------------------------------

func TestCloseIsIdempotent(t *testing.T) {
	c := New("http://example.invalid/mcp")
	if err := c.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}
