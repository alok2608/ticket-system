package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTestServer builds a server backed by a throwaway SQLite file, so the tests
// exercise the real handlers, middleware and queries.
func newTestServer(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := openDB(filepath.Join(t.TempDir(), "test.db"))
	t.Cleanup(func() { db.Close() })

	srv := &server{db: db, jwtSecret: []byte("test-secret")}
	return srv.routes()
}

// do performs a request against the router and returns the recorder.
func do(t *testing.T, r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// registerUser creates a user and returns its token.
func registerUser(t *testing.T, r *gin.Engine, email string) string {
	t.Helper()

	w := do(t, r, http.MethodPost, "/auth/register", "", gin.H{"email": email, "password": "secret123"})
	if w.Code != http.StatusCreated {
		t.Fatalf("register %s: got %d, want 201 (%s)", email, w.Code, w.Body)
	}

	var res struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if res.Token == "" {
		t.Fatal("register returned an empty token")
	}
	return res.Token
}

// createTicket creates a ticket for the token's owner and returns its id.
func createTicket(t *testing.T, r *gin.Engine, token, title string) int64 {
	t.Helper()

	w := do(t, r, http.MethodPost, "/tickets", token, gin.H{"title": title})
	if w.Code != http.StatusCreated {
		t.Fatalf("create ticket: got %d, want 201 (%s)", w.Code, w.Body)
	}

	var ticket Ticket
	if err := json.Unmarshal(w.Body.Bytes(), &ticket); err != nil {
		t.Fatalf("decode ticket: %v", err)
	}
	if ticket.Status != StatusOpen {
		t.Errorf("new ticket status = %q, want %q", ticket.Status, StatusOpen)
	}
	return ticket.ID
}

func TestHealth(t *testing.T) {
	r := newTestServer(t)

	w := do(t, r, http.MethodGet, "/health", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	if got := w.Body.String(); got != `{"status":"ok"}` {
		t.Errorf("body = %s, want {\"status\":\"ok\"}", got)
	}
}

func TestRegisterAndLogin(t *testing.T) {
	r := newTestServer(t)
	registerUser(t, r, "alice@example.com")

	t.Run("duplicate email is rejected", func(t *testing.T) {
		w := do(t, r, http.MethodPost, "/auth/register", "", gin.H{"email": "alice@example.com", "password": "secret123"})
		if w.Code != http.StatusConflict {
			t.Errorf("got %d, want 409", w.Code)
		}
	})

	t.Run("missing password is rejected", func(t *testing.T) {
		w := do(t, r, http.MethodPost, "/auth/register", "", gin.H{"email": "bob@example.com"})
		if w.Code != http.StatusBadRequest {
			t.Errorf("got %d, want 400", w.Code)
		}
	})

	t.Run("login succeeds", func(t *testing.T) {
		w := do(t, r, http.MethodPost, "/auth/login", "", gin.H{"email": "alice@example.com", "password": "secret123"})
		if w.Code != http.StatusOK {
			t.Errorf("got %d, want 200", w.Code)
		}
	})

	t.Run("wrong password is rejected", func(t *testing.T) {
		w := do(t, r, http.MethodPost, "/auth/login", "", gin.H{"email": "alice@example.com", "password": "wrong"})
		if w.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", w.Code)
		}
	})
}

// TestPasswordIsHashed checks the stored credential is not the password itself.
func TestPasswordIsHashed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openDB(filepath.Join(t.TempDir(), "test.db"))
	defer db.Close()

	srv := &server{db: db, jwtSecret: []byte("test-secret")}
	r := srv.routes()
	registerUser(t, r, "alice@example.com")

	var hash string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE email = ?`, "alice@example.com").Scan(&hash); err != nil {
		t.Fatalf("read stored hash: %v", err)
	}
	if hash == "secret123" {
		t.Fatal("password was stored in plain text")
	}
	if len(hash) < 20 {
		t.Errorf("stored value %q does not look like a bcrypt hash", hash)
	}
}

func TestProtectedRoutesRequireBearerToken(t *testing.T) {
	r := newTestServer(t)
	token := registerUser(t, r, "alice@example.com")
	id := createTicket(t, r, token, "Printer broken")

	requests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"create", http.MethodPost, "/tickets", gin.H{"title": "x"}},
		{"list", http.MethodGet, "/tickets", nil},
		{"get", http.MethodGet, "/tickets/1", nil},
		{"patch status", http.MethodPatch, "/tickets/1/status", gin.H{"status": "closed"}},
	}
	for _, tc := range requests {
		t.Run(tc.name+" without a token", func(t *testing.T) {
			if w := do(t, r, tc.method, tc.path, "", tc.body); w.Code != http.StatusUnauthorized {
				t.Errorf("got %d, want 401", w.Code)
			}
		})
	}

	t.Run("invalid token", func(t *testing.T) {
		if w := do(t, r, http.MethodGet, "/tickets", "not-a-jwt", nil); w.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", w.Code)
		}
	})

	t.Run("a valid token still works", func(t *testing.T) {
		if w := do(t, r, http.MethodGet, "/tickets/1", token, nil); w.Code != http.StatusOK {
			t.Errorf("got %d, want 200 for own ticket %d", w.Code, id)
		}
	})
}

// TestOwnership is the rule the brief cares about most: a user must only see and
// change tickets they created.
func TestOwnership(t *testing.T) {
	r := newTestServer(t)

	alice := registerUser(t, r, "alice@example.com")
	bob := registerUser(t, r, "bob@example.com")
	ticketID := createTicket(t, r, alice, "Alice's ticket")

	t.Run("bob's list excludes alice's tickets", func(t *testing.T) {
		w := do(t, r, http.MethodGet, "/tickets", bob, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", w.Code)
		}
		var tickets []Ticket
		if err := json.Unmarshal(w.Body.Bytes(), &tickets); err != nil {
			t.Fatalf("decode list: %v", err)
		}
		if len(tickets) != 0 {
			t.Errorf("bob sees %d tickets, want 0", len(tickets))
		}
	})

	t.Run("bob cannot view alice's ticket", func(t *testing.T) {
		w := do(t, r, http.MethodGet, "/tickets/1", bob, nil)
		if w.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", w.Code)
		}
	})

	t.Run("bob cannot update alice's ticket", func(t *testing.T) {
		w := do(t, r, http.MethodPatch, "/tickets/1/status", bob, gin.H{"status": StatusClosed})
		if w.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", w.Code)
		}

		// The ticket must be untouched by the attempt.
		w = do(t, r, http.MethodGet, "/tickets/1", alice, nil)
		var ticket Ticket
		if err := json.Unmarshal(w.Body.Bytes(), &ticket); err != nil {
			t.Fatalf("decode ticket: %v", err)
		}
		if ticket.Status != StatusOpen {
			t.Errorf("ticket %d status = %q after bob's attempt, want %q", ticketID, ticket.Status, StatusOpen)
		}
	})
}

func TestStatusFlowOverHTTP(t *testing.T) {
	r := newTestServer(t)
	token := registerUser(t, r, "alice@example.com")
	createTicket(t, r, token, "Printer broken")

	patch := func(status string) *httptest.ResponseRecorder {
		return do(t, r, http.MethodPatch, "/tickets/1/status", token, gin.H{"status": status})
	}

	if w := patch(StatusInProgress); w.Code != http.StatusOK {
		t.Fatalf("open -> in_progress: got %d, want 200", w.Code)
	}
	if w := patch(StatusOpen); w.Code != http.StatusBadRequest {
		t.Errorf("in_progress -> open: got %d, want 400", w.Code)
	}
	if w := patch(StatusClosed); w.Code != http.StatusOK {
		t.Fatalf("in_progress -> closed: got %d, want 200", w.Code)
	}

	for _, status := range []string{StatusOpen, StatusInProgress} {
		w := patch(status)
		if w.Code != http.StatusBadRequest {
			t.Errorf("closed -> %s: got %d, want 400", status, w.Code)
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("closed ticket cannot be reopened")) {
			t.Errorf("closed -> %s: body = %s, want the reopen error", status, w.Body)
		}
	}
}

func TestTicketValidation(t *testing.T) {
	r := newTestServer(t)
	token := registerUser(t, r, "alice@example.com")
	createTicket(t, r, token, "Printer broken")

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{"missing title", http.MethodPost, "/tickets", gin.H{"description": "no title"}, http.StatusBadRequest},
		{"blank title", http.MethodPost, "/tickets", gin.H{"title": "   "}, http.StatusBadRequest},
		{"unknown status", http.MethodPatch, "/tickets/1/status", gin.H{"status": "archived"}, http.StatusBadRequest},
		{"unknown ticket", http.MethodGet, "/tickets/4242", nil, http.StatusNotFound},
		{"non-numeric id", http.MethodGet, "/tickets/abc", nil, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if w := do(t, r, tc.method, tc.path, token, tc.body); w.Code != tc.want {
				t.Errorf("got %d, want %d (%s)", w.Code, tc.want, w.Body)
			}
		})
	}
}
