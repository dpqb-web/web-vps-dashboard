# AGENTS.MD

## Build & Run

```bash
go build -o proxmox-dashboard .
./proxmox-dashboard
```

Requires Go 1.22+. Config lives in `config.toml`.

## Tech Stack

- **Backend**: Go standard library (`net/http`), no framework. Single dependency: `github.com/BurntSushi/toml`.
- **Frontend**: Vanilla HTML/CSS/JS. No bundler, no framework. ES modules (`type="module"`).
- **Config**: TOML format (`config.toml`).
- **Icons**: Boxicons font in `public/bx/boxicons-filled.min.css`. Use `<i class="bxf bx-<name>"></i>`. No SVG `<img>` tags.
- **Font**: "Commit Mono" bundled in `public/commit/`. Loaded via `commit.css`.

## Code Conventions

### Go

- Follow standard Go conventions. No external router, use Go 1.22 method-prefixed patterns.
- JSON struct tags must use lowercase (`json:"fieldname"`).
- Error messages in Indonesian to match the frontend.

### Frontend

- **No build tools**. Edit files directly in `public/`.
- All text in Indonesian (Bahasa Indonesia).
- Use `fmtBytes()` for all byte formatting — IEC units (KiB, MiB, GiB, TiB) with space separator.
- Use `<i class="bxf bx-<name>"></i>` for all icons. No SVG `<img>` tags.
- Use HTML `<dialog>` for all modal/confirm dialogs. Never use `window.confirm()` or `alert()` in production code.
- Disabled action buttons should be hidden with `[hidden]` attribute, not grayed out.

### API Routes

| Method | Route | Purpose |
|--------|-------|---------|
| GET | `/api/servers` | List all VMs/LXC |
| GET | `/api/nodes` | List nodes |
| POST | `/api/servers` | Create VM/LXC |
| GET | `/api/servers/{node}/{type}/{vmid}` | Detail |
| GET | `/api/metrics/{node}/{type}/{vmid}` | RRDDATA (disk IO, network) |
| POST | `/api/servers/{node}/{type}/{vmid}/{action}` | start/stop/shutdown/reboot |
| DELETE | `/api/servers/{node}/{type}/{vmid}` | Delete |
| PUT | `/api/servers/{node}/{type}/{vmid}/resize/{kind}` | Resize memory/disk |
| POST | `/api/servers/{node}/{type}/{vmid}/reset` | Reset (stop + delete) |

### Smart Rendering

The frontend uses smart rendering to avoid UI glitches:
- `prevLen` tracks previous server count. If count changes, full DOM rebuild via `buildList`.
- `prevQ` tracks previous search query. If query changes, full DOM rebuild.
- `prevFilteredLen` tracks filtered result count. Uses -1 sentinel for empty state to avoid blank bug.
- `prevRunning[key]` tracks running state per key. Actions only rebuild when running state changes.
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
