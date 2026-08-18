package steamresolve

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/sharecode"
)

func secondShareCode(t *testing.T) string {
	t.Helper()
	return sharecode.Encode(sharecode.Match{MatchID: 1, OutcomeID: 2, TokenID: 3})
}

func TestHistoryClientNextShareCode(t *testing.T) {
	next := secondShareCode(t)
	tests := []struct {
		name      string
		status    int
		body      string
		known     string
		acc       Account
		want      string
		wantErr   bool
		wantQuery bool
	}{
		{
			name:      "next code",
			status:    http.StatusOK,
			body:      `{"result":{"nextcode":"` + next + `"}}`,
			known:     goodCode,
			acc:       validHistoryAccount(),
			want:      next,
			wantQuery: true,
		},
		{
			name:      "n/a means end of chain",
			status:    http.StatusOK,
			body:      `{"result":{"nextcode":"n/a"}}`,
			known:     goodCode,
			acc:       validHistoryAccount(),
			wantQuery: true,
		},
		{
			name:    "missing known code",
			acc:     validHistoryAccount(),
			wantErr: true,
		},
		{
			name:    "account not configured",
			known:   goodCode,
			acc:     Account{SteamID: "76561198000000001"},
			wantErr: true,
		},
		{
			name:      "forbidden",
			status:    http.StatusForbidden,
			body:      `{}`,
			known:     goodCode,
			acc:       validHistoryAccount(),
			wantErr:   true,
			wantQuery: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.RawQuery
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			t.Cleanup(server.Close)
			client := NewHistoryClient(server.Client())
			client.baseURL = server.URL + "/"
			got, err := client.NextShareCode(context.Background(), tt.acc, tt.known)
			if tt.wantErr {
				if err == nil {
					t.Fatal("error = nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if got != tt.want {
				t.Errorf("next = %q, want %q", got, tt.want)
			}
			if tt.wantQuery {
				for _, field := range []string{tt.acc.APIKey, tt.acc.SteamID, tt.acc.AuthCode} {
					if !strings.Contains(gotQuery, urlQueryEscape(field)) && !strings.Contains(gotQuery, field) {
						t.Errorf("query %q missing %q", gotQuery, field)
					}
				}
			}
		})
	}
}

func urlQueryEscape(v string) string {
	return strings.ReplaceAll(v, " ", "+")
}

func TestHistoryClientWalkStopsAtNA(t *testing.T) {
	codes := []string{goodCode, secondShareCode(t)}
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		known := r.URL.Query().Get("knowncode")
		calls++
		next := "n/a"
		if known == codes[0] {
			next = codes[1]
		}
		_, _ = io.WriteString(w, `{"result":{"nextcode":"`+next+`"}}`)
	}))
	t.Cleanup(server.Close)
	client := NewHistoryClient(server.Client())
	client.baseURL = server.URL + "/"
	known, matches, err := client.Walk(context.Background(), Account{
		SteamID:   "76561198000000001",
		AuthCode:  "AAAAA-BBBBB-CCCCC",
		APIKey:    "0123456789ABCDEF",
		KnownCode: codes[0],
	}, 8)
	if err != nil {
		t.Fatal(err)
	}
	if known != codes[1] {
		t.Errorf("known = %q, want %q", known, codes[1])
	}
	if len(matches) != 1 || matches[0].ShareCode != codes[1] {
		t.Errorf("matches = %+v", matches)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func validHistoryAccount() Account {
	return Account{
		SteamID:  "76561198000000001",
		AuthCode: "AAAAA-BBBBB-CCCCC",
		APIKey:   "0123456789ABCDEF",
	}
}
