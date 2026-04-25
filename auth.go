package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ─── Rate limiting login ─────────────────────────────────────────────────────

const (
	loginMaxAttempts = 5
	loginBlockDur    = 15 * time.Minute
)

type loginEntry struct {
	attempts  int
	blockedAt time.Time
}

var loginAttempts sync.Map // map[string]*loginEntry

func loginIP(r *http.Request) string {
	ip := r.RemoteAddr
	if i := len(ip) - 1; i >= 0 {
		for i >= 0 && ip[i] != ':' {
			i--
		}
		if i > 0 {
			ip = ip[:i]
		}
	}
	return ip
}

func isLoginBlocked(ip string) bool {
	v, ok := loginAttempts.Load(ip)
	if !ok {
		return false
	}
	e := v.(*loginEntry)
	if !e.blockedAt.IsZero() {
		if time.Since(e.blockedAt) < loginBlockDur {
			return true
		}
		loginAttempts.Delete(ip)
	}
	return false
}

func recordLoginFailure(ip string) {
	v, _ := loginAttempts.LoadOrStore(ip, &loginEntry{})
	e := v.(*loginEntry)
	e.attempts++
	if e.attempts >= loginMaxAttempts {
		e.blockedAt = time.Now()
	}
}

func resetLoginAttempts(ip string) {
	loginAttempts.Delete(ip)
}

// ─── Session ─────────────────────────────────────────────────────────────────

type Session struct {
	UserID    int64
	Username  string
	Role      string
	ExpireAt  time.Time
	CSRFToken string
}

var (
	store sync.Map // map[string]*Session
)

const (
	cookieName      = "gorage_sid"
	sessionDuration = 12 * time.Hour
)

// ─── Bcrypt ──────────────────────────────────────────────────────────────────

func hashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func checkPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ─── Gestion des sessions ────────────────────────────────────────────────────

func newSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func createSession(userID int64, username, role string) (string, error) {
	sid, err := newSessionID()
	if err != nil {
		return "", err
	}
	csrf, err := newSessionID()
	if err != nil {
		return "", err
	}
	store.Store(sid, &Session{
		UserID:    userID,
		Username:  username,
		Role:      role,
		ExpireAt:  time.Now().Add(sessionDuration),
		CSRFToken: csrf,
	})
	return sid, nil
}

func getSession(r *http.Request) *Session {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	v, ok := store.Load(c.Value)
	if !ok {
		return nil
	}
	s := v.(*Session)
	if time.Now().After(s.ExpireAt) {
		store.Delete(c.Value)
		return nil
	}
	return s
}

func destroySession(r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		store.Delete(c.Value)
	}
}

// ─── Cookies ─────────────────────────────────────────────────────────────────

func setSessionCookie(w http.ResponseWriter, sid string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   int(sessionDuration.Seconds()),
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
	})
}

// ─── Middleware d'authentification ───────────────────────────────────────────

func checkCSRF(r *http.Request, s *Session) bool {
	if r.Method != http.MethodPost {
		return true
	}
	return r.FormValue("csrf_token") == s.CSRFToken
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := getSession(r)
		if s == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		if !checkCSRF(r, s) {
			http.Error(w, "Requête invalide", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func adminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := getSession(r)
		if s == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		if s.Role != "admin" {
			http.Redirect(w, r, "/?err=Accès+réservé+aux+administrateurs", http.StatusFound)
			return
		}
		if !checkCSRF(r, s) {
			http.Error(w, "Requête invalide", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
