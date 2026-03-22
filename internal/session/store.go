package session

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

const sessionDuration = 30 * 24 * time.Hour // 30 days

type SQLiteStore struct {
	db            *sql.DB
	codecs        []securecookie.Codec
	options       *sessions.Options
	encryptionKey []byte
}

// NewSQLiteStore creates a session store. Set secure=true in production (behind TLS).
func NewSQLiteStore(db *sql.DB, signingKey, encryptionKey []byte, secure bool) *SQLiteStore {
	return &SQLiteStore{
		db:            db,
		codecs:        securecookie.CodecsFromPairs(signingKey),
		encryptionKey: encryptionKey,
		options: &sessions.Options{
			Path:     "/",
			MaxAge:   int(sessionDuration.Seconds()),
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		},
	}
}

func (s *SQLiteStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

func (s *SQLiteStore) New(r *http.Request, name string) (*sessions.Session, error) {
	sess := sessions.NewSession(s, name)
	sess.Options = &sessions.Options{
		Path:     s.options.Path,
		MaxAge:   s.options.MaxAge,
		HttpOnly: s.options.HttpOnly,
		Secure:   s.options.Secure,
		SameSite: s.options.SameSite,
	}
	sess.IsNew = true

	cookie, err := r.Cookie(name)
	if err != nil {
		return sess, nil
	}

	var sessionID string
	if err := securecookie.DecodeMulti(name, cookie.Value, &sessionID, s.codecs...); err != nil {
		return sess, nil
	}

	row := s.db.QueryRow(
		"SELECT email, oauth_token, expires_at FROM sessions WHERE id = ?",
		sessionID,
	)
	var email string
	var tokenBlob []byte
	var expiresAt time.Time
	if err := row.Scan(&email, &tokenBlob, &expiresAt); err != nil {
		return sess, nil
	}

	if time.Now().After(expiresAt) {
		s.db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
		return sess, nil
	}

	decrypted, err := Decrypt(s.encryptionKey, tokenBlob)
	if err != nil {
		return sess, nil
	}

	sess.ID = sessionID
	sess.Values["email"] = email
	sess.Values["oauth_token"] = decrypted
	sess.IsNew = false
	return sess, nil
}

func (s *SQLiteStore) Save(r *http.Request, w http.ResponseWriter, sess *sessions.Session) error {
	if sess.Options.MaxAge < 0 {
		if sess.ID != "" {
			if _, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", sess.ID); err != nil {
				return fmt.Errorf("delete session: %w", err)
			}
		}
		http.SetCookie(w, sessions.NewCookie(sess.Name(), "", sess.Options))
		return nil
	}

	if sess.ID == "" {
		id, err := generateSessionID()
		if err != nil {
			return fmt.Errorf("generate session id: %w", err)
		}
		sess.ID = id
	}

	email, _ := sess.Values["email"].(string)
	tokenBytes, _ := sess.Values["oauth_token"].([]byte)

	encrypted, err := Encrypt(s.encryptionKey, tokenBytes)
	if err != nil {
		return fmt.Errorf("encrypt token: %w", err)
	}

	expiresAt := time.Now().Add(sessionDuration)

	_, err = s.db.Exec(`
		INSERT INTO sessions (id, email, oauth_token, expires_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			email = excluded.email,
			oauth_token = excluded.oauth_token,
			expires_at = excluded.expires_at
	`, sess.ID, email, encrypted, expiresAt)
	if err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}

	encoded, err := securecookie.EncodeMulti(sess.Name(), sess.ID, s.codecs...)
	if err != nil {
		return fmt.Errorf("encode cookie: %w", err)
	}

	http.SetCookie(w, sessions.NewCookie(sess.Name(), encoded, sess.Options))
	return nil
}

// SessionData holds the deserialized session contents for use in handlers.
type SessionData struct {
	Email      string
	OAuthToken []byte
}

// GetSessionData extracts structured data from a gorilla session.
func GetSessionData(sess *sessions.Session) *SessionData {
	email, _ := sess.Values["email"].(string)
	token, _ := sess.Values["oauth_token"].([]byte)
	if email == "" || token == nil {
		return nil
	}
	return &SessionData{Email: email, OAuthToken: token}
}

// SetOAuthToken serializes and stores an OAuth token in the session values.
func SetOAuthToken(sess *sessions.Session, token interface{}) error {
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	sess.Values["oauth_token"] = data
	return nil
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
