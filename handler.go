package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// GET /api/servers — daftar semua VM & container di semua node
func handleListServers(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	raw, err := pve("GET", "/nodes")
	if err != nil {
		writeError(w, err)
		return
	}

	var nodes []pveNode
	if err := json.Unmarshal(raw, &nodes); err != nil {
		writeError(w, err)
		return
	}

	all := []serverEntry{}
	for _, n := range nodes {
		qemuRaw, _ := pve("GET", "/nodes/"+n.Node+"/qemu")
		lxcRaw, _ := pve("GET", "/nodes/"+n.Node+"/lxc")

		var qemuList, lxcList []vmSummary
		json.Unmarshal(qemuRaw, &qemuList)
		json.Unmarshal(lxcRaw, &lxcList)

		for _, vm := range qemuList {
			all = append(all, toEntry(vm, n.Node, "qemu"))
		}
		for _, ct := range lxcList {
			all = append(all, toEntry(ct, n.Node, "lxc"))
		}
	}

	holderByVMID := map[int64]*User{}
	admins := map[int64]*User{}
	if users, err := listUsers(); err == nil {
		for _, usr := range users {
			admins[usr.ID] = usr
		}
	}
	if holders, err := allHolders(); err == nil {
		for _, h := range holders {
			holderByVMID[h.VMID] = admins[h.Holder]
		}
	}

	for i := range all {
		if h := holderByVMID[all[i].ID]; h != nil {
			all[i].HolderID = h.ID
			all[i].HolderName = h.Username
		}
	}

	if u != nil && u.Grp == grpAdmin {
		filtered := []serverEntry{}
		for _, s := range all {
			if s.HolderID == u.ID {
				filtered = append(filtered, s)
			}
		}
		all = filtered
	}

	writeJSON(w, all)
}

// GET /api/servers/{node}/{type}/{vmid} — detail satu server
func handleServerDetail(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	typ := r.PathValue("type")
	vmid := r.PathValue("vmid")

	vmidInt, err := strconv.ParseInt(vmid, 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "vmid tidak valid")
		return
	}
	if !canViewServer(r, node, vmidInt) {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}

	status, err := pve("GET", fmt.Sprintf("/nodes/%s/%s/%s/status/current", node, typ, vmid))
	if err != nil {
		writeError(w, err)
		return
	}
	config, err := pve("GET", fmt.Sprintf("/nodes/%s/%s/%s/config", node, typ, vmid))
	if err != nil {
		writeError(w, err)
		return
	}

	var interfaces json.RawMessage
	if typ == "lxc" {
		if ifaces, err := pve("GET", fmt.Sprintf("/nodes/%s/lxc/%s/interfaces", node, vmid)); err == nil {
			interfaces = ifaces
		}
	} else if typ == "qemu" {
		if agentNet, err := pve("GET", fmt.Sprintf("/nodes/%s/qemu/%s/agent/network-get-interfaces", node, vmid)); err == nil {
			interfaces = agentNet
		}
	}

	writeJSON(w, map[string]json.RawMessage{"status": status, "config": config, "interfaces": interfaces})
}

var allowedActions = map[string]bool{"start": true, "stop": true, "shutdown": true, "reboot": true}

// POST /api/servers/{node}/{type}/{vmid}/{action} — start/stop/reboot
func handleServerAction(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	typ := r.PathValue("type")
	vmid := r.PathValue("vmid")
	action := r.PathValue("action")

	if !allowedActions[action] {
		writeAPIError(w, http.StatusBadRequest, "action harus salah satu dari: start, stop, shutdown, reboot")
		return
	}
	vmidInt, err := strconv.ParseInt(vmid, 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "vmid tidak valid")
		return
	}
	if !canManageServer(r, node, vmidInt) {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}

	data, err := pve("POST", fmt.Sprintf("/nodes/%s/%s/%s/status/%s", node, typ, vmid, action))
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, map[string]json.RawMessage{"task": data})
}

// GET /api/nodes — daftar semua node
func handleListNodes(w http.ResponseWriter, r *http.Request) {
	raw, err := pve("GET", "/nodes")
	if err != nil {
		writeError(w, err)
		return
	}
	var nodes []pveNode
	if err := json.Unmarshal(raw, &nodes); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, nodes)
}

