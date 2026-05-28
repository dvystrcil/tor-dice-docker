# tor-dice-docker

TOR-themed dice roller — Go server with embedded [Svelte](https://svelte.dev) SPA. Calls [`rpg-dice-mcp`](https://github.com/dvystrcil/rpg-dice-mcp-docker) over MCP for the actual roll resolution.

One binary, one container, one pod. The browser only ever talks to this server (same-origin, no CORS). Inside the Go process, an MCP client speaks Streamable HTTP to `rpg-dice-mcp` for `roll`, `roll_tor_check`, and `roll_tor_combat`.

Tracked: [dvystrcil/homelab#262](https://github.com/dvystrcil/homelab/issues/262).

## Architecture

```
┌─────────────────────────────────────┐
│  tor-dice (this repo)               │
│                                     │
│  GET  /            embed.FS         │ ← Svelte index.html + js/css
│  GET  /assets/*    embed.FS         │
│  POST /api/roll              ──┐    │ ← REST endpoints translate to
│  POST /api/roll_tor_check    ──┼─→  │   MCP CallTool() over Streamable
│  POST /api/roll_tor_combat   ──┘    │   HTTP (one long-lived session)
└─────────────────────────────────────┘
              │
              ▼ MCP / HTTP
┌─────────────────────────────────────┐
│  rpg-dice-mcp (separate service)    │
│  POST /mcp                          │
└─────────────────────────────────────┘
```

REST endpoints default `format=html_tor` so the response includes a ready-to-render `formatted` field with `<span class="fdie7">7</span>`-style dice spans. The Svelte SPA uses `{@html result.formatted}` for the rendered dice faces, with `.fdie*`/`.sdie*` CSS classes loading background images from un-tor.github.io.

## Layout

```
.
├── main.go                       Go entrypoint + //go:embed all:web/dist
├── internal/
│   ├── server/server.go          HTTP server (REST + embed.FS)
│   ├── server/server_test.go
│   ├── mcpclient/client.go       MCP client wrapper (Streamable HTTP)
│   └── mcpclient/client_test.go
├── web/                          Svelte SPA source
│   ├── package.json
│   ├── vite.config.js
│   ├── index.html
│   └── src/
│       ├── App.svelte
│       ├── main.js
│       ├── app.css
│       ├── lib/
│       │   ├── api.js            fetch() wrappers
│       │   └── history.js        localStorage
│       └── components/
│           ├── SkillCheck.svelte
│           ├── CombatRoll.svelte
│           ├── GenericRoll.svelte
│           └── RollHistory.svelte
├── Dockerfile                    Multi-stage: vite build → go build → scratch
└── .github/workflows/
    ├── build.yaml                Tests + :dev image push on push to main
    └── docker-release.yaml       Promote :dev to :v* on GH release
```

## Local development

The Svelte SPA can run against the Go server via Vite's proxy (defined in `vite.config.js`). One terminal runs Go, the other runs Vite:

```bash
# Terminal 1 — Go server (listens on :8080)
TOR_DICE_MCP_URL=http://localhost:8081/mcp go run .

# Terminal 2 — Svelte dev (listens on :5173, proxies /api → :8080)
cd web && npm install && npm run dev
```

For an end-to-end smoke test without an in-cluster rpg-dice-mcp, run a local one:

```bash
# Terminal 3 — rpg-dice-mcp
cd ../rpg-dice-mcp-docker
go run . --http=:8081
```

## Building the production image

```bash
docker build -t tor-dice:dev .
docker run --rm -p 8080:8080 \
  -e TOR_DICE_MCP_URL=http://rpg-dice-mcp.example.com/mcp \
  tor-dice:dev
```

## Configuration

| Env var | Default | Purpose |
|---|---|---|
| `TOR_DICE_LISTEN` | `:8080` | Bind address |
| `TOR_DICE_MCP_URL` | `http://rpg-dice-mcp.rpg-dice-mcp.svc.cluster.local/mcp` | rpg-dice-mcp endpoint |

## Tests

```bash
go test -cover ./...
```

- `internal/server`: HTTP handlers tested against a fake `DiceClient` interface (89% coverage)
- `internal/mcpclient`: round-trip tests against an in-memory MCP server using the SDK's own `NewStreamableHTTPHandler` (74% coverage)

## License

[MIT](LICENSE). Dice-face PNGs and Aniron / InitialRing fonts © their respective creators, sourced from [un-tor.github.io](https://un-tor.github.io).
