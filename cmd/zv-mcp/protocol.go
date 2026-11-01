package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// pinNegotiatedVersion stores the negotiated protocol version in the session's
// initialize params. The SDK picks input requests from the version the client
// asked for, so a client that asked for a newer version than this server speaks
// would get input_required results it does not understand, and never see the
// approval question.
func pinNegotiatedVersion(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		res, err := next(ctx, method, req)
		if err != nil || method != "initialize" {
			return res, err
		}
		params, okParams := req.GetParams().(*mcp.InitializeParams)
		result, okResult := res.(*mcp.InitializeResult)
		if okParams && okResult && params != nil && result != nil {
			params.ProtocolVersion = result.ProtocolVersion
		}
		return res, err
	}
}
