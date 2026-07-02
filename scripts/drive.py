#!/usr/bin/env python3
"""Drive the kubeaid-mcp server over stdio like a real MCP client would.

Sends the initialize handshake, lists tools, and calls list_namespaces. Useful
for inspecting the raw JSON-RPC protocol without an AI client.

    go build -o kubeaid-mcp .
    python3 scripts/drive.py [--context <ctx>]
"""
import argparse, json, subprocess, sys

parser = argparse.ArgumentParser()
parser.add_argument("--context", default=None)
args = parser.parse_args()

cmd = ["./kubeaid-mcp"]
if args.context:
    cmd += ["--context", args.context]

proc = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                        stderr=subprocess.DEVNULL, text=True, bufsize=1)
_id = 0

def call(method, params=None):
    global _id
    _id += 1
    proc.stdin.write(json.dumps({"jsonrpc": "2.0", "id": _id,
                                 "method": method, "params": params or {}}) + "\n")
    proc.stdin.flush()
    return json.loads(proc.stdout.readline())

def notify(method):
    proc.stdin.write(json.dumps({"jsonrpc": "2.0", "method": method}) + "\n")
    proc.stdin.flush()

call("initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                    "clientInfo": {"name": "drive.py", "version": "0"}})
notify("notifications/initialized")

print("=== tools ===")
for t in call("tools/list")["result"]["tools"]:
    print(f"- {t['name']}: {t['description']}")

print("\n=== list_namespaces ===")
res = call("tools/call", {"name": "list_namespaces", "arguments": {}})["result"]
print(json.dumps(res.get("structuredContent", res), indent=2))

proc.stdin.close()
proc.wait(timeout=10)
