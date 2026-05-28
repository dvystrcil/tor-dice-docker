// Package server wires the embedded Svelte SPA + JSON REST endpoints
// that translate to MCP tool calls on rpg-dice-mcp.
//
// The browser only ever talks to this server — same-origin, so no CORS
// dance. Static files come from the embedded FS (baked into the
// binary at compile time via the //go:embed directive in main.go).
// Three POST endpoints translate plain-JSON requests into MCP
// CallTool() invocations via internal/mcpclient.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/dvystrcil/tor-dice-docker/internal/mcpclient"
)

// DiceClient is the slice of the MCP client surface the server needs.
// Defined as an interface here (rather than reaching into mcpclient
// concretely) so tests can pass a fake.
type DiceClient interface {
	Roll(ctx context.Context, args mcpclient.RollArgs) (*mcpclient.RollResult, error)
	RollTORCheck(ctx context.Context, args mcpclient.TORCheckArgs) (*mcpclient.TORCheckResult, error)
	RollTORCombat(ctx context.Context, args mcpclient.TORCombatArgs) (*mcpclient.TORCombatResult, error)
}

// Server is the tor-dice HTTP server.
type Server struct {
	mcp    DiceClient
	static fs.FS
}

// New constructs a Server. `static` should be the SPA filesystem
// (typically an embed.FS rooted at the build output directory).
func New(mcp DiceClient, static fs.FS) *Server {
	return &Server{mcp: mcp, static: static}
}

// Handler returns the configured http.Handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health probe — used by the readiness/liveness probes in K8s.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// REST endpoints. POST-only with JSON body in / JSON body out.
	mux.HandleFunc("POST /api/roll", s.handleRoll)
	mux.HandleFunc("POST /api/roll_tor_check", s.handleRollTORCheck)
	mux.HandleFunc("POST /api/roll_tor_combat", s.handleRollTORCombat)

	// Static SPA. Anything not matched above falls through to the
	// embedded filesystem.
	mux.Handle("GET /", http.FileServer(http.FS(s.static)))

	return mux
}

// ---- handlers ---------------------------------------------------

func (s *Server) handleRoll(w http.ResponseWriter, r *http.Request) {
	var args mcpclient.RollArgs
	if !readJSON(w, r, &args) {
		return
	}
	if strings.TrimSpace(args.Spec) == "" {
		writeError(w, http.StatusBadRequest, "spec is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	res, err := s.mcp.Roll(ctx, args)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("mcp call failed: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleRollTORCheck(w http.ResponseWriter, r *http.Request) {
	var args mcpclient.TORCheckArgs
	if !readJSON(w, r, &args) {
		return
	}
	if args.SkillRating < 0 {
		writeError(w, http.StatusBadRequest, "skill_rating must be >= 0")
		return
	}
	// Default to html_tor format — the SPA renders the `formatted`
	// field directly into the chat UI.
	if args.Format == "" {
		args.Format = "html_tor"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	res, err := s.mcp.RollTORCheck(ctx, args)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("mcp call failed: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleRollTORCombat(w http.ResponseWriter, r *http.Request) {
	var args mcpclient.TORCombatArgs
	if !readJSON(w, r, &args) {
		return
	}
	if args.AttackerSkill < 0 {
		writeError(w, http.StatusBadRequest, "attacker_skill must be >= 0")
		return
	}
	if args.Format == "" {
		args.Format = "html_tor"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	res, err := s.mcp.RollTORCombat(ctx, args)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("mcp call failed: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---- helpers ----------------------------------------------------

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<14) // 16 KiB plenty
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		// Distinguish empty-body (acceptable for some endpoints; we
		// reject here because all our endpoints require fields) from
		// malformed JSON.
		if errors.Is(err, http.ErrBodyReadAfterClose) {
			writeError(w, http.StatusBadRequest, "request body required")
		} else {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
		}
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
