package steamresolve

import (
	"context"
	"errors"
	"testing"

	"github.com/rechedev9/cliphub/internal/steamgc"
)

// Known-good vector: CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK.
const (
	goodCode      = "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK"
	goodMatchID   = uint64(3230642215713767580)
	goodOutcomeID = uint64(3230647599455273103)
	goodTokenID   = uint32(55788)
)

type fakeTransport struct {
	matches []steamgc.Match
	err     error
	got     *steamgc.Request
}

func (f *fakeTransport) RequestMatch(_ context.Context, req steamgc.Request) ([]steamgc.Match, error) {
	f.got = &req
	return f.matches, f.err
}

func TestResolve(t *testing.T) {
	transportErr := errors.New("gc boom")
	decodedOnly := Result{MatchID: goodMatchID, OutcomeID: goodOutcomeID, TokenID: goodTokenID}
	tests := []struct {
		name        string
		code        string
		transport   *fakeTransport // nil means no transport
		want        Result
		wantErr     error // sentinel checked with errors.Is; nil means no error
		wantRequest bool  // the fake must have received the decoded identifiers
	}{
		{
			name:    "invalid code",
			code:    "CSGO-nope",
			wantErr: ErrInvalidCode,
		},
		{
			name: "nil transport decodes without error",
			code: goodCode,
			want: decodedOnly,
		},
		{
			name:      "matching match with demo URL resolves",
			code:      goodCode,
			transport: &fakeTransport{matches: []steamgc.Match{{MatchID: goodMatchID, DemoURL: "http://replay.example/1.dem.bz2"}}},
			want: Result{
				MatchID: goodMatchID, OutcomeID: goodOutcomeID, TokenID: goodTokenID,
				DemoURL: "http://replay.example/1.dem.bz2", Resolved: true,
			},
			wantRequest: true,
		},
		{
			name:        "no matches means expired demo, not an error",
			code:        goodCode,
			transport:   &fakeTransport{},
			want:        decodedOnly,
			wantRequest: true,
		},
		{
			name:        "different match id is not resolved",
			code:        goodCode,
			transport:   &fakeTransport{matches: []steamgc.Match{{MatchID: 42, DemoURL: "http://replay.example/other.dem.bz2"}}},
			want:        decodedOnly,
			wantRequest: true,
		},
		{
			name:        "empty demo URL is not resolved",
			code:        goodCode,
			transport:   &fakeTransport{matches: []steamgc.Match{{MatchID: goodMatchID}}},
			want:        decodedOnly,
			wantRequest: true,
		},
		{
			name:        "transport error is wrapped",
			code:        goodCode,
			transport:   &fakeTransport{err: transportErr},
			wantErr:     transportErr,
			wantRequest: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var transport Transport
			if tt.transport != nil {
				transport = tt.transport
			}
			got, err := NewService(transport).Resolve(context.Background(), tt.code)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Resolve() error = %v, want errors.Is %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("Resolve() = %+v, want %+v", got, tt.want)
			}
			if tt.wantRequest {
				wantReq := steamgc.Request{MatchID: goodMatchID, OutcomeID: goodOutcomeID, Token: goodTokenID}
				if tt.transport.got == nil {
					t.Fatal("transport received no request")
				}
				if *tt.transport.got != wantReq {
					t.Errorf("transport request = %+v, want %+v", *tt.transport.got, wantReq)
				}
			} else if tt.transport != nil && tt.transport.got != nil {
				t.Errorf("transport unexpectedly received request %+v", *tt.transport.got)
			}
		})
	}
}

func TestSessionFromEnvComplete(t *testing.T) {
	tests := []struct {
		name                      string
		username, password, guard string
		want                      bool
	}{
		{name: "all set", username: "u", password: "p", guard: "g", want: true},
		{name: "missing guard", username: "u", password: "p", want: false},
		{name: "missing password", username: "u", guard: "g", want: false},
		{name: "missing username", password: "p", guard: "g", want: false},
		{name: "none set", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvUsername, tt.username)
			t.Setenv(EnvPassword, tt.password)
			t.Setenv(EnvGuard, tt.guard)
			if got := SessionFromEnv().Complete(); got != tt.want {
				t.Errorf("SessionFromEnv().Complete() = %v, want %v", got, tt.want)
			}
		})
	}
}
