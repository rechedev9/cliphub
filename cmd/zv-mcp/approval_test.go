package main

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestApprovalCodeBindsToolJobAndRequest(t *testing.T) {
	a := newApprover()
	spec := approvalSpec{Tool: "generate_short", JobID: "job-1", Request: map[string]int{"n": 1}}
	ref, err := a.codeFor(spec)
	if err != nil {
		t.Fatal(err)
	}
	changed := map[string]approvalSpec{
		"tool":    {Tool: "render_full_demo", JobID: "job-1", Request: spec.Request},
		"job":     {Tool: spec.Tool, JobID: "job-2", Request: spec.Request},
		"request": {Tool: spec.Tool, JobID: "job-1", Request: map[string]int{"n": 2}},
	}
	for name, c := range changed {
		if got, _ := a.codeFor(c); got == ref {
			t.Errorf("changing the %s keeps the same code", name)
		}
	}
	if other, _ := newApprover().codeFor(spec); other == ref {
		t.Error("two processes derive the same code: the key is not random")
	}
}

func TestApprovalCodeIsSingleUseAndExpires(t *testing.T) {
	a := newApprover()
	now := time.Now()
	a.now = func() time.Time { return now }
	spec := approvalSpec{Tool: "generate_short", JobID: "job-1", Request: "plays"}
	code, _ := a.codeFor(spec)

	spec.Given = code
	if _, err := a.approveByCode(spec); !errors.Is(err, errApprovalInvalid) {
		t.Errorf("a code that was never issued was accepted: %v", err)
	}

	spec.Given = ""
	offered, err := a.approveByCode(spec)
	if err != nil || offered.Approved || offered.Code != code {
		t.Fatalf("first call: %+v, %v", offered, err)
	}
	spec.Given = code
	if res, err := a.approveByCode(spec); err != nil || !res.Approved {
		t.Fatalf("issued code refused: %+v, %v", res, err)
	}
	if _, err := a.approveByCode(spec); !errors.Is(err, errApprovalInvalid) {
		t.Errorf("a used code was accepted again: %v", err)
	}

	spec.Given = ""
	if _, err := a.approveByCode(spec); err != nil {
		t.Fatal(err)
	}
	now = now.Add(approvalTTL + time.Second)
	spec.Given = code
	if _, err := a.approveByCode(spec); !errors.Is(err, errApprovalInvalid) {
		t.Errorf("an expired code was accepted: %v", err)
	}
}

type askIn struct {
	Given string `json:"given,omitempty"`
}

type askOut struct {
	Approved bool   `json:"approved"`
	Code     string `json:"code,omitempty"`
}

func connectWith(t *testing.T, server *mcp.Server, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	return connectVersion(t, server, opts, "")
}

// connectVersion connects a client that negotiates the given protocol version
// (empty means the newest).
func connectVersion(t *testing.T, server *mcp.Server, opts *mcp.ClientOptions, version string) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, opts).Connect(ctx, clientT, &mcp.ClientSessionOptions{ProtocolVersion: version})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func newAskServer(a *approver) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "ask"}, func(ctx context.Context, req *mcp.CallToolRequest, in askIn) (*mcp.CallToolResult, askOut, error) {
		res, err := a.approve(req, approvalSpec{Tool: "ask", JobID: "job-1", Request: "plays", Summary: "Capture.", Given: in.Given})
		if res.Ask != nil {
			return res.Ask, askOut{}, nil
		}
		return nil, askOut{Approved: res.Approved, Code: res.Code}, err
	})
	return server
}

func callAsk(t *testing.T, cs *mcp.ClientSession, given string) (askOut, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "ask", Arguments: map[string]any{"given": given}})
	if err != nil || res.IsError {
		return askOut{}, true
	}
	fields, _ := res.StructuredContent.(map[string]any)
	approved, _ := fields["approved"].(bool)
	code, _ := fields["code"].(string)
	return askOut{Approved: approved, Code: code}, false
}

func TestApproveWithoutElicitationUsesCodes(t *testing.T) {
	cs := connectWith(t, newAskServer(newApprover()), nil)
	offered, failed := callAsk(t, cs, "")
	if failed || offered.Approved || offered.Code == "" {
		t.Fatalf("first call: %+v failed=%v", offered, failed)
	}
	if _, failed := callAsk(t, cs, "0123456789ab"); !failed {
		t.Error("a guessed code was accepted")
	}
	if got, failed := callAsk(t, cs, offered.Code); failed || !got.Approved {
		t.Errorf("issued code: %+v failed=%v", got, failed)
	}
	if _, failed := callAsk(t, cs, offered.Code); !failed {
		t.Error("a replayed code was accepted")
	}
}

func TestApproveWithElicitationNeverUsesCodes(t *testing.T) {
	// "" negotiates the newest protocol (the client retries on its own);
	// 2025-06-18 has the SDK ask the question for the server.
	for _, version := range []string{"", "2025-06-18"} {
		asked := 0
		respond := func(action string, err error) *mcp.ClientOptions {
			return &mcp.ClientOptions{ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
				asked++
				if err != nil {
					return nil, err
				}
				return &mcp.ElicitResult{Action: action}, nil
			}}
		}

		a := newApprover()
		server := newAskServer(a)
		// A valid code, as a code-only client would hold it.
		code, _ := a.codeFor(approvalSpec{Tool: "ask", JobID: "job-1", Request: "plays"})
		a.issue(code)

		accepting := connectVersion(t, server, respond("accept", nil), version)
		if got, failed := callAsk(t, accepting, ""); failed || !got.Approved || got.Code != "" {
			t.Errorf("%q accept: %+v failed=%v", version, got, failed)
		}
		if got, failed := callAsk(t, accepting, "bogus"); failed || !got.Approved || asked != 2 {
			t.Errorf("%q a code must not skip the question: %+v failed=%v asked=%d", version, got, failed, asked)
		}
		if _, failed := callAsk(t, connectVersion(t, server, respond("decline", nil), version), code); !failed {
			t.Errorf("%q a valid code overrode a declined question", version)
		}
		if got, failed := callAsk(t, connectVersion(t, server, respond("", errors.New("no ui")), version), ""); !failed || got.Code != "" {
			t.Errorf("%q a failing elicitation must be an error without a code: %+v failed=%v", version, got, failed)
		}
	}
}

func TestRecoverPanicsKeepsTheServerAlive(t *testing.T) {
	previous := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(previous) })

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	server.AddReceivingMiddleware(recoverPanics)
	mcp.AddTool(server, &mcp.Tool{Name: "boom"}, func(context.Context, *mcp.CallToolRequest, empty) (*mcp.CallToolResult, empty, error) {
		panic("handler bug")
	})
	mcp.AddTool(server, &mcp.Tool{Name: "fine"}, func(context.Context, *mcp.CallToolRequest, empty) (*mcp.CallToolResult, empty, error) {
		return nil, empty{}, nil
	})
	cs := connectWith(t, server, nil)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "boom"})
	if err != nil || !res.IsError {
		t.Fatalf("panic should be a tool error: %v %+v", err, res)
	}
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "fine"})
	if err != nil || res.IsError {
		t.Fatalf("server did not survive the panic: %v %+v", err, res)
	}
}
