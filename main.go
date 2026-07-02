// Command kubeaid-mcp is an MCP (Model Context Protocol) server that exposes a
// Kubernetes cluster to MCP-compatible AI clients. The client launches this
// binary as a subprocess and communicates over stdin/stdout using JSON-RPC 2.0.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1shubham7/kubeaid-mcp/k8s"
	"github.com/1shubham7/kubeaid-mcp/tools"
)

const Version = "0.0.1"

func main() {
	// Logging must go to stderr: stdout carries the JSON-RPC protocol, and any
	// stray output there corrupts message framing.
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	var kubeconfig, kubeContext string
	flag.StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig file (default: KUBECONFIG env or ~/.kube/config)")
	flag.StringVar(&kubeContext, "context", "", "kubeconfig context to use (default: current-context)")
	flag.Parse()

	kc, err := k8s.NewClient(kubeconfig, kubeContext)
	if err != nil {
		logger.Error("failed to connect to Kubernetes", "err", err)
		os.Exit(1)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "kubeaid-mcp",
		Version: Version,
	}, nil)

	tools.RegisterAll(server, kc)

	logger.Info("starting kubeaid-mcp server", "version", Version, "transport", "stdio")

	// Run reads requests from stdin and writes responses to stdout until the
	// client closes the connection.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	logger.Info("server shut down cleanly")
}
