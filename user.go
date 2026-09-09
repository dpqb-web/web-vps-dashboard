package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type signinRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type pendingTOTP struct {
	UserID  int64
	Expires time.Time
}

var (
	pendingTOTPs   = map[string]pendingTOTP{}
	pendingTOTPMu  sync.Mutex
)

// POST /api/auth/signin
func handleSignIn(w http.ResponseWriter, r *http.Request) {
	var req signinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}
	if !validUsername(req.Username) || req.Password == "" {
		writeAPIError(w, http.StatusUnauthorized, "username atau kata sandi salah")
		return
	}
	u, err := getUserByUsername(req.Username)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "username atau kata sandi salah")
		return
	}
	ok, err := verifyPassword(u.Password, req.Password)
	if err != nil || !ok {
		writeAPIError(w, http.StatusUnauthorized, "username atau kata sandi salah")
		return
	}
	if u.TOTPSecret != "" {
		token := randomHex(32)
		pendingTOTPMu.Lock()
		pendingTOTPs[token] = pendingTOTP{UserID: u.ID, Expires: time.Now().Add(5 * time.Minute)}
		pendingTOTPMu.Unlock()
		writeJSON(w, map[string]interface{}{
			"totpRequired": true,
			"tempToken":    token,
		})
		return
	}
	issueSession(w, u)
}

// POST /api/auth/signin/totp
func handleSignInTOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TempToken string `json:"tempToken"`
		Code      string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}
	pendingTOTPMu.Lock()
	pend, ok := pendingTOTPs[req.TempToken]
	if ok && time.Now().After(pend.Expires) {
		delete(pendingTOTPs, req.TempToken)
		ok = false
	}
	pendingTOTPMu.Unlock()
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "sesi TOTP tidak valid atau kedaluwarsa")
		return
	}
	u, err := getUserByID(pend.UserID)
	if err != nil || u.TOTPSecret == "" {
		writeAPIError(w, http.StatusUnauthorized, "user tidak ditemukan")
		return
	}
	if !validateTOTP(req.Code, u.TOTPSecret) {
		writeAPIError(w, http.StatusUnauthorized, "kode TOTP salah")
		return
	}
	pendingTOTPMu.Lock()
	delete(pendingTOTPs, req.TempToken)
	pendingTOTPMu.Unlock()
	issueSession(w, u)
}

// POST /api/auth/signout
func handleSignOut(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		deleteSession(c.Value)
		http.SetCookie(w, &http.Cookie{
			Name: sessionCookie, Value: "", Path: "/",
			HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1,
		})
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/auth/me
func handleMe(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeAPIError(w, http.StatusUnauthorized, "belum masuk")
		return
	}
	writeJSON(w, toUserResponse(u))
}

// GET /api/users
func handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := listUsers()
	if err != nil {
		writeError(w, err)
		return
	}
	out := []userResponse{}
	for _, u := range users {
		out = append(out, toUserResponse(u))
	}
	if group := r.URL.Query().Get("group"); group != "" {
		filtered := []userResponse{}
		for _, u := range out {
			if u.Group == group {
				filtered = append(filtered, u)
			}
		}
		out = filtered
	}
	writeJSON(w, out)
}

type createUserRequest struct {
	Username         string `json:"username"`
	Fullname         string `json:"fullname"`
	Group            string `json:"group"`
	Password         string `json:"password"`
	PasswordConfirm  string `json:"passwordConfirm"`
}

// POST /api/users
func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}
	if !validUsername(req.Username) {
		writeAPIError(w, http.StatusBadRequest, "username hanya boleh alfanumerik tanpa simbol")
		return
	}
	if !validGroups[req.Group] || req.Group == grpRoot {
		writeAPIError(w, http.StatusBadRequest, "grup harus admin atau noc")
		return
	}
	if !validPassword(req.Password) {
		writeAPIError(w, http.StatusBadRequest,
			"kata sandi minimal 8 karakter, berisi 1 huruf besar, 1 huruf kecil, 1 angka, dan 1 simbol ASCII")
		return
	}
	if req.Password != req.PasswordConfirm {
		writeAPIError(w, http.StatusBadRequest, "konfirmasi kata sandi tidak cocok")
		return
	}
	if _, err := getUserByUsername(req.Username); err == nil {
		writeAPIError(w, http.StatusConflict, "username sudah dipakai")
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	u, err := createUser(req.Username, req.Fullname, req.Group, hash)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, toUserResponse(u))
}

