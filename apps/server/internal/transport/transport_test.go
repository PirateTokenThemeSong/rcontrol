// End-to-end API tests: spin up the real HTTP mux (httptest) with stubbed
// power/launcher side effects, then drive the full client flow:
// health -> pair -> apps -> websocket (hello/ping/quality/power/launch) -> launch.
//
// Real side effects are stubbed because tests must never shut down,
// lock, or start programs on the machine running them. Screen capture is
// real — frames are simply skipped over in the assertions.
package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"rcontrol-server/internal/desktop"
	"rcontrol-server/internal/launch"
	"rcontrol-server/internal/power"
	"rcontrol-server/internal/screen"
)

// ---- fakes (no real side effects) ----

type fakeAuth struct{ token, pin, id string }

func (f *fakeAuth) CheckToken(s string) bool { return s == f.token }
func (f *fakeAuth) CheckPIN(s string) bool   { return strings.ReplaceAll(s, "-", "") == f.pin }
func (f *fakeAuth) Token() string            { return f.token }
func (f *fakeAuth) ServerID() string         { return f.id }

type fakePower struct{ ran []power.Action }

func (f *fakePower) Run(a power.Action) error {
	f.ran = append(f.ran, a)
	return nil
}

type fakeLauncher struct {
	apps []launch.App
	ran  []string
}

func (f *fakeLauncher) List() []launch.App { return f.apps }
func (f *fakeLauncher) Search(q string) []launch.App {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	var out []launch.App
	for _, a := range f.apps {
		if strings.Contains(strings.ToLower(a.Label), q) {
			out = append(out, a)
		}
	}
	return out
}
func (f *fakeLauncher) PathFor(id string) (string, bool) {
	for _, a := range f.apps {
		if a.ID == id {
			return "C:/Windows/notepad.exe", true
		}
	}
	return "", false
}

func (f *fakeLauncher) Run(id string) error {
	for _, a := range f.apps {
		if a.ID == id {
			f.ran = append(f.ran, id)
			return nil
		}
	}
	return fmt.Errorf("unknown app %q", id)
}

type fakeUnlocker struct {
	tried []string
	err   error
}

func (f *fakeUnlocker) Unlock(secret string) error {
	f.tried = append(f.tried, secret)
	return f.err
}

type fakeWindows struct {
	list    []desktop.Window
	focused []string
	locked  bool
}

func (f *fakeWindows) Locked() bool { return f.locked }
func (f *fakeWindows) Icon(string) ([]byte, error) { return nil, fmt.Errorf("no icon") }

func (f *fakeWindows) List() []desktop.Window { return f.list }
func (f *fakeWindows) Focus(id string) error {
	for _, w := range f.list {
		if w.ID == id {
			f.focused = append(f.focused, id)
			return nil
		}
	}
	return fmt.Errorf("window is gone")
}

// ---- harness ----

type harness struct {
	t        *testing.T
	srv      *httptest.Server
	power    *fakePower
	launcher *fakeLauncher
	windows  *fakeWindows
	unlock   *fakeUnlocker
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pw := &fakePower{}
	ln := &fakeLauncher{apps: []launch.App{{ID: "calc", Label: "Calculator"}}}
	un := &fakeUnlocker{}
	win := &fakeWindows{locked: true, list: []desktop.Window{
		{ID: "11", Title: "Editor — notes.txt", Active: true},
		{ID: "22", Title: "Music", Minimized: true},
	}}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		HostName: "test-pc",
		Auth:     &fakeAuth{token: "test-token", pin: "123456", id: "test-id"},
		Capturer: screen.NewCapturer(),
		Power:    pw,
		Launcher: ln,
		Windows:  win,
		Unlocker: un,
		Port:     7945,
	})
	h := &harness{t: t, srv: httptest.NewServer(mux), power: pw, launcher: ln, windows: win, unlock: un}
	t.Cleanup(h.srv.Close)
	return h
}

func wsURL(h *harness, query string) string {
	return "ws" + strings.TrimPrefix(h.srv.URL, "http") + "/ws" + query
}

// readText reads messages until a text one arrives (binary JPEG frames skipped).
func readText(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		mt, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if mt != websocket.TextMessage {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("bad json %q: %v", data, err)
		}
		return m
	}
}