type createRequest struct {
	Node   string `json:"node"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Cores  int    `json:"cores"`
	Memory int    `json:"memory"`
	Disk   int    `json:"disk"`
}

// POST /api/servers — buat VM/LXC baru (hanya root)
func handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}
	if req.Node == "" || req.Type == "" || req.Name == "" {
		writeAPIError(w, http.StatusBadRequest, "node, type, dan name wajib diisi")
		return
	}
	if req.Type != "qemu" && req.Type != "lxc" {
		writeAPIError(w, http.StatusBadRequest, "type harus qemu atau lxc")
		return
	}
	if req.Cores <= 0 {
		req.Cores = 1
	}
	if req.Memory <= 0 {
		req.Memory = 512
	}
	if req.Disk <= 0 {
		req.Disk = 8
	}

	payload := fmt.Sprintf("cores=%d&memory=%d&disk=%dG&name=%s", req.Cores, req.Memory, req.Disk, req.Name)
	data, err := pve("POST", fmt.Sprintf("/nodes/%s/%s", req.Node, req.Type)+"?"+payload)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]json.RawMessage{"task": data})
}

// DELETE /api/servers/{node}/{type}/{vmid} — hapus VM/LXC
func handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	typ := r.PathValue("type")
	vmid := r.PathValue("vmid")

	vmidInt, err := strconv.ParseInt(vmid, 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "vmid tidak valid")
		return
	}
	if !canManageServer(r, node, vmidInt) {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}

	data, err := pve("DELETE", fmt.Sprintf("/nodes/%s/%s/%s", node, typ, vmid))
	if err != nil {
		writeError(w, err)
		return
	}
	clearServerHolder(vmidInt, node)
	writeJSON(w, map[string]json.RawMessage{"task": data})
}

type resizeRequest struct {
	Delta float64 `json:"delta"`
}

// PUT /api/servers/{node}/{type}/{vmid}/resize — tambah RAM/disk
func handleResizeServer(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	typ := r.PathValue("type")
	vmid := r.PathValue("vmid")
	kind := r.PathValue("kind")

	vmidInt, err := strconv.ParseInt(vmid, 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "vmid tidak valid")
		return
	}
	if !canManageServer(r, node, vmidInt) {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}

	var req resizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}
	if req.Delta <= 0 {
		writeAPIError(w, http.StatusBadRequest, "delta harus lebih dari 0")
		return
	}

	var endpoint string
	switch {
	case kind == "memory":
		endpoint = fmt.Sprintf("/nodes/%s/%s/%s/config?memory=%d", node, typ, vmid, int64(req.Delta))
	case kind == "disk" && typ == "lxc":
		endpoint = fmt.Sprintf("/nodes/%s/lxc/%s/resize?disk=rootfs&size=%%2B%dG", node, vmid, int64(req.Delta))
	case kind == "disk" && typ == "qemu":
		endpoint = fmt.Sprintf("/nodes/%s/qemu/%s/resize?disk=virtio0&size=%%2B%dG", node, vmid, int64(req.Delta))
	default:
		writeAPIError(w, http.StatusBadRequest, "kind harus memory atau disk")
		return
	}

	data, err := pve("PUT", endpoint)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]json.RawMessage{"task": data})
}

// GET /api/metrics/{node}/{type}/{vmid} — real-time disk IO & network dari status/current
func handleMetrics(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	typ := r.PathValue("type")
	vmid := r.PathValue("vmid")

	vmidInt, err := strconv.ParseInt(vmid, 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "vmid tidak valid")
		return
	}
	if !canViewServer(r, node, vmidInt) {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}

	raw, err := pve("GET", fmt.Sprintf("/nodes/%s/%s/%s/status/current", node, typ, vmid))
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, raw)
}

// POST /api/servers/{node}/{type}/{vmid}/reset — reinstall VM/LXC ke kosong
func handleResetServer(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	typ := r.PathValue("type")
	vmid := r.PathValue("vmid")

	vmidInt, err := strconv.ParseInt(vmid, 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "vmid tidak valid")
		return
	}
	if !canManageServer(r, node, vmidInt) {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}

	pve("POST", fmt.Sprintf("/nodes/%s/%s/%s/status/stop", node, typ, vmid))

	data, err := pve("DELETE", fmt.Sprintf("/nodes/%s/%s/%s", node, typ, vmid))
	if err != nil {
		writeError(w, err)
		return
	}
	clearServerHolder(vmidInt, node)
	writeJSON(w, map[string]json.RawMessage{"task": data})
}

type holderRequest struct {
	Holder int64 `json:"holder"`
}

// PUT /api/servers/{node}/{type}/{vmid}/holder — beri penanggung jawab (root)
func handleSetServerHolder(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	vmid := r.PathValue("vmid")

	vmidInt, err := strconv.ParseInt(vmid, 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "vmid tidak valid")
		return
	}
	var req holderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}
	target, err := getUserByID(req.Holder)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "pengguna tidak ditemukan")
		return
	}
	if target.Grp != grpAdmin {
		writeAPIError(w, http.StatusBadRequest, "penanggung jawab harus dari grup admin")
		return
	}
	if err := setServerHolder(vmidInt, node, target.ID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"holder": toUserResponse(target)})
}

// DELETE /api/servers/{node}/{type}/{vmid}/holder — lepas penanggung jawab (root)
func handleClearServerHolder(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	vmid := r.PathValue("vmid")

	vmidInt, err := strconv.ParseInt(vmid, 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "vmid tidak valid")
		return
	}
	if err := clearServerHolder(vmidInt, node); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "public/index.html")
}

func serveSignIn(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "public/signin.html")
}

func serveUsersPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "public/users.html")
}

func AppRoute() http.Handler {
	mux := http.NewServeMux()

	// Publik: masuk & aset statis yang dipakai halaman masuk
	mux.HandleFunc("GET /signin", serveSignIn)
	mux.Handle("GET /style.css", http.FileServer(http.Dir("public")))
	mux.HandleFunc("POST /api/auth/signin", handleSignIn)
	mux.HandleFunc("POST /api/auth/signin/totp", handleSignInTOTP)
	mux.Handle("GET /js/", http.StripPrefix("/js/", http.FileServer(http.Dir("public/js"))))
	mux.Handle("GET /bx/", http.StripPrefix("/bx/", http.FileServer(http.Dir("public/bx"))))
	mux.Handle("GET /commit/", http.StripPrefix("/commit/", http.FileServer(http.Dir("public/commit"))))

	// noVNC hanya untuk pengguna yang sudah masuk
	mux.Handle("GET /noVNC/", requireAuth(http.StripPrefix("/noVNC/", http.FileServer(http.Dir("public/noVNC")))))

	// Halaman
	mux.Handle("GET /", requireAuth(http.HandlerFunc(serveIndex)))
	mux.Handle("GET /users", requireAuth(requireGroup(grpRoot)(http.HandlerFunc(serveUsersPage))))
	mux.Handle("GET /users.html", requireAuth(requireGroup(grpRoot)(http.HandlerFunc(serveUsersPage))))

	// Sesi
	mux.Handle("POST /api/auth/signout", requireAuth(http.HandlerFunc(handleSignOut)))
	mux.Handle("GET /api/auth/me", requireAuth(http.HandlerFunc(handleMe)))
	mux.Handle("PUT /api/auth/password", requireAuth(http.HandlerFunc(handleChangeOwnPassword)))

	// Manajemen pengguna (hanya root)
	rootAPI := func(h http.Handler) http.Handler {
		return requireAuth(requireGroup(grpRoot)(h))
	}
	mux.Handle("GET /api/users", rootAPI(http.HandlerFunc(handleListUsers)))
	mux.Handle("POST /api/users", rootAPI(http.HandlerFunc(handleCreateUser)))
	mux.Handle("PUT /api/users/{id}", requireAuth(http.HandlerFunc(handleUpdateUser)))
	mux.Handle("DELETE /api/users/{id}", rootAPI(http.HandlerFunc(handleDeleteUser)))
	mux.Handle("PUT /api/users/{id}/password", requireAuth(http.HandlerFunc(handleChangePassword)))
	mux.Handle("POST /api/users/{id}/totp/setup", requireAuth(http.HandlerFunc(handleTOTPSetup)))
	mux.Handle("POST /api/users/{id}/totp/verify", requireAuth(http.HandlerFunc(handleTOTPVerify)))
	mux.Handle("DELETE /api/users/{id}/totp", requireAuth(http.HandlerFunc(handleTOTPDisable)))

	// Penanggung jawab server (hanya root)
	mux.Handle("PUT /api/servers/{node}/{type}/{vmid}/holder", rootAPI(http.HandlerFunc(handleSetServerHolder)))
	mux.Handle("DELETE /api/servers/{node}/{type}/{vmid}/holder", rootAPI(http.HandlerFunc(handleClearServerHolder)))

	// API server
	mux.Handle("GET /api/servers", requireAuth(http.HandlerFunc(handleListServers)))
	mux.Handle("GET /api/nodes", requireAuth(http.HandlerFunc(handleListNodes)))
	mux.Handle("GET /api/metrics/{node}/{type}/{vmid}", requireAuth(http.HandlerFunc(handleMetrics)))
	mux.Handle("POST /api/servers", rootAPI(http.HandlerFunc(handleCreateServer)))
	mux.Handle("GET /api/servers/{node}/{type}/{vmid}", requireAuth(http.HandlerFunc(handleServerDetail)))
	mux.Handle("POST /api/servers/{node}/{type}/{vmid}/{action}", requireAuth(http.HandlerFunc(handleServerAction)))
	mux.Handle("DELETE /api/servers/{node}/{type}/{vmid}", requireAuth(http.HandlerFunc(handleDeleteServer)))
	mux.Handle("PUT /api/servers/{node}/{type}/{vmid}/resize/{kind}", requireAuth(http.HandlerFunc(handleResizeServer)))
	mux.Handle("POST /api/servers/{node}/{type}/{vmid}/reset", requireAuth(http.HandlerFunc(handleResetServer)))

	// Console — broker WebSocket ke Proxmox vncproxy
	mux.Handle("POST /api/console/{node}/{type}/{vmid}", requireAuth(http.HandlerFunc(handleConsoleCreate)))
	mux.Handle("GET /api/console/ws", requireAuth(http.HandlerFunc(handleConsoleWS)))

	return withSecurityHeaders(mux)
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// handleChangeOwnPassword — PUT /api/auth/password untuk ganti kata sandi akun sendiri
func handleChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeAPIError(w, http.StatusUnauthorized, "belum masuk")
		return
	}
	var req passwordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "payload tidak valid")
		return
	}
	ok, err := verifyPassword(u.Password, req.Current)
	if err != nil || !ok {
		writeAPIError(w, http.StatusUnauthorized, "kata sandi saat ini salah")
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
	hash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := updateUserPassword(u.ID, hash); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}