package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/k1LoW/errors"

	"github.com/AnaTofuZ/freekiosk-manager/internal/freekiosk"
)

type Device struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}
type Config struct {
	Devices map[string]Device `json:"devices"`
}

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func DefaultPath() string {
	if p := os.Getenv("FREEKIOSK_CONFIG"); p != "" {
		return p
	}
	return "freekiosk.json"
}
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	var cfg Config
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&cfg) != nil || d.Decode(new(any)) != io.EOF {
		return nil, errors.New("invalid configuration JSON")
	}
	if len(cfg.Devices) == 0 {
		return nil, errors.New("configure at least one device")
	}
	for id, dev := range cfg.Devices {
		if !idPattern.MatchString(id) {
			return nil, errors.New("device ID must match [a-z][a-z0-9_]{0,63}")
		}
		if key, ok := os.LookupEnv("FREEKIOSK_API_KEY_" + id); ok {
			dev.APIKey = key
		}
		if raw, ok := os.LookupEnv("FREEKIOSK_URL_" + id); ok {
			dev.URL = raw
		}
		if dev.Name == "" {
			dev.Name = id
		}
		if _, err := freekiosk.New(dev.URL, dev.APIKey, 8*time.Second); err != nil {
			return nil, fmt.Errorf("device %s: %w", id, err)
		}
		cfg.Devices[id] = dev
	}
	return &cfg, nil
}
