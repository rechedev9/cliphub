package main

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPinNegotiatedVersionKeepsTheServerAnswer(t *testing.T) {
	params := &mcp.InitializeParams{ProtocolVersion: "2099-01-01"}
	next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return &mcp.InitializeResult{ProtocolVersion: "2025-11-25"}, nil
	}
	if _, err := pinNegotiatedVersion(next)(context.Background(), "initialize", &mcp.InitializeRequest{Params: params}); err != nil {
		t.Fatal(err)
	}
	if params.ProtocolVersion != "2025-11-25" {
		t.Fatalf("protocol version = %q, want the negotiated 2025-11-25", params.ProtocolVersion)
	}
}
