package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AnaTofuZ/freekiosk-manager/internal/config"
)

func TestLanguages(t *testing.T) {
	if len(english) != len(japanese) {
		t.Fatal("translation keys differ")
	}
	for key, value := range english {
		if value == "" || japanese[key] == "" {
			t.Fatalf("missing translation: %s", key)
		}
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{"battery":{"level":85,"charging":true}}}`))
	}))
	defer upstream.Close()
	h, err := New(&config.Config{Devices: map[string]config.Device{"living": {Name: "Living", URL: upstream.URL, APIKey: "private-key"}}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, cookie, want, lang string }{
		{"/", "", "Devices", "en"},
		{"/?lang=ja", "", "デバイス一覧", "ja"},
		{"/devices/living?lang=ja", "", "再読み込み", "ja"},
		{"/devices/living", "ja", "画面をOFF", "ja"},
		{"/devices/living?lang=en", "ja", "Screen OFF", "en"},
		{"/?lang=unsupported", "", "Devices", "en"},
		{"/fragments/devices/living/status?lang=ja", "", "バッテリー", ""},
		{"/fragments/devices/living/status?lang=en", "ja", "Battery", ""},
	} {
		r := httptest.NewRequestWithContext(t.Context(), "GET", tc.path, nil)
		if tc.cookie != "" {
			r.AddCookie(&http.Cookie{Name: "language", Value: tc.cookie})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		body := w.Body.String()
		if w.Code != 200 || !strings.Contains(body, tc.want) || strings.Contains(body, "private-key") || strings.Contains(body, "&lt;no value&gt;") {
			t.Fatalf("%s: %d %s", tc.path, w.Code, body)
		}
		if tc.lang != "" {
			if !strings.Contains(body, `lang="`+tc.lang+`"`) || !strings.Contains(w.Header().Get("Set-Cookie"), "language="+tc.lang) {
				t.Fatalf("wrong language: %s", tc.path)
			}
		}
	}
	if translate("ja", "Authentication failed") != "認証に失敗しました" || translate("ja", "Device returned HTTP 503") != "HTTPエラー (503)" {
		t.Fatal("errors not translated")
	}
}