type updateUserRequest struct {
	Username string `json:"username"`
	Fullname string `json:"fullname"`
	Group    string `json:"group"`
}

// PUT /api/users/{id}
func handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	me := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "id tidak valid")
		return
	}
	target, err := getUserByID(id)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "pengguna tidak ditemukan")
		return
	}

	isSelf := me.ID == target.ID
	if me.Grp != grpRoot && !isSelf {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}
	if target.Grp == grpRoot && me.Grp != grpRoot {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}
	if req.Username == "" {
		req.Username = target.Username
	}
	if !validUsername(req.Username) {
		writeAPIError(w, http.StatusBadRequest, "username hanya boleh alfanumerik tanpa simbol")
		return
	}
	if other, err := getUserByUsername(req.Username); err == nil && other.ID != target.ID {
		writeAPIError(w, http.StatusConflict, "username sudah dipakai")
		return
	}
	if req.Group == "" {
		req.Group = target.Grp
	}
	if !validGroups[req.Group] {
		writeAPIError(w, http.StatusBadRequest, "grup tidak valid")
		return
	}

	if me.Grp == grpRoot && req.Group != target.Grp {
		if target.Grp == grpRoot && req.Group != grpRoot {
			writeAPIError(w, http.StatusBadRequest, "grup root tidak boleh diubah")
			return
		}
		if err := updateUserGroup(target.ID, req.Group); err != nil {
			writeError(w, err)
			return
		}
	}

	if req.Username != target.Username || req.Fullname != target.Fullname {
		if err := updateUserProfile(target.ID, req.Username, req.Fullname); err != nil {
			writeError(w, err)
			return
		}
	}

	updated, err := getUserByID(target.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, toUserResponse(updated))
}

type passwordRequest struct {
	Current         string `json:"current"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

// PUT /api/users/{id}/password
func handleChangePassword(w http.ResponseWriter, r *http.Request) {
	me := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "id tidak valid")
		return
	}
	target, err := getUserByID(id)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "pengguna tidak ditemukan")
		return
	}
	isSelf := me.ID == target.ID
	if me.Grp != grpRoot && !isSelf {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}

	var req passwordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}
	if isSelf && !strings.HasPrefix(target.Password, "$argon2id$") {
		writeAPIError(w, http.StatusInternalServerError, "hash kata sandi tidak valid")
		return
	}
	if isSelf {
		ok, err := verifyPassword(target.Password, req.Current)
		if err != nil || !ok {
			writeAPIError(w, http.StatusUnauthorized, "kata sandi saat ini salah")
			return
		}
	}
	if !validPassword(req.Password) {
		writeAPIError(w, http.StatusBadRequest,
			"kata sandi minimal 8 karakter, berisi 1 huruf besar, 1 huruf kecil, 1 angka, dan 1 simbol ASCII")
		return
	}
	if req.Password != req.PasswordConfirm {
		writeAPIError(w, http.StatusBadRequest, "konfirmasi kata sandi tidak cocok")
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := updateUserPassword(target.ID, hash); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/users/{id}
func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "id tidak valid")
		return
	}
	me := currentUser(r)
	target, err := getUserByID(id)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "pengguna tidak ditemukan")
		return
	}
	if me.ID == target.ID {
		writeAPIError(w, http.StatusBadRequest, "tidak bisa menghapus akun sendiri")
		return
	}
	if target.Grp == grpRoot {
		writeAPIError(w, http.StatusBadRequest, "pengguna grup root tidak bisa dihapus")
		return
	}
	if err := deleteUser(target.ID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}