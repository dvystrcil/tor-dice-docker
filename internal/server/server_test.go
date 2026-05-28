package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dvystrcil/tor-dice-docker/internal/mcpclient"
)

// fakeDice is a hand-rolled mock of the DiceClient interface. Each
// field records the last call's args so tests can assert on them; the
// returned values + errors are set by the test up front.
type fakeDice struct {
	lastRoll  mcpclient.RollArgs
	lastCheck mcpclient.TORCheckArgs
	lastCmbt  mcpclient.TORCombatArgs

	rollResult    *mcpclient.RollResult
	rollErr       error
	checkResult   *mcpclient.TORCheckResult
	checkErr      error
	combatResult  *mcpclient.TORCombatResult
	combatErr     error
}

func (f *fakeDice) Roll(_ context.Context, args mcpclient.RollArgs) (*mcpclient.RollResult, error) {
	f.lastRoll = args
	return f.rollResult, f.rollErr
}
func (f *fakeDice) RollTORCheck(_ context.Context, args mcpclient.TORCheckArgs) (*mcpclient.TORCheckResult, error) {
	f.lastCheck = args
	return f.checkResult, f.checkErr
}
func (f *fakeDice) RollTORCombat(_ context.Context, args mcpclient.TORCombatArgs) (*mcpclient.TORCombatResult, error) {
	f.lastCmbt = args
	return f.combatResult, f.combatErr
}

func newTestServer(d DiceClient) *Server {
	// Minimal in-memory SPA fs: an index.html so static serves don't 404.
	staticFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>hi</html>")},
	}
	return New(d, staticFS)
}

func post(t *testing.T, srv *Server, path string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec.Result()
}

func decode[T any](t *testing.T, res *http.Response) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// ---- /healthz ---------------------------------------------------

func TestHealthz(t *testing.T) {
	srv := newTestServer(&fakeDice{})
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("got %d want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Errorf("healthz body: %q", rec.Body.String())
	}
}

// ---- Static SPA -------------------------------------------------

func TestStaticIndex(t *testing.T) {
	srv := newTestServer(&fakeDice{})
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("static / got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<html>") {
		t.Errorf("static body: %q", rec.Body.String())
	}
}

// ---- /api/roll --------------------------------------------------

func TestAPIRoll_OK(t *testing.T) {
	d := &fakeDice{rollResult: &mcpclient.RollResult{Spec: "2d6", Rolls: []int{3, 5}, Total: 8}}
	srv := newTestServer(d)
	res := post(t, srv, "/api/roll", map[string]any{"spec": "2d6"})
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	body := decode[mcpclient.RollResult](t, res)
	if body.Total != 8 {
		t.Errorf("Total=%d want 8", body.Total)
	}
	if d.lastRoll.Spec != "2d6" {
		t.Errorf("forwarded spec=%q want 2d6", d.lastRoll.Spec)
	}
}

func TestAPIRoll_MissingSpec_400(t *testing.T) {
	srv := newTestServer(&fakeDice{})
	res := post(t, srv, "/api/roll", map[string]any{"spec": ""})
	if res.StatusCode != 400 {
		t.Errorf("status=%d want 400", res.StatusCode)
	}
}

func TestAPIRoll_MCPErr_502(t *testing.T) {
	d := &fakeDice{rollErr: errors.New("upstream boom")}
	srv := newTestServer(d)
	res := post(t, srv, "/api/roll", map[string]any{"spec": "1d20"})
	if res.StatusCode != 502 {
		t.Errorf("status=%d want 502", res.StatusCode)
	}
}

