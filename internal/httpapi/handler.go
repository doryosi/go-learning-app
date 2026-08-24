package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type contextKey string

const userIDKey contextKey = "user_id"

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}
type userRecord struct {
	User
	Salt string
	Hash string
}
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Job struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}
type File struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// API stores milestone-one data in memory. Its mutex is needed because net/http
// runs handlers concurrently. Later milestones can replace the maps with DBs.
type API struct {
	mu             sync.RWMutex
	users          map[string]userRecord
	emails, tokens map[string]string
	projects       map[string]Project
	jobs           map[string]Job
	files          map[string]File
	logger         *slog.Logger
}

func NewHandler() http.Handler { return NewHandlerWithLogger(slog.Default()) }

func NewHandlerWithLogger(logger *slog.Logger) http.Handler {
	a := &API{users: map[string]userRecord{}, emails: map[string]string{}, tokens: map[string]string{}, projects: map[string]Project{}, jobs: map[string]Job{}, files: map[string]File{}, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ready"}) })
	mux.HandleFunc("POST /users", a.createUser)
	mux.HandleFunc("POST /login", a.login)
	mux.Handle("POST /projects", a.auth(http.HandlerFunc(a.createProject)))
	mux.Handle("GET /projects", a.auth(http.HandlerFunc(a.listProjects)))
	mux.Handle("POST /jobs", a.auth(http.HandlerFunc(a.createJob)))
	mux.Handle("GET /jobs", a.auth(http.HandlerFunc(a.listJobs)))
	mux.Handle("GET /jobs/{id}", a.auth(http.HandlerFunc(a.getJob)))
	mux.Handle("POST /files", a.auth(http.HandlerFunc(a.createFile)))
	mux.Handle("GET /files/{id}", a.auth(http.HandlerFunc(a.getFile)))
	return a.logRequests(mux)
}

func (a *API) createUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !strings.Contains(in.Email, "@") || len(in.Password) < 8 {
		writeError(w, 400, "valid email and 8-character password required")
		return
	}
	id, salt, err := randomID(), randomID(), error(nil)
	if id == "" || salt == "" {
		err = errors.New("random ID unavailable")
	}
	if err != nil {
		writeError(w, 500, "could not create user")
		return
	}
	record := userRecord{User: User{ID: id, Email: in.Email}, Salt: salt, Hash: hashPassword(salt, in.Password)}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.emails[in.Email]; exists {
		writeError(w, 409, "email already registered")
		return
	}
	a.users[id], a.emails[in.Email] = record, id
	writeJSON(w, 201, record.User)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	a.mu.RLock()
	id := a.emails[strings.ToLower(strings.TrimSpace(in.Email))]
	user, ok := a.users[id]
	a.mu.RUnlock()
	got := hashPassword(user.Salt, in.Password)
	if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(user.Hash)) != 1 {
		writeError(w, 401, "invalid credentials")
		return
	}
	token := randomID()
	if token == "" {
		writeError(w, 500, "could not create session")
		return
	}
	a.mu.Lock()
	a.tokens[token] = id
	a.mu.Unlock()
	writeJSON(w, 200, map[string]string{"token": token})
}

func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if decodeJSON(w, r, &in) != nil || strings.TrimSpace(in.Name) == "" {
		writeError(w, 400, "name is required")
		return
	}
	p := Project{ID: randomID(), Name: strings.TrimSpace(in.Name), OwnerID: currentUser(r), CreatedAt: time.Now().UTC()}
	a.mu.Lock()
	a.projects[p.ID] = p
	a.mu.Unlock()
	writeJSON(w, 201, p)
}
func (a *API) listProjects(w http.ResponseWriter, r *http.Request) {
	items := []Project{}
	a.mu.RLock()
	for _, v := range a.projects {
		if v.OwnerID == currentUser(r) {
			items = append(items, v)
		}
	}
	a.mu.RUnlock()
	writeJSON(w, 200, map[string]any{"projects": items})
}
func (a *API) createJob(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Type string `json:"type"`
	}
	if decodeJSON(w, r, &in) != nil || strings.TrimSpace(in.Type) == "" {
		writeError(w, 400, "type is required")
		return
	}
	j := Job{ID: randomID(), Type: strings.TrimSpace(in.Type), Status: "queued", OwnerID: currentUser(r), CreatedAt: time.Now().UTC()}
	a.mu.Lock()
	a.jobs[j.ID] = j
	a.mu.Unlock()
	writeJSON(w, 202, j)
}
func (a *API) listJobs(w http.ResponseWriter, r *http.Request) {
	items := []Job{}
	a.mu.RLock()
	for _, v := range a.jobs {
		if v.OwnerID == currentUser(r) {
			items = append(items, v)
		}
	}
	a.mu.RUnlock()
	writeJSON(w, 200, map[string]any{"jobs": items})
}
func (a *API) getJob(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	v, ok := a.jobs[r.PathValue("id")]
	a.mu.RUnlock()
	if !ok || v.OwnerID != currentUser(r) {
		writeError(w, 404, "job not found")
		return
	}
	writeJSON(w, 200, v)
}
func (a *API) createFile(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if decodeJSON(w, r, &in) != nil || strings.TrimSpace(in.Name) == "" {
		writeError(w, 400, "name is required")
		return
	}
	f := File{ID: randomID(), Name: strings.TrimSpace(in.Name), OwnerID: currentUser(r), CreatedAt: time.Now().UTC()}
	a.mu.Lock()
	a.files[f.ID] = f
	a.mu.Unlock()
	writeJSON(w, 201, f)
}
func (a *API) getFile(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	v, ok := a.files[r.PathValue("id")]
	a.mu.RUnlock()
	if !ok || v.OwnerID != currentUser(r) {
		writeError(w, 404, "file not found")
		return
	}
	writeJSON(w, 200, v)
}

func (a *API) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		a.mu.RLock()
		id, ok := a.tokens[token]
		a.mu.RUnlock()
		if token == "" || !ok {
			writeError(w, 401, "valid bearer token required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, id)))
	})
}
func (a *API) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		a.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "duration", time.Since(started))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func currentUser(r *http.Request) string { id, _ := r.Context().Value(userIDKey).(string); return id }

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(dst) != nil {
		return errors.New("invalid JSON body")
	}
	if d.Decode(&struct{}{}) == nil {
		return errors.New("body must contain one JSON object")
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
func hashPassword(salt, password string) string {
	sum := sha256.Sum256([]byte(salt + password))
	return hex.EncodeToString(sum[:])
}
