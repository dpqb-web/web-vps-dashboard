package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
)

//go:embed public
var publicPath embed.FS

// GET /api/servers — daftar semua VM & container di semua node
func handleListServers(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, all)
}

// GET /api/servers/{node}/{type}/{vmid} — detail satu server
func handleServerDetail(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	typ := r.PathValue("type")
	vmid := r.PathValue("vmid")

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

	writeJSON(w, map[string]json.RawMessage{"status": status, "config": config})
}

var allowedActions = map[string]bool{"start": true, "stop": true, "shutdown": true, "reboot": true}

// POST /api/servers/{node}/{type}/{vmid}/{action} — start/stop/reboot
func handleServerAction(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	typ := r.PathValue("type")
	vmid := r.PathValue("vmid")
	action := r.PathValue("action")

	if !allowedActions[action] {
		http.Error(w, `{"error":"action harus salah satu dari: start, stop, shutdown, reboot"}`, http.StatusBadRequest)
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

// POST /api/servers — buat VM/LXC baru
func handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload tidak valid"}`, http.StatusBadRequest)
		return
	}
	if req.Node == "" || req.Type == "" || req.Name == "" {
		http.Error(w, `{"error":"node, type, dan name wajib diisi"}`, http.StatusBadRequest)
		return
	}
	if req.Type != "qemu" && req.Type != "lxc" {
		http.Error(w, `{"error":"type harus qemu atau lxc"}`, http.StatusBadRequest)
		return
	}
	if req.Cores <= 0 { req.Cores = 1 }
	if req.Memory <= 0 { req.Memory = 512 }
	if req.Disk <= 0 { req.Disk = 8 }

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

	data, err := pve("DELETE", fmt.Sprintf("/nodes/%s/%s/%s", node, typ, vmid))
	if err != nil {
		writeError(w, err)
		return
	}
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

	var req resizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload tidak valid"}`, http.StatusBadRequest)
		return
	}
	if req.Delta <= 0 {
		http.Error(w, `{"error":"delta harus lebih dari 0"}`, http.StatusBadRequest)
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
		http.Error(w, `{"error":"kind harus memory atau disk"}`, http.StatusBadRequest)
		return
	}

	data, err := pve("PUT", endpoint)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]json.RawMessage{"task": data})
}

// GET /api/metrics/{node}/{type}/{vmid} — RRDDATA untuk grafik disk IO & network
func handleMetrics(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	typ := r.PathValue("type")
	vmid := r.PathValue("vmid")

	raw, err := pve("GET", fmt.Sprintf("/nodes/%s/%s/%s/rrddata?timeframe=hour", node, typ, vmid))
	if err != nil {
		writeError(w, err)
		return
	}

	var entries []map[string]interface{}
	if err := json.Unmarshal(raw, &entries); err != nil {
		writeJSON(w, map[string]interface{}{
			"error": "gagal parse rrddata",
			"raw":   string(raw),
		})
		return
	}

	if len(entries) == 0 {
		writeJSON(w, map[string]interface{}{
			"error": "rrddata kosong",
			"raw":   string(raw),
		})
		return
	}

	type dp struct {
		T int64   `json:"t"`
		V float64 `json:"v"`
	}

	result := map[string][]dp{}

	for _, e := range entries {
		ts, _ := e["timestamp"].(float64)
		data, _ := e["data"].(map[string]interface{})
		if data == nil {
			continue
		}
		for key, v := range data {
			if key == "uptime" || key == "status" || key == "name" || key == "vmid" || key == "maxcpu" {
				continue
			}
			switch val := v.(type) {
			case float64:
				result[key] = append(result[key], dp{T: int64(ts), V: val})
			case string:
				var f float64
				fmt.Sscanf(val, "%f", &f)
				result[key] = append(result[key], dp{T: int64(ts), V: f})
			}
		}
	}

	writeJSON(w, result)
}

// POST /api/servers/{node}/{type}/{vmid}/reset — reinstall VM/LXC ke kosong
func handleResetServer(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	typ := r.PathValue("type")
	vmid := r.PathValue("vmid")

	pve("POST", fmt.Sprintf("/nodes/%s/%s/%s/status/stop", node, typ, vmid))

	data, err := pve("DELETE", fmt.Sprintf("/nodes/%s/%s/%s", node, typ, vmid))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]json.RawMessage{"task": data})
}

func AppRoute() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/servers", handleListServers)
	mux.HandleFunc("GET /api/nodes", handleListNodes)
	mux.HandleFunc("GET /api/metrics/{node}/{type}/{vmid}", handleMetrics)
	mux.HandleFunc("POST /api/servers", handleCreateServer)
	mux.HandleFunc("GET /api/servers/{node}/{type}/{vmid}", handleServerDetail)
	mux.HandleFunc("POST /api/servers/{node}/{type}/{vmid}/{action}", handleServerAction)
	mux.HandleFunc("DELETE /api/servers/{node}/{type}/{vmid}", handleDeleteServer)
	mux.HandleFunc("PUT /api/servers/{node}/{type}/{vmid}/resize/{kind}", handleResizeServer)
	mux.HandleFunc("POST /api/servers/{node}/{type}/{vmid}/reset", handleResetServer)

	mux.Handle("/", http.FileServer(http.Dir("public")))
	// webFS, err := fs.Sub(publicPath, "public")
	// if err != nil {log.Fatalln(err)}
	// mux.Handle("/", http.FileServer(http.FS(webFS)))

	return mux
}
