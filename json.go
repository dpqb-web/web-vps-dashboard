package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// pve memanggil Proxmox API dan mengembalikan isi field "data" dari
// responsnya (Proxmox selalu membungkus hasil dalam {"data": ...}).
func pve(method, path string) (json.RawMessage, error) {
	req, err := http.NewRequest(method, fmt.Sprintf("https://%s:%d/api2/json%s", config.Proxmox.Host, config.Proxmox.Port, path), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s@%s!%s=%s", config.Proxmox.User, config.Proxmox.Realm, config.Proxmox.ID, config.Proxmox.Secret))

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("proxmox API %d: %s", resp.StatusCode, string(body))
	}

	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Data, nil
}

func toEntry(vm vmSummary, node, typ string) serverEntry {
	return serverEntry{
		ID: vm.VMID, Name: vm.Name, Node: node, Type: typ, Status: vm.Status,
		Cpu: vm.Cpus, CpuUsage: vm.Cpu, MaxMem: vm.MaxMem, Mem: vm.Mem,
		MaxDisk: vm.MaxDisk, Uptime: vm.Uptime,
	}
}

// Struct ini merepresentasikan bentuk data yang dikembalikan Proxmox.
// Nama field pakai huruf besar (wajib di Go supaya bisa diakses dari luar
// package), tapi tag `json:"..."` menentukan nama field aslinya di JSON.
type pveNode struct {
	Node string `json:"node"`
}

type vmSummary struct {
	VMID    int64   `json:"vmid"`
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	Cpus    float64 `json:"cpus"`
	Cpu     float64 `json:"cpu"`
	MaxMem  int64   `json:"maxmem"`
	Mem     int64   `json:"mem"`
	MaxDisk int64   `json:"maxdisk"`
	Uptime  int64   `json:"uptime"`
}

// Ini bentuk data yang KITA kirim ke frontend — sengaja disederhanakan
// dan diberi nama field yang konsisten (id, type, cpuUsage, dst) supaya
// index.html tidak perlu tahu bedanya struktur qemu vs lxc dari Proxmox.
type serverEntry struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Node     string  `json:"node"`
	Type     string  `json:"type"`
	Status   string  `json:"status"`
	Cpu      float64 `json:"cpu"`
	CpuUsage float64 `json:"cpuUsage"`
	MaxMem   int64   `json:"maxmem"`
	Mem      int64   `json:"mem"`
	MaxDisk  int64   `json:"maxdisk"`
	Uptime   int64   `json:"uptime"`
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
