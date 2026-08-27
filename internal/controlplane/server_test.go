package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAccountAndDevicePairingJourney(t *testing.T) {
	store := openTestStore(t)
	now := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	server := newTestServer(t, store)
	client := newCookieClient(t)

	register := requestJSON(t, client, http.MethodPost, server.URL+"/api/account/register", map[string]string{
		"email": "Player@Example.com", "password": "correct-horse-battery",
	}, "")
	assertStatus(t, register, http.StatusCreated)

	session := requestJSON(t, client, http.MethodGet, server.URL+"/api/account/session", nil, "")
	assertStatus(t, session, http.StatusOK)
	var sessionBody struct {
		User User `json:"user"`
	}
	decodeResponse(t, session, &sessionBody)
	if sessionBody.User.Email != "player@example.com" {
		t.Fatalf("session email = %q, want normalized email", sessionBody.User.Email)
	}

	deviceID := uuid.NewString()
	secret := strings.Repeat("a", 64)
	pairingResponse := requestJSON(t, http.DefaultClient, http.MethodPost, server.URL+"/api/agent/pairings", pairingRequest{
		DeviceID: deviceID,
		Name:     "Gaming PC",
		Platform: "windows-amd64",
		Version:  "2.5.0",
		Secret:   secret,
	}, "")
	assertStatus(t, pairingResponse, http.StatusCreated)
	var pairingBody struct {
		Pairing Pairing `json:"pairing"`
	}
	decodeResponse(t, pairingResponse, &pairingBody)
	if pairingBody.Pairing.DeviceID != deviceID || len(pairingBody.Pairing.Code) != 10 {
		t.Fatalf("pairing = %+v, want device and ten-character code", pairingBody.Pairing)
	}

	claim := requestJSON(t, client, http.MethodPost, server.URL+"/api/account/devices/claim", map[string]string{
		"code": pairingBody.Pairing.Code[:5] + "-" + pairingBody.Pairing.Code[5:],
	}, "")
	assertStatus(t, claim, http.StatusOK)

	status := requestJSON(t, http.DefaultClient, http.MethodPost, server.URL+"/api/agent/pairings/"+deviceID+"/status", struct{}{}, secret)
	assertStatus(t, status, http.StatusOK)
	var statusBody struct {
		Claimed bool `json:"claimed"`
	}
	decodeResponse(t, status, &statusBody)
	if !statusBody.Claimed {
		t.Fatal("pairing claimed = false, want true")
	}

	heartbeat := requestJSON(t, http.DefaultClient, http.MethodPost, server.URL+"/api/agent/heartbeat", map[string]string{
		"deviceId": deviceID, "version": "2.5.1",
	}, secret)
	assertStatus(t, heartbeat, http.StatusNoContent)

	devices := requestJSON(t, client, http.MethodGet, server.URL+"/api/account/devices", nil, "")
	assertStatus(t, devices, http.StatusOK)
	var devicesBody struct {
		Devices []Device `json:"devices"`
	}
	decodeResponse(t, devices, &devicesBody)
	if len(devicesBody.Devices) != 1 || !devicesBody.Devices[0].Online || devicesBody.Devices[0].Version != "2.5.1" {
		t.Fatalf("devices = %+v, want one online updated device", devicesBody.Devices)
	}

	deleted := requestJSON(t, client, http.MethodDelete, server.URL+"/api/account/devices/"+deviceID, nil, "")
	assertStatus(t, deleted, http.StatusNoContent)
}

func TestAuthenticationAndOwnershipFailures(t *testing.T) {
	store := openTestStore(t)
	server := newTestServer(t, store)
	owner := newCookieClient(t)
	other := newCookieClient(t)
	for _, registration := range []struct {
		client *http.Client
		email  string
	}{
		{client: owner, email: "owner@example.com"},
		{client: other, email: "other@example.com"},
	} {
		response := requestJSON(t, registration.client, http.MethodPost, server.URL+"/api/account/register", map[string]string{
			"email": registration.email, "password": "correct-horse-battery",
		}, "")
		assertStatus(t, response, http.StatusCreated)
	}

	badLogin := requestJSON(t, newCookieClient(t), http.MethodPost, server.URL+"/api/account/login", map[string]string{
		"email": "owner@example.com", "password": "incorrect-password",
	}, "")
	assertStatus(t, badLogin, http.StatusUnauthorized)

	deviceID := uuid.NewString()
	secret := strings.Repeat("b", 64)
	pairing := requestJSON(t, http.DefaultClient, http.MethodPost, server.URL+"/api/agent/pairings", pairingRequest{
		DeviceID: deviceID, Name: "Owner PC", Platform: "windows-amd64", Version: "1.0.0", Secret: secret,
	}, "")
	assertStatus(t, pairing, http.StatusCreated)
	var body struct {
		Pairing Pairing `json:"pairing"`
	}
	decodeResponse(t, pairing, &body)
	claim := requestJSON(t, owner, http.MethodPost, server.URL+"/api/account/devices/claim", map[string]string{"code": body.Pairing.Code}, "")
	assertStatus(t, claim, http.StatusOK)

	foreignDelete := requestJSON(t, other, http.MethodDelete, server.URL+"/api/account/devices/"+deviceID, nil, "")
	assertStatus(t, foreignDelete, http.StatusNotFound)
	wrongSecret := requestJSON(t, http.DefaultClient, http.MethodPost, server.URL+"/api/agent/heartbeat", map[string]string{
		"deviceId": deviceID, "version": "1.0.0",
	}, strings.Repeat("c", 64))
	assertStatus(t, wrongSecret, http.StatusUnauthorized)
}

func TestMutationOriginGuard(t *testing.T) {
	store := openTestStore(t)
	server := newTestServer(t, store)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/account/login", bytes.NewBufferString(`{"email":"a@example.com","password":"long-enough-password"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, response, http.StatusForbidden)
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "control.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return store
}

func newTestServer(t *testing.T, store *Store) *httptest.Server {
	t.Helper()
	server, err := NewServer(store, "http://cliphub.test", log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)
	return httpServer
}

func newCookieClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

func requestJSON(t *testing.T, client *http.Client, method, target string, body any, bearer string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, target, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func assertStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if response.StatusCode != want {
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		t.Fatalf("status = %d body = %q, want %d", response.StatusCode, body, want)
	}
	if response.StatusCode == http.StatusNoContent {
		_ = response.Body.Close()
	}
}

func decodeResponse(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatal(err)
	}
}
