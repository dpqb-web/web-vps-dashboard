# AGENTS.MD

## Build & Run

```bash
go build -o proxmox-dashboard .
./proxmox-dashboard
```

Requires Go 1.22+. Config lives in `config.toml`. Backend source lives in the repo root
(package `main`, module name `proxmox-dashboard`).

On first start (no `grp='root'` user in the DB) a `root` user is bootstrapped. If
`[app].root_password` is empty, a random temporary password is printed to stdout. Change it
after first login.

## Tech Stack

- **Backend**: Go standard library (`net/http`), no framework. SQLite via `modernc.org/sqlite`.
- **Dependencies**: `github.com/BurntSushi/toml`, `modernc.org/sqlite`,
  `golang.org/x/crypto/argon2`, `github.com/pquerna/otp/totp`, `github.com/gorilla/websocket`
  (console WebSocket proxy).
- **Frontend**: Vanilla HTML/CSS/JS. No bundler, no framework. ES modules (`type="module"`).
- **Config**: TOML format (`config.toml`).
- **Icons**: Boxicons font in `public/bx/boxicons-filled.min.css`. Use `<i class="bxf bx-<name>"></i>`. No SVG `<img>` tags.
- **Font**: "Commit Mono" bundled in `public/commit/`. Loaded via `commit.css`.

## Code Conventions

### Go

- Follow standard Go conventions. No external router, use Go 1.22 method-prefixed patterns.
- JSON struct tags must use lowercase (`json:"fieldname"`).
- Error messages in Indonesian to match the frontend.
- Authentication middleware chain: `requireAuth` must wrap `requireGroup(...)`, not the other
  way around — `requireGroup` reads the user from the request context, which `requireAuth`
  populates (see `handleSignIn` bug history).

### Frontend

- **No build tools**. Edit files directly in `public/`.
- All text in Indonesian (Bahasa Indonesia).
- Use `fmtBytes()` for all byte formatting — IEC units (KiB, MiB, GiB, TiB) with space separator.
- Use `<i class="bxf bx-<name>"></i>` for all icons. No SVG `<img>` tags.
- Use HTML `<dialog>` for all modal/confirm dialogs. Never use `window.confirm()` or `alert()` in production code.
- Disabled action buttons should be hidden with `[hidden]` attribute, not grayed out.
- `api()` helper in `public/js/webview.js` redirects to `/signin` on 401; current user is cached
  and refreshed on reload. Page scripts: `public/js/index.js` (dashboard), `public/js/users.js`
  (users page), `public/js/signin.js` (signin page).

## Auth & Users

- Groups: `root` (everything), `admin` (only servers it holds via `server_holders`), `noc`
  (read-only, sees all). Server owner is unique per vmid (`server_holders` table, FK to users).
- Passwords: Argon2id (`golang.org/x/crypto/argon2`), minimum 8 chars with upper+lower+digit+
  symbol, ASCII printable only (`validPassword`). Usernames: alphanumeric only (`validUsername`).
- Sessions: token in `sessions` table, cookie `session` (HttpOnly, SameSite=Lax, 24h).
- TOTP: `github.com/pquerna/otp/totp`. Two-step signin: `POST /api/auth/signin` →
  `{totpRequired,tempToken}` (tempToken valid 5 min) → `POST /api/auth/signin/totp`. QR image
  delivered as a `data:image/png` URL from the setup endpoint.
- Root users cannot be deleted or demoted; a root cannot delete itself. Non-root users can only
  manage their own profile/password (with current password).

### API Routes

