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
- **Icons**: Boxicons v3.0.8 SVGs in `public/img/`. Always use `fill="currentColor"` so CSS controls color.
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
- Use `svgIcon("/img/bx-*.svg")` for all icons. Never use Unicode emoji/symbols.
- Use HTML `<dialog>` for all modal/confirm dialogs. Never use `window.confirm()` or `alert()` in production code.
- Disabled action buttons should be hidden with `[hidden]` attribute, not grayed out.

### API Routes

| Method | Route | Purpose |
|--------|-------|---------|
| GET | `/api/servers` | List all VMs/LXC |
| GET | `/api/nodes` | List nodes |
| POST | `/api/servers` | Create VM/LXC |
| GET | `/api/servers/{node}/{type}/{vmid}` | Detail |
| POST | `/api/servers/{node}/{type}/{vmid}/{action}` | start/stop/shutdown/reboot |
| DELETE | `/api/servers/{node}/{type}/{vmid}` | Delete |
| PUT | `/api/servers/{node}/{type}/{vmid}/resize/{kind}` | Resize memory/disk |
| POST | `/api/servers/{node}/{type}/{vmid}/reset` | Reset (stop + delete) |

### Smart Rendering

The frontend uses smart rendering to avoid UI glitches:
- `prevLen` tracks previous server count. If count changes, full DOM rebuild.
- If count stays the same, only status pills and action buttons are updated per row.
- Expanded row details are refreshed each cycle.
- Graph data accumulates in `graphData[key]` (max 60 points) and redraws on canvas.

### Line Graphs

Canvas-based line graphs for running VMs (CPU + RAM). Data accumulates at 1s intervals.
Colors: CPU = teal (`#0d9488`), RAM = indigo (`#6366f1`).
