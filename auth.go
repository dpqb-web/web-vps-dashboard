package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 1
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16

	sessionCookie = "session"
)

const grpRoot = "root"
const grpAdmin = "admin"
const grpNOC = "noc"

var validGroups = map[string]bool{grpRoot: true, grpAdmin: true, grpNOC: true}

type ctxKey int

const (
	ctxUser ctxKey = iota
)

func hashPassword(plain string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyPassword(hash, plain string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("format hash tidak dikenali")
	}
	var version int
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, err
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	computed := argon2.IDKey([]byte(plain), salt, time, memory, threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(computed, expected) == 1, nil
}

func validUsername(u string) bool {
	if u == "" || len(u) > 64 {
		return false
	}
	for _, r := range u {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func validPassword(p string) bool {
	if len(p) < 8 {
		return false
	}
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range p {
		if r > 0x7E || r < 0x20 {
			return false
		}
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSymbol = true
		}
	}
	return hasUpper && hasLower && hasDigit && hasSymbol && utf8.RuneCountInString(p) == len(p)
}

func writeAPIError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func currentUser(r *http.Request) *User {
	u, _ := r.Context().Value(ctxUser).(*User)
	return u
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeAPIError(w, http.StatusUnauthorized, "belum masuk")
		return
	}
	http.Redirect(w, r, "/signin", http.StatusSeeOther)
}

func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			unauthorized(w, r)
			return
		}
		sess, err := getSession(c.Value)
		if err != nil {
			deleteSession(c.Value)
			unauthorized(w, r)
			return
		}
		u, err := getUserByID(sess.UserID)
		if err != nil {
			deleteSession(c.Value)
			unauthorized(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxUser, u)))
	})
}

func requireGroup(groups ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := currentUser(r)
			if u == nil {
				unauthorized(w, r)
				return
			}
			for _, g := range groups {
				if u.Grp == g {
					next.ServeHTTP(w, r)
					return
				}
			}
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
				return
			}
			http.Error(w, "403 Forbidden", http.StatusForbidden)
		})
	}
}

func canManageServer(r *http.Request, node string, vmid int64) bool {
	u := currentUser(r)
	if u == nil {
		return false
	}
	if u.Grp == grpRoot {
		return true
	}
	if u.Grp == grpAdmin {
		ok, err := isHolder(node, vmid, u.ID)
		return err == nil && ok
	}
	return false
}

func canViewServer(r *http.Request, node string, vmid int64) bool {
	u := currentUser(r)
	if u == nil {
		return false
	}
	if u.Grp == grpRoot || u.Grp == grpNOC {
		return true
	}
	ok, err := isHolder(node, vmid, u.ID)
	return err == nil && ok
}

func issueSession(w http.ResponseWriter, u *User) {
	sess, err := createSession(u.ID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "gagal membuat sesi")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sess.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
	writeJSON(w, toUserResponse(u))
}

type userResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Fullname string `json:"fullname"`
	Group    string `json:"group"`
	TOTP     bool   `json:"totp"`
	Created  string `json:"created"`
}

func toUserResponse(u *User) userResponse {
	return userResponse{
		ID:       u.ID,
		Username: u.Username,
		Fullname: u.Fullname,
		Group:    u.Grp,
		TOTP:     u.TOTPSecret != "",
		Created:  u.Created,
	}
}