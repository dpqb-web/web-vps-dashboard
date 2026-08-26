# Rack — Dashboard Proxmox VE

Dashboard web untuk mengelola VM (QEMU) dan container (LXC) di Proxmox VE.
Ditulis dalam Go (backend) dan vanilla HTML/CSS/JS (frontend). Tanpa
framework, tanpa bundler — cukup satu binary + folder `public/`.

## Fitur

- **Accordion** — klik nama server untuk melihat detail, grafik, dan aksi.
- **Grafik garis** — CPU dan RAM ditampilkan sebagai grafik garis real-time
  (seperti Task Manager Windows) saat server sedang berjalan.
- **Resize** — ubah RAM atau disk server yang sedang berhenti dengan jumlah
  bebas.
- **Buat / Hapus / Reset** — buat VM/container baru, hapus, atau reset ke
  keadaan kosong.
- **Dialog terpisah** — setiap aksi (stop, reboot, hapus, reset) menggunakan
  dialog konfirmasi sendiri.
- **Pencarian** — cari server berdasarkan nama atau ID.
- **Ikon SVG** — semua ikon menggunakan file SVG, bukan karakter Unicode.
- **Font Commit Mono** — monospace font yang konsisten di semua browser.
- **Auto-refresh** — data diperbarui setiap 1 detik secara otomatis.

## Persyaratan

- Go 1.22 atau lebih baru
- Proxmox VE yang bisa diakses dari jaringan lokal

## Cara Build & Jalankan

### 1. Install Go

```bash
sudo apt-get install -y golang-go
# atau download dari https://go.dev/dl/
go version  # pastikan >= 1.22
```

### 2. Buat API Token di Proxmox

1. Login ke Proxmox web interface.
2. Buat user baru di **Datacenter → Permissions** (atau pakai root).
3. Buat API token: **Datacenter → API Tokens → Add**.
4. Catat `Token ID` dan `Token Secret`.

### 3. Konfigurasi

Buat file `config.toml`:

```toml
[proxmox]
host = "192.168.1.100"  # IP Proxmox
port = 8006
user = "root"
realm = "pam"
id = "dashboard"        # nama user API token
secret = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"

[app]
port = 8080  # port dashboard web (default: 8080)
```

### 4. Build & Jalankan

```bash
go build -o proxmox-dashboard .
./proxmox-dashboard
```

Buka `http://localhost:8080` di browser.

### Compile ke Binary

```bash
go build -o proxmox-dashboard .
```

Hasilnya satu file `proxmox-dashboard` yang bisa dijalankan di mana saja
(asal OS/arsitekturnya sama). Tidak perlu install Node.js atau dependency
lain di server tujuan. Cukup copy binary + folder `public/` + `config.toml`.

## Struktur

```
main.go              → entry point, config, HTTP server
handler.go           → semua route handler (API + static file)
json.go              → Proxmox API client + struct data
config.toml          → konfigurasi Proxmox dan aplikasi
public/
  index.html         → halaman dashboard
  style.css          → semua styling
  js/index.js        → logic utama frontend (accordion, grafik, dialog)
  js/app.js          → fungsi utilitas (fmtBytes, pct, svgIcon, fmtUptime)
  js/webview.js      → disable text selection (mode kiosk)
  commit/            → font Commit Mono
  img/               → ikon SVG (Boxicons)
```

## Catatan Keamanan

`InsecureSkipVerify: true` di `main.go` mengizinkan koneksi ke Proxmox
dengan sertifikat self-signed. Hanya aman dipakai di jaringan yang kamu
percaya (LAN/VPN). Jangan expose dashboard ke internet tanpa proteksi.
