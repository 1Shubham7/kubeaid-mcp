// Command kubeaid-mcp is an MCP (Model Context Protocol) server that exposes a
// Kubernetes cluster to MCP-compatible AI clients. The client launches this
// binary as a subprocess and communicates over stdin/stdout using JSON-RPC 2.0.
package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1shubham7/kubeaid-mcp/k8s"
	"github.com/1shubham7/kubeaid-mcp/prompts"
	"github.com/1shubham7/kubeaid-mcp/tools"
)

// Version is stamped at build time via -ldflags "-X main.Version=...". When the
// binary is produced by `go install ...@vX.Y.Z`, version() recovers the module
// version from the build info instead.
var Version = "dev"

func version() string {
	if Version != "dev" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return Version
}

// splitComma splits a comma-separated flag value into a trimmed, non-empty list.
func splitComma(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func main() {
	// Logging must go to stderr: stdout carries the JSON-RPC protocol, and any
	// stray output there corrupts message framing.
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	var kubeconfig, kubeContext, protectedContexts string
	var requestTimeout time.Duration
	var allowWrites, allowExec bool
	flag.StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig file (default: KUBECONFIG env or ~/.kube/config)")
	flag.StringVar(&kubeContext, "context", "", "default kubeconfig context; individual tool calls may override it (default: current-context)")
	flag.DurationVar(&requestTimeout, "request-timeout", 30*time.Second, "per-request timeout for Kubernetes API calls")
	flag.BoolVar(&allowWrites, "allow-writes", false, "enable mutating tools (apply, patch, delete, scale, rollout restart)")
	flag.BoolVar(&allowExec, "allow-exec", false, "enable the exec tool (run commands inside containers)")
	flag.StringVar(&protectedContexts, "protected-context", "", "comma-separated contexts that may never be written to or exec'd into")
	flag.Parse()

	kc, err := k8s.NewClientManager(k8s.Config{
		KubeconfigPath:    kubeconfig,
		DefaultContext:    kubeContext,
		RequestTimeout:    requestTimeout,
		AllowWrites:       allowWrites,
		AllowExec:         allowExec,
		ProtectedContexts: splitComma(protectedContexts),
	})
	if err != nil {
		logger.Error("failed to load kubeconfig", "err", err)
		os.Exit(1)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "kubeaid-mcp",
		Version: version(),
	}, nil)

	tools.RegisterAll(server, kc)
	prompts.RegisterAll(server)

	// Cancel the run context on Ctrl-C / SIGTERM so the server shuts down
	// cleanly instead of being force-killed mid-request.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("starting kubeaid-mcp server",
		"version", version(), "transport", "stdio",
		"allowWrites", allowWrites, "allowExec", allowExec)

	// Run reads requests from stdin and writes responses to stdout until the
	// client closes the connection or the context is cancelled. A cancelled
	// context (signal) or EOF (client disconnect) are both normal shutdowns.
	err = server.Run(ctx, &mcp.StdioTransport{})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	logger.Info("server shut down cleanly")
}
