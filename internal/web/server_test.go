package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AnaTofuZ/freekiosk-manager/internal/config"
)

func TestHandlers(t *testing.T) {
	var fail atomic.Bool
	var calls atomic.Int32
	var img bytes.Buffer
	_ = png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("X-Api-Key") != "secret" {
			t.Error("missing key")
		}
		if fail.Load() {
			http.Error(w, "secret", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"battery":{"level":85,"charging":true},"screen":{"on":true,"brightness":30},"webview":{"currentUrl":"<script>alert(1)</script>"},"wifi":{"ssid":"Home"}}}`))
		case "/api/screenshot":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(img.Bytes())
		case "/api/brightness":
			var body struct {
				Value int `json:"value"`
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil || body.Value != 30 {
				t.Error("wrong brightness body")
			}
			fallthrough
		default:
			if r.Method != "POST" {
				t.Error("wrong command method")
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"executed":true}}`))
		}
	}))
	defer upstream.Close()
	h, err := New(&config.Config{Devices: map[string]config.Device{"living": {Name: "Living", URL: upstream.URL, APIKey: "secret"}}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if strings.Contains(w.Body.String(), "secret") {
			t.Fatal("key leaked")
		}
		return w
	}
	for _, path := range []string{"/", "/devices/living", "/api/devices", "/static/generated/main.js", "/static/styles.css"} {
		w := call("GET", path, "")
		if w.Code != 200 {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
	if calls.Load() != 0 {
		t.Fatal("page render blocked on device")
	}
	status := call("GET", "/api/devices/living/status", "")
	var snap Snapshot
	if json.Unmarshal(status.Body.Bytes(), &snap) != nil || !snap.Online || snap.LastSuccess == nil {
		t.Fatal(status.Body.String())
	}
	fragment := call("GET", "/fragments/devices/living/status", "")
	if strings.Contains(fragment.Body.String(), "<script>") || !strings.Contains(fragment.Body.String(), "85") {
		t.Fatal("unsafe or missing status")
	}
	if calls.Load() != 1 {
		t.Fatal("concurrent polls not cached")
	}
	for _, tc := range []struct{ action, body string }{{"reload", "{}"}, {"clear-cache", "{}"}, {"screen", `{"state":"off"}`}, {"brightness", `{"value":30}`}, {"volume", `{"value":20}`}, {"url", `{"url":"https://signage.home/"}`}, {"toast", `{"text":"hello"}`}, {"tts", `{"text":"夕食","language":"ja"}`}} {
		w := call("POST", "/api/devices/living/"+tc.action, tc.body)
		if w.Code != 200 {
			t.Fatal(tc.action, w.Body.String())
		}
	}
	shot := call("GET", "/api/devices/living/screenshot", "")
	if shot.Header().Get("Content-Type") != "image/png" || !bytes.Equal(shot.Body.Bytes(), img.Bytes()) {
		t.Fatal("invalid screenshot")
	}
	for _, body := range []string{`{`, `{"value":101}`, `{"value":0.5}`, `{}`, `{"value":30,"extra":1}`, `{"value":30} {}`} {
		if w := call("POST", "/api/devices/living/brightness", body); w.Code != 400 {
			t.Fatal("invalid request accepted", body, w.Code)
		}
	}
	if w := call("POST", "/api/devices/missing/reload", "{}"); w.Code != 404 {
		t.Fatal("unknown device accepted")
	}
	before := calls.Load()
	r := httptest.NewRequestWithContext(t.Context(), "POST", "http://manager/api/devices/living/reload", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 || calls.Load() != before {
		t.Fatal("cross origin command accepted")
	}
	fail.Store(true)
	stale := call("GET", "/api/devices/living/status", "")
	var last Snapshot
	_ = json.Unmarshal(stale.Body.Bytes(), &last)
	if last.Online || last.Error == nil || last.Error.Kind != "authentication" || last.LastSuccess == nil || !last.LastSuccess.Equal(*snap.LastSuccess) || last.Status == nil {
		t.Fatal("last good status lost", stale.Body.String())
	}
}
