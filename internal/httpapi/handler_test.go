package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "liveness", path: "/healthz", body: "{\"status\":\"ok\"}\n"},
		{name: "readiness", path: "/readyz", body: "{\"status\":\"ready\"}\n"},
	}

	handler := NewHandler()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			if got := response.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if got := response.Body.String(); got != tt.body {
				t.Errorf("body = %q, want %q", got, tt.body)
			}
		})
	}
}

func TestSaaSWorkflow(t *testing.T) {
	handler := NewHandlerWithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	userResponse := request(t, handler, http.MethodPost, "/users", `{"email":"learner@example.com","password":"learning-go"}`, "")
	if userResponse.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, body = %s", userResponse.Code, userResponse.Body.String())
	}

	loginResponse := request(t, handler, http.MethodPost, "/login", `{"email":"learner@example.com","password":"learning-go"}`, "")
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResponse.Code, loginResponse.Body.String())
	}
	var session struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginResponse.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{name: "create project", method: http.MethodPost, path: "/projects", body: `{"name":"learning lab"}`, wantStatus: http.StatusCreated},
		{name: "list projects", method: http.MethodGet, path: "/projects", wantStatus: http.StatusOK},
		{name: "create job", method: http.MethodPost, path: "/jobs", body: `{"type":"report"}`, wantStatus: http.StatusAccepted},
		{name: "list jobs", method: http.MethodGet, path: "/jobs", wantStatus: http.StatusOK},
		{name: "create file metadata", method: http.MethodPost, path: "/files", body: `{"name":"report.pdf"}`, wantStatus: http.StatusCreated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := request(t, handler, tt.method, tt.path, tt.body, session.Token)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, tt.wantStatus, response.Body.String())
			}
		})
	}
}

func TestProtectedRoutesRequireToken(t *testing.T) {
	response := request(t, NewHandlerWithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), http.MethodGet, "/projects", "", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func request(t *testing.T, handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestUnknownRoute(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	response := httptest.NewRecorder()

	NewHandler().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
