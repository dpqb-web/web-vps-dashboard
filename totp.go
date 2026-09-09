package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"net/http"
	"strconv"
	"strings"

	"github.com/pquerna/otp/totp"
)

func validateTOTP(code, secret string) bool {
	if secret == "" {
		return false
	}
	return totp.Validate(strings.TrimSpace(code), secret)
}

// POST /api/users/{id}/totp/setup
func handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
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
	if me.Grp != grpRoot && me.ID != target.ID {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}
	if target.TOTPSecret != "" {
		writeAPIError(w, http.StatusBadRequest, "TOTP sudah aktif")
		return
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Proxmox Dashboard",
		AccountName: target.Username,
		Period:      30,
		SecretSize:  20,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	if err := setTOTPPending(target.ID, key.Secret()); err != nil {
		writeError(w, err)
		return
	}

	img, err := key.Image(240, 240)
	var dataURL string
	if err == nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err == nil {
			dataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
		}
	}

	writeJSON(w, map[string]string{
		"otpauth": key.URL(),
		"secret":  key.Secret(),
		"qr":      dataURL,
	})
}

// POST /api/users/{id}/totp/verify
func handleTOTPVerify(w http.ResponseWriter, r *http.Request) {
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
	if me.Grp != grpRoot && me.ID != target.ID {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}
	if target.TOTPPending == "" {
		writeAPIError(w, http.StatusBadRequest, "tidak ada TOTP menunggu verifikasi")
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}
	if !validateTOTP(req.Code, target.TOTPPending) {
		writeAPIError(w, http.StatusBadRequest, "kode TOTP salah")
		return
	}
	if err := confirmTOTP(target.ID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"enabled": true})
}

// DELETE /api/users/{id}/totp
func handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
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
	if me.Grp != grpRoot && me.ID != target.ID {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}
	if err := clearTOTP(target.ID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}