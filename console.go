package main

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type consoleTicket struct {
	Node, Type, VMID string
	Port             int
	Ticket           string
	Created          time.Time
}

var (
	consoleMu      sync.Mutex
	consoleTickets = map[string]*consoleTicket{}
)

func newConsoleToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// handleConsoleCreate — POST /api/console/{node}/{type}/{vmid}
// Meminta vncproxy dari Proxmox, mengembalikan token + password ke frontend.
func handleConsoleCreate(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	typ := r.PathValue("type")
	vmid := r.PathValue("vmid")
	if node == "" || typ == "" || vmid == "" {
		writeAPIError(w, http.StatusBadRequest, "parameter tidak lengkap")
		return
	}
	if typ != "qemu" && typ != "lxc" {
		writeAPIError(w, http.StatusBadRequest, "tipe harus qemu atau lxc")
		return
	}
	if u := currentUser(r); u == nil || !canManageServer(r, node, parseVmid(vmid)) {
		writeAPIError(w, http.StatusForbidden, "tidak diizinkan")
		return
	}
	if !nodeExists(node) {
		writeAPIError(w, http.StatusNotFound, "node tidak ditemukan")
		return
	}

	vncURL := fmt.Sprintf("https://%s:%d/api2/json/nodes/%s/%s/%s/vncproxy",
		config.Proxmox.Host, config.Proxmox.Port,
		url.PathEscape(node), url.PathEscape(typ), url.PathEscape(vmid))

	req, err := http.NewRequestWithContext(r.Context(), "POST", vncURL, strings.NewReader("websocket=1"))
	if err != nil {
		writeError(w, err)
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s@%s!%s=%s",
		config.Proxmox.User, config.Proxmox.Realm, config.Proxmox.ID, config.Proxmox.Secret))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, "tidak bisa menghubungi Proxmox")
		return
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	if resp.StatusCode >= 300 {
		writeAPIError(w, http.StatusBadGateway, "Proxmox: "+string(respBody))
		return
	}

	var wrap struct {
		Data struct {
			Port     string `json:"port"`
			Ticket   string `json:"ticket"`
			Password string `json:"password"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &wrap); err != nil || wrap.Data.Ticket == "" {
		writeAPIError(w, http.StatusBadGateway, "respons Proxmox tidak dikenal")
		return
	}
	port, err := strconv.Atoi(wrap.Data.Port)
	if err != nil || port == 0 {
		writeAPIError(w, http.StatusBadGateway, "Proxmox tidak memberi port proxy")
		return
	}

	token, err := newConsoleToken()
	if err != nil {
		writeError(w, err)
		return
	}
	consoleMu.Lock()
	consoleTickets[token] = &consoleTicket{
		Node:    node,
		Type:    typ,
		VMID:    vmid,
		Port:    port,
		Ticket:  wrap.Data.Ticket,
		Created: time.Now(),
	}
	consoleMu.Unlock()

	writeJSON(w, map[string]interface{}{
		"token":    token,
		"password": wrap.Data.Password,
	})
}

func nodeExists(node string) bool {
	raw, err := pve("GET", "/nodes")
	if err != nil {
		return false
	}
	var list []struct {
		Node string `json:"node"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return false
	}
	for _, n := range list {
		if n.Node == node {
			return true
		}
	}
	return false
}

func parseVmid(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// takeTicket mengambil & menghapus token dari peta (sekali pakai, 90 detik).
func takeTicket(r *http.Request) (*consoleTicket, error) {
	token := r.URL.Query().Get("token")
	if token == "" {
		return nil, fmt.Errorf("token tidak ada")
	}
	consoleMu.Lock()
	defer consoleMu.Unlock()
	t, ok := consoleTickets[token]
	if !ok {
		return nil, fmt.Errorf("token tidak dikenal atau sudah dipakai")
	}
	if time.Since(t.Created) > 90*time.Second {
		delete(consoleTickets, token)
		return nil, fmt.Errorf("token kedaluwarsa")
	}
	delete(consoleTickets, token)
	return t, nil
}

// handleConsoleWS — GET /api/console/ws?token=...
// WebSocket bridge: browser → server ini (x/net/websocket Handler) →
// Proxmox vncwebsocket (via websocket.Dial). Karena tiket hanya diambil dari
// peta server, port & ticket Proxmox tidak pernah sampai ke browser.
func handleConsoleWS(w http.ResponseWriter, r *http.Request) {
	t, err := takeTicket(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	targetHost := fmt.Sprintf("%s:%d", config.Proxmox.Host, config.Proxmox.Port)
	targetPath := fmt.Sprintf("/api2/json/nodes/%s/%s/%s/vncwebsocket?port=%d&vncticket=%s",
		url.PathEscape(t.Node), url.PathEscape(t.Type), url.PathEscape(t.VMID),
		t.Port, url.QueryEscape(t.Ticket))

	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "http://" + r.Host
	}

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer clientConn.Close()

	authHeader := fmt.Sprintf("PVEAPIToken=%s@%s!%s=%s",
		config.Proxmox.User, config.Proxmox.Realm, config.Proxmox.ID, config.Proxmox.Secret)

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	upstream, _, dialErr := dialer.Dial("wss://"+targetHost+targetPath, http.Header{
		"Authorization": {authHeader},
		"Origin":        {origin},
	})
	if dialErr != nil {
		return
	}
	defer upstream.Close()

	errc := make(chan error, 2)
	go func() {
		for {
			mt, data, err := upstream.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := clientConn.WriteMessage(mt, data); err != nil {
				errc <- err
				return
			}
		}
	}()
	go func() {
		for {
			mt, data, err := clientConn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := upstream.WriteMessage(mt, data); err != nil {
				errc <- err
				return
			}
		}
	}()
	<-errc
}