| Method | Route | Auth | Purpose |
|--------|-------|------|---------|
| GET | `/api/servers` | logged in | List all VMs/LXC (admins: only held servers) |
| GET | `/api/nodes` | logged in | List nodes |
| POST | `/api/servers` | root | Create VM/LXC |
| GET | `/api/servers/{node}/{type}/{vmid}` | view | Detail |
| GET | `/api/metrics/{node}/{type}/{vmid}` | view | RRDDATA (disk IO, network) |
| POST | `/api/servers/{node}/{type}/{vmid}/{action}` | manage | start/stop/shutdown/reboot |
| DELETE | `/api/servers/{node}/{type}/{vmid}` | manage | Delete |
| PUT | `/api/servers/{node}/{type}/{vmid}/resize/{kind}` | manage | Resize memory/disk |
| POST | `/api/servers/{node}/{type}/{vmid}/reset` | manage | Reset (stop + delete) |
| PUT | `/api/servers/{node}/{type}/{vmid}/holder` | root | Set server owner |
| DELETE | `/api/servers/{node}/{type}/{vmid}/holder` | root | Clear server owner |
| POST | `/api/console/{node}/{type}/{vmid}` | manage | Issue one-time console token (see "Console") |
| GET | `/api/console/ws` | logged in | WebSocket bridge to Proxmox vncwebsocket (needs token) |
| POST | `/api/auth/signin` | public | Sign in (password) |
| POST | `/api/auth/signin/totp` | public | Complete signin with TOTP |
| POST | `/api/auth/signout` | logged in | Sign out |
| GET | `/api/auth/me` | logged in | Current user |
| PUT | `/api/auth/password` | logged in | Change own password (needs `current`) |
| GET | `/api/users` | root | List users |
| POST | `/api/users` | root | Create user |
| PUT | `/api/users/{id}` | root or self | Update username/fullname/group |
| DELETE | `/api/users/{id}` | root | Delete user |
| PUT | `/api/users/{id}/password` | root or self | Change password |
| POST | `/api/users/{id}/totp/setup` | root or self | TOTP setup (returns secret + QR data URL) |
| POST | `/api/users/{id}/totp/verify` | root or self | Verify TOTP code |
| DELETE | `/api/users/{id}/totp` | root or self | Disable TOTP |

"view" = root/noc/all, or admin holding the server. "manage" = root, or admin holding the server.

Console (WebSocket proxy): `POST /api/console/{node}/{type}/{vmid}` (manage) calls Proxmox
`vncproxy` (works for both qemu & lxc), stores the PVE port+ticket server-side, and returns a
one-time random token (90s expiry). The browser then loads `/noVNC/vnc.html` and connects to
`GET /api/console/ws?token=...` (logged in), which upgrades to a WS and bridges to PVE's
`/api2/json/nodes/{node}/{type}/{vmid}/vncwebsocket?port=...&vncticket=...` (TLS
InsecureSkipVerify, PVEAPIToken auth header). Port/ticket never reach the browser.

## noVNC

`public/noVNC/` is a self-contained noVNC bundle served at `/noVNC/` (auth required).
In the dashboard, the **Console** button (running qemu/lxc, manage access) POSTs to the console
endpoint above, then opens
`/noVNC/vnc.html?autoconnect=1&resize=scale&path=api/console/ws?token=...&password=...`
in a new tab. The password is the per-session VNC password from `vncproxy`. NoVNC connects via
the `path` setting to the same origin — no direct guest-IP/VNC-port access is used.

## Smart Rendering

The frontend uses smart rendering to avoid UI glitches:
- `prevLen` tracks previous server count. If count changes, full DOM rebuild via `buildList`.
- `prevQ` tracks previous search query. If query changes, full DOM rebuild.
- `prevFilteredLen` tracks filtered result count. Uses -1 sentinel for empty state to avoid blank bug.
- `prevRunning[key]` tracks running state per key. Actions only rebuild when running state changes.
- Threshold badge: `updateList()` creates `span[data-threshold]` if missing (appends to
  `.status-col`) so it appears immediately when a server starts — no full rebuild needed.
- `serverIPs[key]` holds the guest IP (from the agent); only meaningful for running servers. If it
  is still empty for a running expanded server, `updateExpandedGraphs()` re-fetches the config
  (`fetchConfigAndDraw`) throttled to every ~3s via `detailRetry[key]` until the IP appears.
- **Detail panel**: Built once via `buildDetail()` (sync, creates DOM), config fetched separately via `fetchConfigAndDraw()` (async). Dynamic values (cpu%, mem, uptime) updated in-place every second via `updateDetailValues()` — no innerHTML rebuild.
- `detailCache[key]` stores fetched config to avoid redundant API calls.
- Expanded row details are refreshed each cycle.
- Graph data accumulates in `graphData[key]` (max 60 points) and redraws on canvas.
- RRD data (disk IO, network) fetched on expand + refreshed every 30s via `fetchRRDData()` with `_rrdLoading` guard to prevent concurrent fetches.
- `[hidden]` attribute used for DOM toggling (row-detail, disabled buttons).

### Line Graphs

Canvas-based line graphs for running VMs. CPU/mem data accumulates at 1s intervals.
- **CPU**: teal (`#0d9488`), percentage scale 0-100%
- **RAM**: indigo (`#6366f1`), percentage scale 0-100%
- **Disk IO**: amber (`#f59e0b`) for read, red (`#ef4444`) for write. Auto-scaled.
- **Network**: blue (`#3b82f6`) for in, pink (`#ec4899`) for out. Auto-scaled.
- Disk/network data fetched from `/api/metrics/` (RRDDATA endpoint), refreshed every 30s.
