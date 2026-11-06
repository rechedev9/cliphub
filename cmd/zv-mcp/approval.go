package main

import (
	"cmp"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const approvalTTL = 10 * time.Minute

var (
	errApprovalInvalid  = errors.New("the approval code is not valid for this request; ask the user again, then call without approve to get a new code")
	errApprovalDeclined = errors.New("the user declined; nothing was started")
)

// sessionApprovals is shared by every tool that starts a capture or a long render.
var sessionApprovals = newApprover()

// approver issues approval codes for clients without elicitation. A code is an
// HMAC under a per-process key over the tool, the job and the exact request, so
// the model cannot compute one; it works once and lapses after approvalTTL.
type approver struct {
	key    []byte
	now    func() time.Time
	mu     sync.Mutex
	issued map[string]time.Time
}

func newApprover() *approver {
	key := make([]byte, 32)
	// crypto/rand.Read never fails since Go 1.24.
	_, _ = rand.Read(key)
	return &approver{key: key, now: time.Now, issued: map[string]time.Time{}}
}

// approvalSpec names what the user is asked to approve.
type approvalSpec struct {
	Tool    string
	JobID   string
	Request any
	Summary string
	// Given is the approve argument of the call, if any.
	Given string
	// Question follows Summary in the elicitation; empty asks to start now.
	Question string
}

// approval is the outcome of a call: Approved, a Code to offer for a retry, or
// Ask, an elicitation the tool must return so the client can put it to the user.
type approval struct {
	Approved bool
	Code     string
	Ask      *mcp.CallToolResult
}

// elicitKey names the one question a tool asks.
const elicitKey = "approve"

// approve asks through elicitation whenever the client advertises it, and then
// ignores any code. Only a client without elicitation can approve with a code.
//
// Elicitation is an input request returned from the tool (SEP-2322): the SDK
// puts it to clients on older protocol versions itself and calls the tool again
// with the answer, and a failed question fails the call. The request state binds
// the answer to this exact request.
func (a *approver) approve(req *mcp.CallToolRequest, spec approvalSpec) (approval, error) {
	if !supportsElicitation(req) {
		return a.approveByCode(spec)
	}
	state, err := a.codeFor(spec)
	if err != nil {
		return approval{}, err
	}
	answer, answered := answerTo(req, state)
	if !answered {
		ask := &mcp.ElicitParams{
			Message:         spec.Summary + " " + cmp.Or(spec.Question, "¿Empezar ahora?"),
			RequestedSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		}
		return approval{Ask: &mcp.CallToolResult{InputRequests: mcp.InputRequestMap{elicitKey: ask}, RequestState: state}}, nil
	}
	if answer.Action != "accept" {
		return approval{}, errApprovalDeclined
	}
	return approval{Approved: true}, nil
}

// answerTo returns the user's answer when the call is the retry of this request's question.
func answerTo(req *mcp.CallToolRequest, state string) (*mcp.ElicitResult, bool) {
	if req.Params == nil || !hmac.Equal([]byte(req.Params.RequestState), []byte(state)) {
		return nil, false
	}
	answer, ok := req.Params.InputResponses[elicitKey].(*mcp.ElicitResult)
	return answer, ok && answer != nil
}

func supportsElicitation(req *mcp.CallToolRequest) bool {
	params := req.Session.InitializeParams()
	return params != nil && params.Capabilities != nil && params.Capabilities.Elicitation != nil
}

func (a *approver) approveByCode(spec approvalSpec) (approval, error) {
	code, err := a.codeFor(spec)
	if err != nil {
		return approval{}, err
	}
	if spec.Given == "" {
		a.issue(code)
		return approval{Code: code}, nil
	}
	if !a.consume(spec.Given, code) {
		return approval{}, errApprovalInvalid
	}
	return approval{Approved: true}, nil
}

func (a *approver) codeFor(spec approvalSpec) (string, error) {
	body, err := json.Marshal(spec.Request)
	if err != nil {
		return "", fmt.Errorf("encode approval request: %w", err)
	}
	mac := hmac.New(sha256.New, a.key)
	for _, part := range [][]byte{[]byte(spec.Tool), []byte(spec.JobID), body} {
		mac.Write(part)
		mac.Write([]byte{0})
	}
	return hex.EncodeToString(mac.Sum(nil))[:24], nil
}

func (a *approver) issue(code string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	for c, expires := range a.issued {
		if !now.Before(expires) {
			delete(a.issued, c)
		}
	}
	a.issued[code] = now.Add(approvalTTL)
}

// consume accepts given once, and only when it is the code issued for this request.
func (a *approver) consume(given, expected string) bool {
	if !hmac.Equal([]byte(given), []byte(expected)) {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	expires, ok := a.issued[expected]
	delete(a.issued, expected)
	return ok && a.now().Before(expires)
}