func send(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	raw, _ := json.Marshal(v)
	if err := c.WriteMessage(websocket.TextMessage, raw); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// ---- tests ----

func TestHealth(t *testing.T) {
	h := newHarness(t)
	resp, err := http.Get(h.srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["ok"] != true || body["name"] != "test-pc" {
		t.Fatalf("body = %v", body)
	}
}

func TestPairRejectsBadPIN(t *testing.T) {
	h := newHarness(t)
	resp, err := http.Post(h.srv.URL+"/pair", "application/json", strings.NewReader(`{"pin":"000-000"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestSearchEndpoints(t *testing.T) {
	h := newHarness(t)

	// No token -> 401.
	resp, err := http.Get(h.srv.URL + "/search?q=calc")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}

	// Match.
	resp, err = http.Get(h.srv.URL + "/search?q=calc&token=test-token")
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Apps []launch.App `json:"apps"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if len(body.Apps) != 1 || body.Apps[0].ID != "calc" {
		t.Fatalf("search calc = %+v", body.Apps)
	}

	// No match -> 200 with empty list (never null, so the phone can skip null checks).
	resp, err = http.Get(h.srv.URL + "/search?q=zzz-no-such-app&token=test-token")
	if err != nil {
		t.Fatal(err)
	}
	raw := make(map[string]json.RawMessage)
	_ = json.NewDecoder(resp.Body).Decode(&raw)
	resp.Body.Close()
	if string(raw["apps"]) != "[]" {
		t.Fatalf("empty search = %s, want []", raw["apps"])
	}
}

func TestAppsRequiresAuth(t *testing.T) {
	h := newHarness(t)
	resp, err := http.Get(h.srv.URL + "/apps?token=wrong")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// TestEndToEnd drives the whole client flow with a valid token.
func TestEndToEnd(t *testing.T) {
	h := newHarness(t)

	// 1. Pair with PIN -> token.
	resp, err := http.Post(h.srv.URL+"/pair", "application/json", strings.NewReader(`{"pin":"123-456"}`))
	if err != nil {
		t.Fatal(err)
	}
	var paired map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&paired)
	resp.Body.Close()
	if resp.StatusCode != 200 || paired["token"] != "test-token" {
		t.Fatalf("pair: status=%d body=%v", resp.StatusCode, paired)
	}
	token := paired["token"]

	// 2. Apps list (labels only, no command paths).
	resp, err = http.Get(h.srv.URL + "/apps?token=" + token)
	if err != nil {
		t.Fatal(err)
	}
	var appsBody struct {
		Apps []launch.App `json:"apps"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&appsBody)
	resp.Body.Close()
	if len(appsBody.Apps) != 1 || appsBody.Apps[0].ID != "calc" {
		t.Fatalf("apps = %+v", appsBody.Apps)
	}

	// 3. HTTP launch of unknown app is rejected without side effects.
	resp, err = http.Post(h.srv.URL+"/launch?token="+token, "application/json",
		strings.NewReader(`{"app":"nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("launch unknown: status = %d, want 400", resp.StatusCode)
	}
	if len(h.launcher.ran) != 0 {
		t.Fatalf("launcher ran = %v, want no side effects", h.launcher.ran)
	}

	// 4. WebSocket full session.
	c, _, err := websocket.DefaultDialer.Dial(wsURL(h, "?token="+token), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if m := readText(t, c); m["type"] != "hello" {
		t.Fatalf("want hello, got %v", m)
	}

	send(t, c, map[string]any{"type": "ping"})
	if m := readText(t, c); m["type"] != "pong" {
		t.Fatalf("want pong, got %v", m)
	}

	send(t, c, map[string]any{"type": "quality", "fps": 3, "jpeg": 60})
	if m := readText(t, c); m["type"] != "quality-ack" {
		t.Fatalf("want quality-ack, got %v", m)
	}

	// Invalid power action: rejected, nothing executed.
	send(t, c, map[string]any{"type": "power", "action": "self-destruct"})
	if m := readText(t, c); m["type"] != "error" {
		t.Fatalf("want error, got %v", m)
	}
	if len(h.power.ran) != 0 {
		t.Fatalf("power ran = %v, want no side effects", h.power.ran)
	}

	// Valid power action: recorded by the stub (real one would shut down the PC).
	send(t, c, map[string]any{"type": "power", "action": "sleep"})
	if m := readText(t, c); m["type"] != "power-ack" || m["action"] != "sleep" {
		t.Fatalf("want power-ack sleep, got %v", m)
	}
	if len(h.power.ran) != 1 || h.power.ran[0] != power.Sleep {
		t.Fatalf("power ran = %v", h.power.ran)
	}

	// Launch via WS: known app acked + recorded, unknown app rejected.
	send(t, c, map[string]any{"type": "launch", "app": "calc"})
	if m := readText(t, c); m["type"] != "launch-ack" || m["app"] != "calc" {
		t.Fatalf("want launch-ack calc, got %v", m)
	}
	send(t, c, map[string]any{"type": "launch", "app": "nope"})
	if m := readText(t, c); m["type"] != "error" {
		t.Fatalf("want error, got %v", m)
	}
	if len(h.launcher.ran) != 1 || h.launcher.ran[0] != "calc" {
		t.Fatalf("launcher ran = %v", h.launcher.ran)
	}

	// Unknown message type never kills the connection.
	send(t, c, map[string]any{"type": "warp-drive"})
	if m := readText(t, c); m["type"] != "error" {
		t.Fatalf("want error, got %v", m)
	}
	send(t, c, map[string]any{"type": "ping"})
	if m := readText(t, c); m["type"] != "pong" {
		t.Fatalf("connection died after unknown type: %v", m)
	}
}

func TestWebSocketRejectsBadToken(t *testing.T) {
	h := newHarness(t)
	c, _, err := websocket.DefaultDialer.Dial(wsURL(h, "?token=wrong"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, _, err = c.ReadMessage()
	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("want close error, got %v", err)
	}
	if closeErr.Code != 4401 {
		t.Fatalf("close code = %d, want 4401", closeErr.Code)
	}
}

func TestWindowsListAndFocus(t *testing.T) {
	h := newHarness(t)

	resp, err := http.Get(h.srv.URL + "/windows?token=test-token")
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Windows []desktop.Window `json:"windows"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if resp.StatusCode != 200 || len(body.Windows) != 2 {
		t.Fatalf("windows: status=%d body=%+v", resp.StatusCode, body)
	}
	if body.Windows[0].Title != "Editor — notes.txt" || !body.Windows[0].Active {
		t.Fatalf("first window = %+v", body.Windows[0])
	}

	// Lock state travels with the list and on its own endpoint.
	resp, err = http.Get(h.srv.URL + "/windows?token=test-token")
	if err != nil {
		t.Fatal(err)
	}
	var withLock struct {
		Locked bool `json:"locked"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&withLock)
	resp.Body.Close()
	if !withLock.Locked {
		t.Fatal("windows: locked should be true")
	}
	resp, err = http.Get(h.srv.URL + "/lockstate?token=test-token")
	if err != nil {
		t.Fatal(err)
	}
	var lockBody struct {
		Locked bool `json:"locked"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&lockBody)
	resp.Body.Close()
	if !lockBody.Locked {
		t.Fatal("lockstate should be true")
	}

	// Unknown id is rejected without side effects.
	resp, err = http.Post(h.srv.URL+"/focus?token=test-token", "application/json",
		strings.NewReader(`{"id":"99"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 || len(h.windows.focused) != 0 {
		t.Fatalf("focus unknown: status=%d focused=%v", resp.StatusCode, h.windows.focused)
	}

	// Known id is focused.
	resp, err = http.Post(h.srv.URL+"/focus?token=test-token", "application/json",
		strings.NewReader(`{"id":"22"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || len(h.windows.focused) != 1 || h.windows.focused[0] != "22" {
		t.Fatalf("focus: status=%d focused=%v", resp.StatusCode, h.windows.focused)
	}

	// WebSocket path returns the same list and can focus.
	c, _, err := websocket.DefaultDialer.Dial(wsURL(h, "?token=test-token"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if m := readText(t, c); m["type"] != "hello" {
		t.Fatalf("want hello, got %v", m)
	}
	send(t, c, map[string]any{"type": "windows"})
	m := readText(t, c)
	if m["type"] != "windows" {
		t.Fatalf("want windows, got %v", m)
	}
	list, _ := m["windows"].([]any)
	if len(list) != 2 {
		t.Fatalf("ws windows = %v", m["windows"])
	}
	send(t, c, map[string]any{"type": "focus", "id": "11"})
	if m := readText(t, c); m["type"] != "focus-ack" || m["id"] != "11" {
		t.Fatalf("want focus-ack 11, got %v", m)
	}
}

func TestUnlockEndpoint(t *testing.T) {
	h := newHarness(t)

	// Empty secret is rejected before touching the helper.
	resp, err := http.Post(h.srv.URL+"/unlock?token=test-token", "application/json", strings.NewReader(`{"secret":""}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 || len(h.unlock.tried) != 0 {
		t.Fatalf("empty secret: status=%d tried=%v", resp.StatusCode, h.unlock.tried)
	}

	// A secret is forwarded to the helper.
	resp, err = http.Post(h.srv.URL+"/unlock?token=test-token", "application/json", strings.NewReader(`{"secret":"1234"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || len(h.unlock.tried) != 1 || h.unlock.tried[0] != "1234" {
		t.Fatalf("unlock: status=%d tried=%v", resp.StatusCode, h.unlock.tried)
	}

	// Helper failures surface as 400.
	h.unlock.err = fmt.Errorf("typist exited with 3")
	resp, err = http.Post(h.srv.URL+"/unlock?token=test-token", "application/json", strings.NewReader(`{"secret":"1"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("helper error: status = %d, want 400", resp.StatusCode)
	}
}
