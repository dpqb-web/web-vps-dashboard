package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

type User struct {
	ID         int64
	Username   string
	Fullname   string
	Password   string
	Grp        string
	TOTPSecret string
	TOTPPending string
	Created    string
}

type Session struct {
	Token   string
	UserID  int64
	Expires time.Time
}

type ServerHolder struct {
	VMID   int64
	Node   string
	Holder int64
}

func initDB(path, rootPassword string) {
	dsn := fmt.Sprintf("%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	var err error
	db, err = sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalln("Gagal buka database:", err)
	}
	db.SetMaxOpenConns(1)

	stmt := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			username  TEXT NOT NULL UNIQUE,
			fullname  TEXT NOT NULL DEFAULT '',
			password  TEXT NOT NULL,
			grp       TEXT NOT NULL DEFAULT 'noc' CHECK (grp IN ('root','admin','noc')),
			totp_secret  TEXT,
			totp_pending TEXT,
			created   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS server_holders (
			vmid   INTEGER UNIQUE NOT NULL,
			node   TEXT UNIQUE NOT NULL,
			holder INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			PRIMARY KEY (vmid, node)
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token   TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires TIMESTAMP NOT NULL
		)`,
	}
	for _, q := range stmt {
		if _, err := db.Exec(q); err != nil {
			log.Fatalln("Gagal inisialisasi tabel:", err)
		}
	}

	bootstrapRoot(rootPassword)
}

func bootstrapRoot(rootPassword string) {
	var n int
	if err := db.QueryRow("SELECT COUNT(1) FROM users WHERE grp='root'").Scan(&n); err != nil {
		log.Fatalln("Gagal cek user root:", err)
	}
	if n > 0 {
		return
	}

	if rootPassword == "" {
		b := make([]byte, 12)
		rand.Read(b)
		rootPassword = hex.EncodeToString(b)[:16]
		log.Printf("Rekam baik-baik: root user dibuat dengan kata sandi sementara: %s", rootPassword)
	} else {
		log.Println("root user dibuat dari config [app].root_password")
	}

	hash, err := hashPassword(rootPassword)
	if err != nil {
		log.Fatalln("Gagal hash kata sandi root:", err)
	}
	_, err = db.Exec("INSERT INTO users (username, fullname, password, grp) VALUES ('root', 'Administrator', ?, 'root')", hash)
	if err != nil {
		log.Fatalln("Gagal membuat user root:", err)
	}
}

func rowToUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Fullname, &u.Password, &u.Grp, &u.TOTPSecret, &u.TOTPPending, &u.Created)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func scanUsers(rows *sql.Rows) ([]*User, error) {
	defer rows.Close()
	list := []*User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Fullname, &u.Password, &u.Grp, &u.TOTPSecret, &u.TOTPPending, &u.Created); err != nil {
			return nil, err
		}
		list = append(list, &u)
	}
	return list, rows.Err()
}

func getUserByID(id int64) (*User, error) {
	return rowToUser(db.QueryRow(
		"SELECT id, username, fullname, password, grp, COALESCE(totp_secret,'') AS totp_secret, COALESCE(totp_pending,'') AS totp_pending, created FROM users WHERE id=?", id))
}

func getUserByUsername(username string) (*User, error) {
	return rowToUser(db.QueryRow(
		"SELECT id, username, fullname, password, grp, COALESCE(totp_secret,'') AS totp_secret, COALESCE(totp_pending,'') AS totp_pending, created FROM users WHERE username=?", username))
}

func listUsers() ([]*User, error) {
	rows, err := db.Query(
		"SELECT id, username, fullname, password, grp, COALESCE(totp_secret,'') AS totp_secret, COALESCE(totp_pending,'') AS totp_pending, created FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	return scanUsers(rows)
}

func createUser(username, fullname, grp, passwordHash string) (*User, error) {
	res, err := db.Exec(
		"INSERT INTO users (username, fullname, password, grp) VALUES (?,?,?,?)",
		username, fullname, passwordHash, grp)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return getUserByID(id)
}

func updateUserProfile(id int64, username, fullname string) error {
	_, err := db.Exec("UPDATE users SET username=?, fullname=? WHERE id=?", username, fullname, id)
	return err
}

func updateUserGroup(id int64, grp string) error {
	_, err := db.Exec("UPDATE users SET grp=? WHERE id=?", grp, id)
	return err
}

func updateUserPassword(id int64, passwordHash string) error {
	_, err := db.Exec("UPDATE users SET password=? WHERE id=?", passwordHash, id)
	return err
}

func setTOTPPending(id int64, secret string) error {
	_, err := db.Exec("UPDATE users SET totp_pending=? WHERE id=?", secret, id)
	return err
}

func confirmTOTP(id int64) error {
	_, err := db.Exec(
		"UPDATE users SET totp_secret=totp_pending, totp_pending=NULL WHERE id=?", id)
	return err
}

func clearTOTP(id int64) error {
	_, err := db.Exec("UPDATE users SET totp_secret=NULL, totp_pending=NULL WHERE id=?", id)
	return err
}

func deleteUser(id int64) error {
	if _, err := db.Exec("DELETE FROM server_holders WHERE holder=?", id); err != nil {
		return err
	}
	_, err := db.Exec("DELETE FROM users WHERE id=?", id)
	return err
}

func createSession(userID int64) (*Session, error) {
	token := randomHex(32)
	s := &Session{Token: token, UserID: userID, Expires: time.Now().Add(24 * time.Hour)}
	_, err := db.Exec(
		"INSERT INTO sessions (token, user_id, expires) VALUES (?,?,?)",
		token, userID, s.Expires)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func getSession(token string) (*Session, error) {
	s := &Session{}
	err := db.QueryRow(
		"SELECT token, user_id, expires FROM sessions WHERE token=?", token,
	).Scan(&s.Token, &s.UserID, &s.Expires)
	if err != nil {
		return nil, err
	}
	if time.Now().After(s.Expires) {
		deleteSession(token)
		return nil, fmt.Errorf("sesi kedaluwarsa")
	}
	return s, nil
}

func deleteSession(token string) {
	db.Exec("DELETE FROM sessions WHERE token=?", token)
}

func setServerHolder(vmid int64, node string, holder int64) error {
	_, err := db.Exec(`
		INSERT INTO server_holders (vmid, node, holder) VALUES (?,?,?)
		ON CONFLICT(vmid) DO UPDATE SET node=excluded.node, holder=excluded.holder`,
		vmid, node, holder)
	return err
}

func clearServerHolder(vmid int64, node string) error {
	_, err := db.Exec("DELETE FROM server_holders WHERE vmid=? AND node=?", vmid, node)
	return err
}

func holderForServer(vmid int64) (*ServerHolder, error) {
	h := &ServerHolder{}
	err := db.QueryRow(
		"SELECT vmid, node, holder FROM server_holders WHERE vmid=?", vmid,
	).Scan(&h.VMID, &h.Node, &h.Holder)
	if err != nil {
		return nil, err
	}
	return h, nil
}

func allHolders() ([]*ServerHolder, error) {
	rows, err := db.Query("SELECT vmid, node, holder FROM server_holders")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []*ServerHolder{}
	for rows.Next() {
		var h ServerHolder
		if err := rows.Scan(&h.VMID, &h.Node, &h.Holder); err != nil {
			return nil, err
		}
		list = append(list, &h)
	}
	return list, rows.Err()
}

func isHolder(node string, vmid int64, userID int64) (bool, error) {
	var n int
	err := db.QueryRow(
		"SELECT COUNT(1) FROM server_holders WHERE node=? AND vmid=? AND holder=?",
		node, vmid, userID).Scan(&n)
	return err == nil && n > 0, err
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}