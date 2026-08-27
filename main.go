package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Proxmox struct {
		Host   string `toml:"host"`
		Port	 int    `toml:"port"`
		User   string `toml:"user"`
		Realm  string `toml:"realm"`
		ID     string `toml:"id"`
		Secret string `toml:"secret"`
	} `toml:"proxmox"`
	App struct {
		Host   string `toml:"host"`
		Port	  int  `toml:"port"`
	} `toml:"app"`
}
var (
	config      Config
	httpClient  *http.Client
)

func main() {
	_, err := toml.DecodeFile("config.toml", &config)
	if err != nil { log.Fatalln("Gagal baca config.toml:", err) }
	if config.Proxmox.Port == 0 { config.Proxmox.Port = 8006 }
	if config.Proxmox.User == "" { config.Proxmox.User = "root" }
	if config.Proxmox.Realm == "" { config.Proxmox.Realm = "pam" }
	if config.App.Host == "" { config.App.Host = "127.0.0.1" }
	if config.App.Port == 0 { config.App.Port = 8080 }

	if config.Proxmox.Host == "" || config.Proxmox.ID == "" || config.Proxmox.Secret == "" {
		log.Fatalln("config [proxmox] host, id, secret = not set")
	}

	// Proxmox biasanya pakai sertifikat self-signed. InsecureSkipVerify
	// mengizinkan koneksi tetap jalan walau sertifikatnya tidak resmi.
	// Hanya aman dipakai di jaringan yang kamu percaya (LAN/VPN).
	httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	log.Printf("http://%s:%d\n", config.App.Host, config.App.Port)
	log.Fatalln(http.ListenAndServe(fmt.Sprintf("%s:%d", config.App.Host, config.App.Port), AppRoute()))
}
