// Package freekiosk implements the documented FreeKiosk LAN REST API.
package freekiosk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/k1LoW/errors"
)

type Error struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func (e *Error) Error() string           { return e.Message }
func failure(kind, message string) error { return errors.WithStack(&Error{kind, message}) }
func Invalid(message string) error       { return failure("validation", message) }

// Errors deliberately omit upstream bodies, URLs and transport error strings:
// these can contain credentials echoed by a device or proxy.
func transportError(err error) error {
	var ne net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return failure("canceled", "Request canceled")
	case errors.As(err, &ne) && ne.Timeout():
		return failure("timeout", "Device timed out")
	case errors.Is(err, syscall.ECONNREFUSED):
		return failure("connection_refused", "Device connection refused")
	default:
		return failure("network", "Device is unreachable")
	}
}

type Client struct {
	base, key string
	http      *http.Client
}

func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || strings.ContainsAny(raw, "\r\n") {
		return Invalid("URL must be an absolute HTTP(S) URL without credentials")
	}
	return nil
}
func New(base, key string, timeout time.Duration) (*Client, error) {
	if err := ValidateURL(base); err != nil {
		return nil, err
	}
	u, _ := url.Parse(base)
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, Invalid("Device URL must contain only scheme, host and port")
	}
	if timeout <= 0 || timeout > time.Minute {
		return nil, Invalid("Timeout must be greater than zero and at most one minute")
	}
	if strings.ContainsAny(key, "\r\n") {
		return nil, Invalid("Invalid API key")
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil // LAN credentials must not pass through environment-configured proxies.
	return &Client{strings.TrimRight(base, "/"), key, &http.Client{Timeout: timeout, Transport: tr, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *Client) request(ctx context.Context, method, path string, body any, limit int64) ([]byte, string, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, "", err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return nil, "", Invalid("Invalid request")
	}
	req.Header.Set("X-Api-Key", c.key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, "", transportError(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == 401 {
		return nil, "", failure("authentication", "Authentication failed")
	}
	if res.StatusCode == 403 {
		return nil, "", failure("forbidden", "Remote control is disabled or access is forbidden")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, "", failure("http", fmt.Sprintf("Device returned HTTP %d", res.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, "", transportError(err)
	}
	if int64(len(data)) > limit {
		return nil, "", failure("invalid_response", "Device response is too large")
	}
	return data, res.Header.Get("Content-Type"), nil
}

func (c *Client) json(ctx context.Context, method, path string, body, out any) error {
	data, _, err := c.request(ctx, method, path, body, 1<<20)
	if err != nil {
		return err
	}
	var envelope struct {
		Success *bool           `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if json.Unmarshal(data, &envelope) != nil || envelope.Success == nil {
		return failure("invalid_response", "Invalid API response")
	}
	if !*envelope.Success {
		return failure("api", "Device rejected the command")
	}
	if len(envelope.Data) == 0 || envelope.Data[0] != '{' || json.Unmarshal(envelope.Data, out) != nil {
		return failure("invalid_response", "Invalid API response data")
	}
	return nil
}

// Optional fields remain nil rather than reporting missing values as zero/off.
type Status struct {
	Battery *struct {
		Level    *int  `json:"level"`
		Charging *bool `json:"charging"`
	} `json:"battery,omitempty"`
	Screen *struct {
		On                *bool `json:"on"`
		Brightness        *int  `json:"brightness"`
		ScreensaverActive *bool `json:"screensaverActive"`
	} `json:"screen,omitempty"`
	Webview *struct {
		CurrentURL *string `json:"currentUrl"`
	} `json:"webview,omitempty"`
	Audio *struct {
		Volume *int `json:"volume"`
	} `json:"audio,omitempty"`
	Wifi *struct {
		Connected *bool   `json:"connected"`
		SSID      *string `json:"ssid"`
		RSSI      *int    `json:"rssi"`
		IP        *string `json:"ip"`
	} `json:"wifi,omitempty"`
	Device *struct {
		IP    *string `json:"ip"`
		Model *string `json:"model"`
	} `json:"device,omitempty"`
}

func (c *Client) Status(ctx context.Context) (*Status, error) {
	var s Status
	if err := c.json(ctx, http.MethodGet, "/api/status", nil, &s); err != nil {
		return nil, err
	}
	if s.Battery == nil && s.Screen == nil && s.Webview == nil && s.Audio == nil && s.Wifi == nil && s.Device == nil {
		return nil, failure("invalid_response", "Status contains no recognized fields")
	}
	return &s, nil
}
func (c *Client) CurrentURL(ctx context.Context) (string, error) {
	s, err := c.Status(ctx)
	if err != nil {
		return "", err
	}
	if s.Webview == nil || s.Webview.CurrentURL == nil {
		return "", failure("invalid_response", "Current URL is unavailable")
	}
	return *s.Webview.CurrentURL, nil
}

type Command struct {
	Value    *int   `json:"value,omitempty"`
	URL      string `json:"url,omitempty"`
	Text     string `json:"text,omitempty"`
	State    string `json:"state,omitempty"`
	Language string `json:"language,omitempty"`
}

func (c *Client) Execute(ctx context.Context, action string, p Command) error {
	path := "/api/" + action
	var body any
	switch action {
	case "reload":
	case "clear-cache":
		path = "/api/clearCache"
	case "screen":
		if p.State != "on" && p.State != "off" {
			return Invalid("Screen state must be on or off")
		}
		path += "/" + p.State
	case "brightness", "volume":
		if p.Value == nil || *p.Value < 0 || *p.Value > 100 {
			return Invalid("Value must be an integer from 0 to 100")
		}
		body = struct {
			Value int `json:"value"`
		}{*p.Value}
	case "url":
		if len(p.URL) > 8192 {
			return Invalid("URL is too long")
		}
		if err := ValidateURL(p.URL); err != nil {
			return err
		}
		body = struct {
			URL string `json:"url"`
		}{p.URL}
	case "toast", "tts":
		if strings.TrimSpace(p.Text) == "" || len(p.Text) > 4096 {
			return Invalid("Text must contain 1–4096 bytes")
		}
		if len(p.Language) > 35 {
			return Invalid("Language tag is too long")
		}
		body = struct {
			Text     string `json:"text"`
			Language string `json:"language,omitempty"`
		}{p.Text, p.Language}
	default:
		return Invalid("Unknown command")
	}
	var result struct {
		Executed *bool  `json:"executed"`
		Error    string `json:"error"`
	}
	if err := c.json(ctx, http.MethodPost, path, body, &result); err != nil {
		return err
	}
	if result.Executed == nil {
		return failure("invalid_response", "Missing command execution result")
	}
	if !*result.Executed || result.Error != "" {
		return failure("api", "Device could not execute the command")
	}
	return nil
}
func (c *Client) Screenshot(ctx context.Context) ([]byte, error) {
	data, contentType, err := c.request(ctx, http.MethodGet, "/api/screenshot", nil, 16<<20)
	if err != nil {
		return nil, err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || !strings.HasPrefix(contentType, "image/png") || cfg.Width > 8192 || cfg.Height > 8192 {
		return nil, failure("invalid_response", "Invalid PNG screenshot")
	}
	return data, nil
}
