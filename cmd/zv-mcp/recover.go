package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// recoverPanics keeps the stdio server alive when a handler panics: the client
// would otherwise only see "Connection closed". The panic goes to stderr and
// the model gets a tool error.
func recoverPanics(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (result mcp.Result, err error) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			log.Printf("zv-mcp: panic in %s: %v\n%s", method, r, debug.Stack())
			failure := errors.New("internal error in zv-mcp; the request was not completed")
			if method == "tools/call" {
				res := &mcp.CallToolResult{}
				res.SetError(failure)
				result, err = res, nil
				return
			}
			result, err = nil, fmt.Errorf("%s: %w", method, failure)
		}()
		return next(ctx, method, req)
	}
}
