package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"devices":{"living":{"url":"http://localhost:8080","api_key":"secret"},"desk":{"name":"Desk","url":"http://localhost:8081"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FREEKIOSK_API_KEY_living", "override")
	t.Setenv("FREEKIOSK_URL_living", "http://localhost:9000")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Devices["living"].APIKey != "override" || cfg.Devices["living"].Name != "living" || cfg.Devices["living"].URL != "http://localhost:9000" || len(cfg.Devices) != 2 {
		t.Fatal("wrong configuration")
	}
	for _, body := range []string{`{}`, `{"devices":{}}`, `{"devices":{"../x":{"url":"http://localhost"}}}`, `{"devices":{"desk":{"url":"http://user:secret@localhost"}}}`, `{"devices":{"desk":{"url":"http://localhost","unknown":1}}}`, `{"devices":{}} {}`, `{"devices":{"desk":{"url": 123}}}`} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(path)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("invalid config or leaked key", err)
		}
	}
}
