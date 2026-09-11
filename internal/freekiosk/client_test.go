package freekiosk

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/k1LoW/errors"
)

const statusJSON = `{"success":true,"data":{"battery":{"level":85,"charging":true},"screen":{"on":true,"brightness":30},"webview":{"currentUrl":"https://signage.home/"},"audio":{"volume":20},"wifi":{"connected":true,"ssid":"Home","ip":"192.168.1.50"}}}`

func TestClient(t *testing.T) {
	for _, tc := range []struct {
		action, path, body string
		p                  Command
	}{
		{"status", "/api/status", "", Command{}},
		{"reload", "/api/reload", "", Command{}},
		{"clear-cache", "/api/clearCache", "", Command{}},
		{"brightness", "/api/brightness", `{"value":30}`, Command{Value: new(30)}},
		{"volume", "/api/volume", `{"value":20}`, Command{Value: new(20)}},
		{"screen", "/api/screen/off", "", Command{State: "off"}},
		{"screen", "/api/screen/on", "", Command{State: "on"}},
		{"url", "/api/url", `{"url":"https://signage.home/"}`, Command{URL: "https://signage.home/"}},
		{"toast", "/api/toast", `{"text":"hello"}`, Command{Text: "hello"}},
		{"tts", "/api/tts", `{"text":"夕食","language":"ja"}`, Command{Text: "夕食", Language: "ja"}},
	} {
		t.Run(tc.action+tc.path, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path || r.Header.Get("X-Api-Key") != "secret" {
					t.Error("wrong path or missing authentication")
				}
				method := http.MethodPost
				if tc.action == "status" {
					method = http.MethodGet
				}
				if r.Method != method {
					t.Errorf("method = %s", r.Method)
				}
				if tc.body != "" {
					var got, want any
					if json.NewDecoder(r.Body).Decode(&got) != nil {
						t.Error("invalid request JSON")
					}
					_ = json.Unmarshal([]byte(tc.body), &want)
					g, _ := json.Marshal(got)
					v, _ := json.Marshal(want)
					if !bytes.Equal(g, v) {
						t.Errorf("body = %s, want %s", g, v)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				if tc.action == "status" {
					_, _ = w.Write([]byte(statusJSON))
				} else {
					_, _ = w.Write([]byte(`{"success":true,"data":{"executed":true}}`))
				}
			}))
			defer srv.Close()
			c, err := New(srv.URL, "secret", time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if tc.action == "status" {
				s, err := c.Status(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if *s.Battery.Level != 85 || *s.Audio.Volume != 20 {
					t.Fatal("incorrect status")
				}
				u, err := c.CurrentURL(t.Context())
				if err != nil || u != "https://signage.home/" {
					t.Fatal(u, err)
				}
			} else if err := c.Execute(t.Context(), tc.action, tc.p); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, kind string
		status           int
		delay            bool
	}{
		{"authentication", "secret", "authentication", 401, false},
		{"forbidden", "secret", "forbidden", 403, false},
		{"http", "secret", "http", 500, false},
		{"malformed", "{", "invalid_response", 200, false},
		{"empty", "{}", "invalid_response", 200, false},
		{"null", `{"success":true,"data":null}`, "invalid_response", 200, false},
		{"schema", `{"success":true,"data":{"battery":{"level":"bad"}}}`, "invalid_response", 200, false},
		{"unknown status", `{"success":true,"data":{}}`, "invalid_response", 200, false},
		{"rejected", `{"success":false,"error":"secret"}`, "api", 200, false},
		{"timeout", "", "timeout", 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.delay {
					<-r.Context().Done()
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c, _ := New(srv.URL, "secret", 30*time.Millisecond)
			_, err := c.Status(t.Context())
			var e *Error
			if !errors.As(err, &e) || e.Kind != tc.kind {
				t.Fatalf("got %v, want %s", err, tc.kind)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("leaked key")
			}
		})
	}
	t.Run("connection refused", func(t *testing.T) {
		l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := l.Addr().String()
		_ = l.Close()
		c, _ := New("http://"+addr, "", time.Second)
		_, err = c.Status(t.Context())
		var e *Error
		if !errors.As(err, &e) || e.Kind != "connection_refused" {
			t.Fatal(err)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		c, _ := New("http://127.0.0.1:1", "", time.Second)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := c.Status(ctx)
		var e *Error
		if !errors.As(err, &e) || e.Kind != "canceled" {
			t.Fatal(err)
		}
	})
}
func TestValidationAndRedirect(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "http://user:secret@localhost", "http://localhost/api", "http://localhost?key=secret"} {
		if _, err := New(raw, "", time.Second); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	sink := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { t.Error("followed redirect with credentials") }))
	defer sink.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, sink.URL, http.StatusFound) }))
	defer srv.Close()
	c, _ := New(srv.URL, "secret", time.Second)
	if _, err := c.Status(t.Context()); err == nil {
		t.Fatal("accepted redirect")
	}
	for _, tc := range []struct {
		action string
		p      Command
	}{{"brightness", Command{}}, {"volume", Command{Value: new(101)}}, {"screen", Command{State: "maybe"}}, {"url", Command{URL: "javascript:alert(1)"}}, {"toast", Command{Text: " "}}, {"js", Command{}}} {
		if err := c.Execute(t.Context(), tc.action, tc.p); err == nil {
			t.Fatal("accepted invalid command")
		}
	}
}
func TestScreenshotAndCommandFailure(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, content, body string
		ok                  bool
	}{
		{"png", "image/png", b.String(), true}, {"bad png", "image/png", "garbage", false}, {"json", "application/json", `{"success":false}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/screenshot" || r.Header.Get("X-Api-Key") != "secret" {
					t.Error("wrong screenshot request")
				}
				w.Header().Set("Content-Type", tc.content)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c, _ := New(srv.URL, "secret", time.Second)
			_, err := c.Screenshot(t.Context())
			if (err == nil) != tc.ok {
				t.Fatal(err)
			}
		})
	}
	for _, body := range []string{`{"success":true,"data":{"executed":false,"error":"secret"}}`, `{"success":true,"data":{}}`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
		c, _ := New(srv.URL, "secret", time.Second)
		err := c.Execute(t.Context(), "reload", Command{})
		srv.Close()
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("invalid command response accepted", err)
		}
	}
}
