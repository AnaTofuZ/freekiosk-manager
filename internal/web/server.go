package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/k1LoW/errors"

	bf "github.com/barefootjs/runtime/bf"

	"github.com/AnaTofuZ/freekiosk-manager/internal/config"
	"github.com/AnaTofuZ/freekiosk-manager/internal/freekiosk"
	assets "github.com/AnaTofuZ/freekiosk-manager/web"
	"github.com/AnaTofuZ/freekiosk-manager/web/views"
)

type Snapshot struct {
	Status      *freekiosk.Status `json:"status"`
	Online      bool              `json:"online"`
	LastSuccess *time.Time        `json:"last_success"`
	LastAttempt *time.Time        `json:"last_attempt"`
	Error       *freekiosk.Error  `json:"error,omitempty"`
}
type device struct {
	id, name string
	client   *freekiosk.Client
	mu       sync.Mutex
	fetching bool
	snapshot Snapshot
}
type card struct {
	ID, Name string
	Snapshot Snapshot
}
type Server struct {
	devices   map[string]*device
	templates *template.Template
	renderer  *bf.Renderer
}

func New(cfg *config.Config, timeout time.Duration) (http.Handler, error) {
	t, err := template.New("").Funcs(bf.FuncMap()).ParseFS(assets.Files, "templates/*.gohtml", "generated/*.tmpl")
	if err != nil {
		return nil, err
	}
	s := &Server{devices: make(map[string]*device), templates: t, renderer: bf.NewRenderer(t, nil)}
	for id, dev := range cfg.Devices {
		c, err := freekiosk.New(dev.URL, dev.APIKey, timeout)
		if err != nil {
			return nil, err
		}
		s.devices[id] = &device{id: id, name: dev.Name, client: c}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.page)
	mux.HandleFunc("GET /devices/{id}", s.page)
	mux.HandleFunc("GET /api/devices", func(w http.ResponseWriter, _ *http.Request) {
		type info struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		list := []info{}
		for _, c := range s.cards() {
			list = append(list, info{c.ID, c.Name})
		}
		writeJSON(w, http.StatusOK, list)
	})
	mux.HandleFunc("GET /api/devices/{id}/status", s.status)
	mux.HandleFunc("GET /api/devices/{id}/screenshot", s.screenshot)
	mux.HandleFunc("POST /api/devices/{id}/{action}", s.command)
	static, err := fs.Sub(assets.Files, "static")
	if err != nil {
		return nil, err
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	protected := http.NewCrossOriginProtection().Handler(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' blob:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "Cross-site request denied", http.StatusForbidden)
			return
		}
		protected.ServeHTTP(w, r)
	}), nil
}
func (d *device) current() Snapshot { d.mu.Lock(); defer d.mu.Unlock(); return d.snapshot }
func (s *Server) cards() []card {
	list := make([]card, 0, len(s.devices))
	for id, d := range s.devices {
		list = append(list, card{id, d.name, d.current()})
	}
	slices.SortFunc(list, func(a, b card) int { return strings.Compare(a.ID, b.ID) })
	return list
}
func (s *Server) lookup(w http.ResponseWriter, r *http.Request) *device {
	d := s.devices[r.PathValue("id")]
	if d == nil {
		writeJSON(w, http.StatusNotFound, &freekiosk.Error{Kind: "unknown_device", Message: "Unknown device"})
	}
	return d
}
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	var b bytes.Buffer
	if err := s.templates.ExecuteTemplate(&b, name, data); err != nil {
		http.Error(w, "Could not render page", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b.Bytes())
}
func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	lang := language(r)
	http.SetCookie(w, &http.Cookie{Name: "language", Value: lang, Path: "/", MaxAge: 31536000, HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode})
	input := views.PageInput{Text: translations(lang), Title: translate(lang, "Devices"), Cards: []views.PageCardsItem{}, DeviceCards: []views.DeviceCardInput{}}
	for _, c := range s.cards() {
		input.Cards = append(input.Cards, views.PageCardsItem{ID: c.ID, Name: c.Name})
		input.DeviceCards = append(input.DeviceCards, views.DeviceCardInput{ID: c.ID, Name: c.Name, Text: input.Text})
	}
	if r.PathValue("id") != "" {
		d := s.lookup(w, r)
		if d == nil {
			return
		}
		input.Title = d.name
		input.DetailID = d.id
		input.Cards = []views.PageCardsItem{{ID: d.id, Name: d.name}}
		input.DeviceCards = []views.DeviceCardInput{{ID: d.id, Name: d.name, Text: input.Text, Controls: true}}
	}
	props := views.NewPageProps(input)
	scripts := bf.NewScriptCollector()
	content := s.renderer.RenderFragment(bf.RenderOptions{ComponentName: "Page", Props: &props}, scripts, bf.NewPortalCollector())
	s.render(w, "layout", struct {
		Title, Language  string
		Content, Scripts template.HTML
	}{input.Title, lang, content, bf.BfScripts(scripts)})
}
func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	d := s.lookup(w, r)
	if d == nil {
		return
	}
	d.mu.Lock()
	// Coalesce concurrent browser polls; no scheduler and no lock during network I/O.
	fetch := !d.fetching && (d.snapshot.LastAttempt == nil || time.Since(*d.snapshot.LastAttempt) > 3*time.Second)
	if fetch {
		d.fetching = true
	}
	d.mu.Unlock()
	if fetch {
		state, err := d.client.Status(r.Context())
		now := time.Now()
		d.mu.Lock()
		d.fetching = false
		d.snapshot.LastAttempt = &now
		if err != nil {
			d.snapshot.Online = false
			d.snapshot.Error = apiError(err)
		} else {
			d.snapshot.Status = state
			d.snapshot.Online = true
			d.snapshot.LastSuccess = &now
			d.snapshot.Error = nil
		}
		d.mu.Unlock()
	}
	snapshot := d.current()
	writeJSON(w, http.StatusOK, struct {
		Snapshot
		View statusView `json:"view"`
	}{snapshot, statusViewFor(snapshot, language(r))})
}
func (s *Server) command(w http.ResponseWriter, r *http.Request) {
	d := s.lookup(w, r)
	if d == nil {
		return
	}
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		writeJSON(w, 415, freekiosk.Invalid("Expected application/json"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var p freekiosk.Command
	if dec.Decode(&p) != nil || dec.Decode(new(any)) != io.EOF {
		writeJSON(w, 400, freekiosk.Invalid("Invalid command JSON"))
		return
	}
	action := r.PathValue("action")
	if err := d.client.Execute(r.Context(), action, p); err != nil {
		code := 502
		if apiError(err).Kind == "validation" {
			code = 400
		}
		writeJSON(w, code, apiError(err))
		return
	}
	d.mu.Lock()
	d.snapshot.LastAttempt = nil
	d.mu.Unlock()
	writeJSON(w, 200, map[string]string{"message": fmt.Sprintf("%s succeeded", action)})
}
func (s *Server) screenshot(w http.ResponseWriter, r *http.Request) {
	d := s.lookup(w, r)
	if d == nil {
		return
	}
	data, err := d.client.Screenshot(r.Context())
	if err != nil {
		writeJSON(w, 502, apiError(err))
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", `inline; filename="screenshot.png"`)
	_, _ = w.Write(data)
}
func apiError(err error) *freekiosk.Error {
	var e *freekiosk.Error
	if errors.As(err, &e) {
		return e
	}
	return &freekiosk.Error{Kind: "internal", Message: "Request failed"}
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	if err, ok := v.(error); ok {
		v = apiError(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
