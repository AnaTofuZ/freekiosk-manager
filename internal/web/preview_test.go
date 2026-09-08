package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/AnaTofuZ/freekiosk-manager/internal/config"
)

// Opt-in preview for UI checks and documentation; never contacts a real tablet.
func TestPreview(t *testing.T) {
	addr := os.Getenv("FREEKIOSK_PREVIEW")
	if addr == "" {
		t.Skip("set FREEKIOSK_PREVIEW=127.0.0.1:8099 for the mock UI")
	}
	var mu sync.Mutex
	brightness, volume, screen, current := 30, 20, true, "https://signage.home/"
	var shot bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 800, 480))
	for y := 0; y < 480; y++ {
		for x := 0; x < 800; x++ {
			img.Set(x, y, color.RGBA{uint8(30 + x/8), uint8(70 + y/4), 95, 255})
		}
	}
	if err := png.Encode(&shot, img); err != nil {
		t.Fatal(err)
	}
	tablet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/api/screenshot" {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(shot.Bytes())
			return
		}
		if r.URL.Path == "/api/status" {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
				"battery": map[string]any{"level": 85, "charging": true}, "screen": map[string]any{"on": screen, "brightness": brightness, "screensaverActive": false}, "audio": map[string]any{"volume": volume}, "webview": map[string]any{"currentUrl": current}, "wifi": map[string]any{"connected": true, "ssid": "Home", "rssi": -45, "ip": "192.168.1.50"}, "device": map[string]any{"model": "Lenovo Tab M8", "ip": "192.168.1.50"},
			}})
			return
		}
		var p struct {
			Value int    `json:"value"`
			URL   string `json:"url"`
		}
		_ = json.NewDecoder(r.Body).Decode(&p)
		switch r.URL.Path {
		case "/api/brightness":
			brightness = p.Value
		case "/api/volume":
			volume = p.Value
		case "/api/screen/on":
			screen = true
		case "/api/screen/off":
			screen = false
		case "/api/url":
			current = p.URL
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]bool{"executed": true}})
	}))
	defer tablet.Close()
	handler, err := New(&config.Config{Devices: map[string]config.Device{
		"living": {Name: "Living Room", URL: tablet.URL, APIKey: "preview-only"},
		"desk":   {Name: "Desk", URL: "http://127.0.0.1:1", APIKey: "preview-only"},
	}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Mock UI at http://" + addr)
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: time.Second}
	t.Fatal(server.ListenAndServe())
}
