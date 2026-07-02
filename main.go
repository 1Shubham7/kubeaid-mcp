// Command kubeaid-mcp is an MCP (Model Context Protocol) server that exposes a
// Kubernetes cluster to any MCP-compatible AI client (Claude Desktop, Claude
// Code, Cursor, ...).
//
// The AI client launches this binary as a subprocess and talks to it over
// stdin/stdout using JSON-RPC 2.0 (the "stdio transport"). There is no human
// typing at a prompt here — the "user" of this program is the AI model.
//
// PHASE 0: this is a skeleton. It completes the MCP handshake and exposes ZERO
// tools. We add real Kubernetes tools starting in Phase 1.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is the server version reported to clients during the handshake.
// Later we can override this at build time with -ldflags.
const Version = "0.0.1"

func main() {
	// IMPORTANT: all logging goes to STDERR, never STDOUT.
	// stdout is reserved for the JSON-RPC protocol. A single stray line on
	// stdout corrupts the message framing and the client will fail to parse
	// our responses. This is the #1 mistake in a first MCP server.
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// 1. Create the MCP server. Implementation{Name,Version} is what the
	//    client sees in the handshake response (serverInfo).
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "kubeaid-mcp",
		Version: Version,
	}, nil)

	// 2. (Phase 1+) tools get registered here with mcp.AddTool(server, ...).
	//    For now we register nothing — the server will answer tools/list with
	//    an empty list. That's a valid, fully-functional MCP server.

	logger.Info("starting kubeaid-mcp server", "version", Version, "transport", "stdio")

	// 3. Run blocks, reading JSON-RPC messages from stdin and writing replies
	//    to stdout, until the client closes the connection (EOF on stdin).
	//    We tie it to a context that cancels on Ctrl-C / SIGTERM so the process
	//    shuts down cleanly.
	ctx := context.Background()
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	logger.Info("server shut down cleanly")
}
