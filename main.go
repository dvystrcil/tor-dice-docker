// tor-dice — TOR-themed dice roller web app.
//
// Serves an embedded Svelte SPA and proxies dice-roll requests via
// MCP to rpg-dice-mcp. Single binary, single container, single pod.
//
// Configuration:
//
//	TOR_DICE_LISTEN    — bind address (default :8080)
//	TOR_DICE_MCP_URL   — rpg-dice-mcp endpoint
//	                     (default http://rpg-dice-mcp.rpg-dice-mcp.svc.cluster.local/mcp)
//
// Tracked: dvystrcil/homelab#262.
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dvystrcil/tor-dice-docker/internal/mcpclient"
	"github.com/dvystrcil/tor-dice-docker/internal/server"
)

// Embedded Svelte SPA build output. The Dockerfile multi-stage build
// runs `npm run build` to populate web/dist before `go build` ingests
// it via this directive.
//
//go:embed all:web/dist
var spaFS embed.FS

const (
	defaultListen = ":8080"
	defaultMCPURL = "http://rpg-dice-mcp.rpg-dice-mcp.svc.cluster.local/mcp"
)

func main() {
	listen := envOr("TOR_DICE_LISTEN", defaultListen)
	mcpURL := envOr("TOR_DICE_MCP_URL", defaultMCPURL)

	// Strip the "web/dist" prefix so URLs map to the SPA root.
	staticFS, err := fs.Sub(spaFS, "web/dist")
	if err != nil {
		log.Fatalf("embedded SPA fs: %v", err)
	}

	mcp := mcpclient.New(mcpURL)
	defer mcp.Close()

	srv := server.New(mcp, staticFS)
	httpServer := &http.Server{
		Addr:              listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Graceful shutdown — give in-flight tool calls a chance to finish.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("tor-dice listening on %s; mcp=%s", listen, mcpURL)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	fmt.Println("bye")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
