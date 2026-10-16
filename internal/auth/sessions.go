package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SessionTTL is how long a sign-in lasts (docs/adr/0004).
const SessionTTL = 7 * 24 * time.Hour

// Sessions keeps who is signed in on which browser: a random token in a cookie, its SHA-256 in the
// sessions table, tied to a member. Removing the member removes their sessions.
type Sessions struct {
	db     *gorm.DB
	cookie cookieKind
}

// NewSessions stores sessions in db. publicURL is where the app is served: an https one makes the
// cookies Secure.
func NewSessions(db *gorm.DB, publicURL string) *Sessions {
	return &Sessions{db: db, cookie: cookieKindFor(publicURL)}
}

// Start signs this browser in as the member, and clears out expired sessions while at it.
func (s *Sessions) Start(c *gin.Context, memberID string) error {
	token := randomBytes()
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM sessions WHERE expires_at < now()").Error; err != nil {
			return err
		}
		return tx.Exec("INSERT INTO sessions (token_hash, member_id, expires_at) VALUES (decode(?, 'hex'), ?, ?)",
			hashToken(token), memberID, time.Now().Add(SessionTTL)).Error
	})
	if err != nil {
		return err
	}
	http.SetCookie(c.Writer, s.cookie.new(sessionCookie, base64.RawURLEncoding.EncodeToString(token), SessionTTL))
	return nil
}

// End signs this browser out. It's fine to call without a session.
func (s *Sessions) End(c *gin.Context) error {
	http.SetCookie(c.Writer, s.cookie.clear(sessionCookie))
	token, ok := s.token(c.Request)
	if !ok {
		return nil
	}
	return s.db.WithContext(c.Request.Context()).Exec("DELETE FROM sessions WHERE token_hash = decode(?, 'hex')", hashToken(token)).Error
}

// Middleware signs the request in from its session cookie, or answers 401.
func (s *Sessions) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := s.lookup(c.Request)
		if errors.Is(err, errNoSession) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sign-in required"})
			return
		}
		if err != nil {
			_ = c.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		c.Set(userKey, user)
		c.Next()
	}
}

var errNoSession = errors.New("no session")

// UserOf is who the request's session signs in, for pages a browser opens directly rather than the API.
// ok is false without a valid session.
func (s *Sessions) UserOf(r *http.Request) (u User, ok bool, err error) {
	u, err = s.lookup(r)
	if errors.Is(err, errNoSession) {
		return User{}, false, nil
	}
	return u, err == nil, err
}

func (s *Sessions) lookup(r *http.Request) (User, error) {
	token, ok := s.token(r)
	if !ok {
		return User{}, errNoSession
	}
	var row struct{ ID, Email, Name string }
	err := s.db.WithContext(r.Context()).Raw(`
		SELECT m.id, m.email, m.name FROM sessions s JOIN members m ON m.id = s.member_id
		WHERE s.token_hash = decode(?, 'hex') AND s.expires_at > now()`, hashToken(token)).Scan(&row).Error
	if err != nil {
		return User{}, err
	}
	if row.ID == "" {
		return User{}, errNoSession
	}
	return User{Subject: row.ID, Email: row.Email, Name: row.Name}, nil
}

func (s *Sessions) token(r *http.Request) ([]byte, bool) {
	c, err := r.Cookie(s.cookie.name(sessionCookie))
	if err != nil {
		return nil, false
	}
	token, err := base64.RawURLEncoding.DecodeString(c.Value)
	return token, err == nil && len(token) == 32
}

// hashToken is the token's SHA-256, in hex for Postgres to decode into bytea.
func hashToken(token []byte) string {
	sum := sha256.Sum256(token)
	return hex.EncodeToString(sum[:])
}

const (
	sessionCookie = "crm_session"
	flowCookie    = "crm_sign_in"
)

// cookieKind makes this app's cookies: HttpOnly and SameSite=Lax, so other sites' pages can't send them
// with a POST, and on https Secure with the __Host- prefix, so they're only ever set by this origin.
type cookieKind struct{ secure bool }

func cookieKindFor(url string) cookieKind {
	return cookieKind{secure: strings.HasPrefix(url, "https://")}
}

func (k cookieKind) name(base string) string {
	if k.secure {
		return "__Host-" + base
	}
	return base
}

func (k cookieKind) new(base, value string, ttl time.Duration) *http.Cookie {
	return &http.Cookie{
		Name: k.name(base), Value: value, Path: "/", MaxAge: int(ttl.Seconds()),
		HttpOnly: true, Secure: k.secure, SameSite: http.SameSiteLaxMode,
	}
}

func (k cookieKind) clear(base string) *http.Cookie {
	c := k.new(base, "", 0)
	c.MaxAge = -1
	return c
}
