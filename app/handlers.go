package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"time"
)

//go:embed templates/*.html
var templateFiles embed.FS

const s3Timeout = 10 * time.Second

type App struct {
	cfg      Config
	store    ObjectStore
	tmpl     *template.Template
	log      *slog.Logger
	hostname string
	exit     func(int)
	now      func() time.Time
	newID    func() (string, error)
}

type formView struct {
	AppName string
	Version string
	Error   string
	Name    string
	Email   string
	Message string
}

type successView struct {
	AppName string
	Version string
	Name    string
	Key     string
	Bucket  string
	Region  string
}

type errorView struct {
	AppName string
	Version string
}

func loadTemplates() (*template.Template, error) {
	return template.ParseFS(templateFiles, "templates/*.html")
}

func newApp(cfg Config, store ObjectStore, tmpl *template.Template, logger *slog.Logger, hostname string) *App {
	return &App{
		cfg:      cfg,
		store:    store,
		tmpl:     tmpl,
		log:      logger,
		hostname: hostname,
		exit:     osExit,
		now:      time.Now,
		newID:    newID,
	}
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", a.handleIndex)
	mux.HandleFunc("POST /submit", a.handleSubmit)
	mux.HandleFunc("GET /health", a.handleHealth)
	mux.HandleFunc("GET /ready", a.handleReady)
	mux.HandleFunc("GET /info", a.handleInfo)
	mux.HandleFunc("GET /crash", a.handleCrash)
	return mux
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	a.render(w, http.StatusOK, "index.html", formView{
		AppName: a.cfg.AppName,
		Version: a.cfg.AppVersion,
	})
}

func (a *App) handleSubmit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	if err := r.ParseForm(); err != nil {
		a.render(w, http.StatusBadRequest, "index.html", formView{
			AppName: a.cfg.AppName,
			Version: a.cfg.AppVersion,
			Error:   "The form could not be read. Please try again.",
		})
		return
	}

	name, email, message, err := validateFields(
		r.PostFormValue("name"),
		r.PostFormValue("email"),
		r.PostFormValue("message"),
	)
	if err != nil {
		a.render(w, http.StatusBadRequest, "index.html", formView{
			AppName: a.cfg.AppName,
			Version: a.cfg.AppVersion,
			Error:   err.Error(),
			Name:    r.PostFormValue("name"),
			Email:   r.PostFormValue("email"),
			Message: r.PostFormValue("message"),
		})
		return
	}

	id, err := a.newID()
	if err != nil {
		a.log.Error("generate submission id", "err", err)
		a.renderStorageError(w, http.StatusInternalServerError)
		return
	}

	now := a.now().UTC()
	sub := buildSubmission(name, email, message, id, now)
	body, err := marshalSubmission(sub)
	if err != nil {
		a.log.Error("encode submission", "err", err)
		a.renderStorageError(w, http.StatusInternalServerError)
		return
	}

	if a.store == nil {
		a.log.Error("store submission", "err", "storage is not configured")
		a.renderStorageError(w, http.StatusServiceUnavailable)
		return
	}

	key := objectKey(now, id)
	ctx, cancel := context.WithTimeout(r.Context(), s3Timeout)
	defer cancel()
	if err := a.store.PutJSON(ctx, key, body); err != nil {
		a.log.Error("store submission", "key", key, "err", err)
		a.renderStorageError(w, http.StatusBadGateway)
		return
	}

	a.log.Info("stored submission", "key", key)
	a.render(w, http.StatusOK, "success.html", successView{
		AppName: a.cfg.AppName,
		Version: a.cfg.AppVersion,
		Name:    name,
		Key:     key,
		Bucket:  a.cfg.S3Bucket,
		Region:  a.cfg.AWSRegion,
	})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeText(w, http.StatusOK, "ok\n")
}

func (a *App) handleReady(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeText(w, http.StatusServiceUnavailable, "not ready\n")
		return
	}
	writeText(w, http.StatusOK, "ready\n")
}

func (a *App) handleInfo(w http.ResponseWriter, r *http.Request) {
	body := fmt.Sprintf(
		"name: %s\nversion: %s\nhostname: %s\nstorage: s3\nregion: %s\nbucket: %s\n",
		a.cfg.AppName,
		a.cfg.AppVersion,
		a.hostname,
		a.cfg.AWSRegion,
		a.cfg.S3Bucket,
	)
	writeText(w, http.StatusOK, body)
}

// handleCrash ends the process so Kubernetes can demonstrate a restart.
// Tests replace App.exit so the test process itself keeps running.
func (a *App) handleCrash(w http.ResponseWriter, r *http.Request) {
	a.log.Info("crash requested; exiting", "code", 1)
	a.exit(1)
}

func (a *App) renderStorageError(w http.ResponseWriter, status int) {
	a.render(w, status, "error.html", errorView{
		AppName: a.cfg.AppName,
		Version: a.cfg.AppVersion,
	})
}

func (a *App) render(w http.ResponseWriter, status int, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := a.tmpl.ExecuteTemplate(w, name, data); err != nil {
		a.log.Error("render template", "template", name, "err", err)
	}
}

func writeText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}