func TestAPIRoll_BadJSON_400(t *testing.T) {
	srv := newTestServer(&fakeDice{})
	req := httptest.NewRequest("POST", "/api/roll",
		strings.NewReader(`{"spec":}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("status=%d want 400", rec.Code)
	}
}

// ---- /api/roll_tor_check ----------------------------------------

func TestAPIRollTORCheck_OK(t *testing.T) {
	d := &fakeDice{checkResult: &mcpclient.TORCheckResult{
		FeatDie: 7, SuccessDice: []int{4, 6}, Total: 17,
		Succeeds: true, Margin: 3, TargetNumber: 14,
		Formatted: `<span class="fdie7">7</span> = 17`,
	}}
	srv := newTestServer(d)
	res := post(t, srv, "/api/roll_tor_check", map[string]any{
		"skill_rating":  2,
		"target_number": 14,
	})
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	body := decode[mcpclient.TORCheckResult](t, res)
	if body.Total != 17 || !body.Succeeds {
		t.Errorf("body=%+v", body)
	}
	if d.lastCheck.SkillRating != 2 || d.lastCheck.TargetNumber != 14 {
		t.Errorf("forwarded args=%+v", d.lastCheck)
	}
}

func TestAPIRollTORCheck_DefaultsFormatToHTMLTOR(t *testing.T) {
	// When the request body omits `format`, the server should default
	// to "html_tor" before calling MCP. This is the contract that lets
	// the SPA render formatted HTML without each component opting in.
	d := &fakeDice{checkResult: &mcpclient.TORCheckResult{Total: 0}}
	srv := newTestServer(d)
	res := post(t, srv, "/api/roll_tor_check", map[string]any{
		"skill_rating":  1,
		"target_number": 14,
		// no "format" key
	})
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	if d.lastCheck.Format != "html_tor" {
		t.Errorf("format=%q want html_tor (default)", d.lastCheck.Format)
	}
}

func TestAPIRollTORCheck_NegativeSkill_400(t *testing.T) {
	srv := newTestServer(&fakeDice{})
	res := post(t, srv, "/api/roll_tor_check", map[string]any{
		"skill_rating":  -1,
		"target_number": 14,
	})
	if res.StatusCode != 400 {
		t.Errorf("status=%d want 400", res.StatusCode)
	}
}

func TestAPIRollTORCheck_PassesWearinessMiserable(t *testing.T) {
	d := &fakeDice{checkResult: &mcpclient.TORCheckResult{}}
	srv := newTestServer(d)
	_ = post(t, srv, "/api/roll_tor_check", map[string]any{
		"skill_rating":  1,
		"target_number": 14,
		"weariness":     true,
		"miserable":     true,
	})
	if !d.lastCheck.Weariness || !d.lastCheck.Miserable {
		t.Errorf("flags lost: %+v", d.lastCheck)
	}
}

// ---- /api/roll_tor_combat ---------------------------------------

func TestAPIRollTORCombat_OK(t *testing.T) {
	d := &fakeDice{combatResult: &mcpclient.TORCombatResult{
		FeatDie: 8, Total: 14, Hits: true, Margin: 0, DefenderTN: 14,
	}}
	srv := newTestServer(d)
	res := post(t, srv, "/api/roll_tor_combat", map[string]any{
		"attacker_skill": 3,
		"defender_tn":    14,
	})
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	if d.lastCmbt.AttackerSkill != 3 {
		t.Errorf("forwarded attacker_skill=%d", d.lastCmbt.AttackerSkill)
	}
}

func TestAPIRollTORCombat_NegativeSkill_400(t *testing.T) {
	srv := newTestServer(&fakeDice{})
	res := post(t, srv, "/api/roll_tor_combat", map[string]any{
		"attacker_skill": -1,
		"defender_tn":    14,
	})
	if res.StatusCode != 400 {
		t.Errorf("status=%d want 400", res.StatusCode)
	}
}

// ---- routing edge cases -----------------------------------------
//
// Go 1.22+ http.ServeMux's method-vs-path interaction makes these
// edge cases nuanced. We document the observed behavior here rather
// than enforce a specific status — the goal is "not 500 / not 200
// on a bogus request". A proper REST-perfectionist API would route
// these explicitly with a /api/ catch-all, but for a one-page roller
// the actual response codes don't matter much.

func TestBogusAPIPath_RejectedNot200(t *testing.T) {
	srv := newTestServer(&fakeDice{})
	req := httptest.NewRequest("POST", "/api/definitely-not-a-real-endpoint",
		strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code == 200 || rec.Code >= 500 {
		t.Errorf("status=%d want 4xx-ish", rec.Code)
	}
}

func TestGETOnPostEndpoint_RejectedNot200(t *testing.T) {
	srv := newTestServer(&fakeDice{})
	req := httptest.NewRequest("GET", "/api/roll", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code == 200 || rec.Code >= 500 {
		t.Errorf("status=%d want 4xx-ish", rec.Code)
	}
}
