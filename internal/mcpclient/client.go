// Package mcpclient wraps the modelcontextprotocol/go-sdk client surface
// for the specific tools tor-dice needs from rpg-dice-mcp. Keeps the
// MCP protocol details out of the HTTP server layer.
//
// A single ClientSession is maintained for the lifetime of the
// tor-dice process — the MCP Streamable HTTP transport supports
// long-lived sessions and reusing one across many tool calls is
// cheaper than initialize-per-call.
package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client is the tor-dice → rpg-dice-mcp wrapper. Safe for concurrent
// use; the underlying session is serialized internally.
type Client struct {
	endpoint string
	impl     *mcp.Implementation

	mu      sync.Mutex
	session *mcp.ClientSession
}

// New constructs a Client. endpoint is the rpg-dice-mcp /mcp URL
// (e.g. http://rpg-dice-mcp.rpg-dice-mcp.svc.cluster.local/mcp).
func New(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		impl: &mcp.Implementation{
			Name:    "tor-dice",
			Version: "0.1.0",
		},
	}
}

// connect lazy-initializes the MCP session if it isn't already open.
// Called under c.mu.
func (c *Client) connect(ctx context.Context) error {
	if c.session != nil {
		return nil
	}
	transport := &mcp.StreamableClientTransport{Endpoint: c.endpoint}
	client := mcp.NewClient(c.impl, nil)
	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	sess, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", c.endpoint, err)
	}
	c.session = sess
	return nil
}

// Close tears down the MCP session. Idempotent.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil
	}
	err := c.session.Close()
	c.session = nil
	return err
}

// callTool invokes a tool on rpg-dice-mcp and unmarshals the JSON
// response into `out`. Reconnects once on session failure.
func (c *Client) callTool(ctx context.Context, name string, args any, out any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for attempt := 0; attempt < 2; attempt++ {
		if err := c.connect(ctx); err != nil {
			return err
		}
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("marshal args: %w", err)
		}
		res, err := c.session.CallTool(ctx, &mcp.CallToolParams{
			Name:      name,
			Arguments: json.RawMessage(argsJSON),
		})
		if err != nil {
			// Likely the session died (server restart etc.). Drop
			// the cached session and retry once.
			c.session = nil
			if attempt == 0 {
				continue
			}
			return fmt.Errorf("call %s: %w", name, err)
		}
		if res.IsError {
			// Tool reported a domain error (e.g. negative skill_rating).
			// Surface the text content.
			return fmt.Errorf("%s: %s", name, contentText(res.Content))
		}
		raw := contentText(res.Content)
		if raw == "" {
			return fmt.Errorf("%s: empty response", name)
		}
		if err := json.Unmarshal([]byte(raw), out); err != nil {
			return fmt.Errorf("unmarshal %s response: %w; body=%q", name, err, raw)
		}
		return nil
	}
	return fmt.Errorf("call %s: exhausted retries", name)
}

func contentText(items []mcp.Content) string {
	for _, c := range items {
		if t, ok := c.(*mcp.TextContent); ok {
			return t.Text
		}
	}
	return ""
}

// ---- Public per-tool wrappers (typed inputs, typed outputs) -----

// RollArgs is the generic dice roll input.
type RollArgs struct {
	Spec string `json:"spec"`
}

// RollResult is the generic dice roll response.
type RollResult struct {
	Spec     string `json:"spec"`
	Rolls    []int  `json:"rolls"`
	Modifier int    `json:"modifier"`
	Total    int    `json:"total"`
}

// Roll calls rpg-dice-mcp.roll.
func (c *Client) Roll(ctx context.Context, args RollArgs) (*RollResult, error) {
	out := &RollResult{}
	if err := c.callTool(ctx, "roll", args, out); err != nil {
		return nil, err
	}
	return out, nil
}

// TORCheckArgs is the input for a TOR skill check.
type TORCheckArgs struct {
	SkillRating  int    `json:"skill_rating"`
	TargetNumber int    `json:"target_number"`
	Weariness    bool   `json:"weariness,omitempty"`
	Miserable    bool   `json:"miserable,omitempty"`
	Format       string `json:"format,omitempty"`
}

// TORCheckResult is the response shape from rpg-dice-mcp.roll_tor_check.
type TORCheckResult struct {
	FeatDie       int    `json:"feat_die"`
	GandalfRune   bool   `json:"gandalf_rune"`
	EyeOfSauron   bool   `json:"eye_of_sauron"`
	SuccessDice   []int  `json:"success_dice"`
	EffectiveDice []int  `json:"effective_dice"`
	Total         int    `json:"total"`
	Succeeds      bool   `json:"succeeds"`
	Margin        int    `json:"margin"`
	MiserableEye  bool   `json:"miserable_eye"`
	TargetNumber  int    `json:"target_number"`
	Formatted     string `json:"formatted,omitempty"`
	Format        string `json:"format,omitempty"`
}

// RollTORCheck calls rpg-dice-mcp.roll_tor_check.
func (c *Client) RollTORCheck(ctx context.Context, args TORCheckArgs) (*TORCheckResult, error) {
	out := &TORCheckResult{}
	if err := c.callTool(ctx, "roll_tor_check", args, out); err != nil {
		return nil, err
	}
	return out, nil
}

// TORCombatArgs is the input for a TOR combat attack roll.
type TORCombatArgs struct {
	AttackerSkill int    `json:"attacker_skill"`
	DefenderTN    int    `json:"defender_tn"`
	Weariness     bool   `json:"weariness,omitempty"`
	Miserable     bool   `json:"miserable,omitempty"`
	Format        string `json:"format,omitempty"`
}

// TORCombatResult is the response shape from rpg-dice-mcp.roll_tor_combat.
type TORCombatResult struct {
	FeatDie      int    `json:"feat_die"`
	GandalfRune  bool   `json:"gandalf_rune"`
	EyeOfSauron  bool   `json:"eye_of_sauron"`
	SuccessDice  []int  `json:"success_dice"`
	Total        int    `json:"total"`
	Hits         bool   `json:"hits"`
	Margin       int    `json:"margin"`
	MiserableEye bool   `json:"miserable_eye"`
	DefenderTN   int    `json:"defender_tn"`
	Formatted    string `json:"formatted,omitempty"`
	Format       string `json:"format,omitempty"`
}

// RollTORCombat calls rpg-dice-mcp.roll_tor_combat.
func (c *Client) RollTORCombat(ctx context.Context, args TORCombatArgs) (*TORCombatResult, error) {
	out := &TORCombatResult{}
	if err := c.callTool(ctx, "roll_tor_combat", args, out); err != nil {
		return nil, err
	}
	return out, nil
}
