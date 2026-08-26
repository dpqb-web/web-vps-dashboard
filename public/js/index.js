import { fmtBytes, pct, barClass, svgIcon, fmtUptime } from "./app.js";

var listEl = document.getElementById("list");
var searchEl = document.getElementById("searchInput");
var servers = [];
var prevLen = 0;
var prevQ = "";
var expanded = new Set();
var graphData = {};
var MAX_POINTS = 60;
var GRAPH_COLORS = {
  cpu: { line: "#0d9488", fill: "rgba(13,148,136,0.12)" },
  mem: { line: "#6366f1", fill: "rgba(99,102,241,0.12)" },
};

function render() {
  var q = (searchEl.value || "").toLowerCase().trim();
  var filtered = servers.filter(function (s) {
    if (!q) return true;
    return (s.name || "").toLowerCase().includes(q) || String(s.id).includes(q);
  });

  document.getElementById("statTotal").textContent = servers.length;
  document.getElementById("statUp").textContent = servers.filter(function (s) {
    return s.status === "running";
  }).length;
  document.getElementById("statDown").textContent = servers.filter(
    function (s) {
      return s.status !== "running";
    },
  ).length;

  if (!filtered.length) {
    if (prevLen !== 0 || servers.length === 0) {
      listEl.innerHTML = servers.length
        ? '<div class="empty">tidak ada hasil cocok</div>'
        : '<div class="empty">belum ada VM atau container ditemukan</div>';
      prevLen = servers.length;
    }
    return;
  }

  filtered.sort(function (a, b) {
    return a.id - b.id;
  });

  if (servers.length !== prevLen || q !== prevQ) {
    buildList(filtered);
    prevLen = servers.length;
    prevQ = q;
  } else {
    updateList(filtered);
  }
}

function buildList(filtered) {
  listEl.innerHTML = filtered
    .map(function (s) {
      return rowHtml(s);
    })
    .join("");

  expanded.forEach(function (key) {
    var row = listEl.querySelector('.row[data-key="' + key + '"]');
    if (row) renderDetail(row, key);
  });
}

function updateList(filtered) {
  filtered.forEach(function (s) {
    var key = s.node + "-" + s.type + "-" + s.id;
    var row = listEl.querySelector('.row[data-key="' + key + '"]');
    if (!row) return;

    var running = s.status === "running";
    var wasExpanded = expanded.has(key);

    row.className = "row" + (wasExpanded ? " expanded" : "");

    var statusEl = row.querySelector(".status-pill");
    if (statusEl) {
      statusEl.className = "status-pill " + (running ? "running" : "stopped");
      statusEl.querySelector(".d").nextSibling.textContent = running
        ? "running"
        : "stopped";
    }

    var actionsEl = row.querySelector(".actions");
    if (actionsEl) {
      actionsEl.innerHTML = "";
      if (!running) {
        actionsEl.innerHTML +=
          '<button title="Start" data-action="start">' +
          svgIcon("/img/bx-play.svg") +
          "</button>";
      }
      if (running) {
        actionsEl.innerHTML +=
          '<button title="Reboot" data-action="reboot">' +
          svgIcon("/img/bx-rotate-cw.svg") +
          "</button>";
        actionsEl.innerHTML +=
          '<button class="danger" title="Stop" data-action="stop">' +
          svgIcon("/img/bx-stop.svg") +
          "</button>";
      }
    }

    if (wasExpanded) {
      renderDetail(row, key);
    }
  });
}

function rowHtml(s) {
  var running = s.status === "running";
  var key = s.node + "-" + s.type + "-" + s.id;
  var isExpanded = expanded.has(key);

  return (
    '<div class="row ' +
    (isExpanded ? "expanded" : "") +
    '" data-key="' +
    key +
    '">' +
    '<div class="slot">#' +
    s.id +
    "</div>" +
    '<div class="idcol row-summary">' +
    '<div class="name">' +
    (s.name || "vm-" + s.id) +
    ' <span class="chevron">' +
    svgIcon("/img/bx-chevron-right.svg") +
    "</span></div>" +
    '<div class="meta">' +
    s.node +
    "</div>" +
    "</div>" +
    "<div>" +
    '<span class="status-pill ' +
    (running ? "running" : "stopped") +
    '"><span class="d"></span>' +
    (running ? "running" : "stopped") +
    "</span>" +
    "</div>" +
    '<div class="actions">' +
    (!running
      ? '<button title="Start" data-action="start">' +
        svgIcon("/img/bx-play.svg") +
        "</button>"
      : "") +
    (running
      ? '<button title="Reboot" data-action="reboot">' +
        svgIcon("/img/bx-rotate-cw.svg") +
        "</button>"
      : "") +
    (running
      ? '<button class="danger" title="Stop" data-action="stop">' +
        svgIcon("/img/bx-stop.svg") +
        "</button>"
      : "") +
    "</div>" +
    '<div class="row-detail"></div>' +
    "</div>"
  );
}

async function renderDetail(row, key) {
  var detailEl = row.querySelector(".row-detail");
  if (!detailEl) return;

  var parts = key.split("-");
  var node = parts[0];
  var type = parts[1];
  var vmid = parts[2];
  var s = servers.find(function (s) {
    return s.node + "-" + s.type + "-" + s.id === key;
  });
  if (!s) return;
  var running = s.status === "running";

  try {
    var res = await fetch("/api/servers/" + node + "/" + type + "/" + vmid);
    if (!res.ok) throw new Error("gagal mengambil detail");
    var data = await res.json();
    var cfg = data.config || {};

    var specs =
      '<div class="detail-specs">' +
      '<div class="spec-item"><div class="spec-label">Node</div><div class="spec-value">' +
      node +
      "</div></div>" +
      '<div class="spec-item"><div class="spec-label">Tipe</div><div class="spec-value">' +
      type.toUpperCase() +
      "</div></div>" +
      '<div class="spec-item"><div class="spec-label">VMID</div><div class="spec-value">' +
      vmid +
      "</div></div>" +
      '<div class="spec-item"><div class="spec-label">CPU</div><div class="spec-value">' +
      (cfg.cores || s.cpu || "-") +
      " core" +
      ((cfg.cores || s.cpu) > 1 ? "s" : "") +
      "</div></div>" +
      '<div class="spec-item"><div class="spec-label">RAM</div><div class="spec-value">' +
      fmtBytes(cfg.memory ? cfg.memory * 1024 * 1024 : s.maxmem) +
      "</div></div>" +
      '<div class="spec-item"><div class="spec-label">Disk</div><div class="spec-value">' +
      fmtBytes(s.maxdisk) +
      "</div></div>" +
      '<div class="spec-item"><div class="spec-label">Uptime</div><div class="spec-value">' +
      fmtUptime(s.uptime) +
      "</div></div>" +
      "</div>";

    var graphs = "";
    if (running) {
      graphs =
        '<div class="graphs">' +
        '<div class="graph-card"><h4>CPU</h4>' +
        '<div class="graph-val">' +
        Math.round((s.cpuUsage || 0) * 100) +
        "%</div>" +
        '<canvas class="graph-canvas" data-metric="cpu" data-key="' +
        key +
        '"></canvas>' +
        '<div class="graph-canvas-label"><span>0%</span><span>100%</span></div>' +
        "</div>" +
        '<div class="graph-card"><h4>RAM</h4>' +
        '<div class="graph-val">' +
        fmtBytes(s.mem) +
        " / " +
        fmtBytes(s.maxmem) +
        "</div>" +
        '<canvas class="graph-canvas" data-metric="mem" data-key="' +
        key +
        '"></canvas>' +
        '<div class="graph-canvas-label"><span>0</span><span>' +
        fmtBytes(s.maxmem) +
        "</span></div>" +
        "</div></div>";
    } else {
      var curMemMB = cfg.memory || Math.round((s.maxmem || 0) / (1024 * 1024));
      var curDiskGB = Math.round((s.maxdisk || 0) / (1024 * 1024 * 1024));
      graphs =
        '<div class="resize-panel">' +
        '<div class="resize-item"><span class="rlabel">RAM</span>' +
        '<span class="rcurrent">' +
        fmtBytes(curMemMB * 1024 * 1024) +
        "</span>" +
        '<button data-resize="memory">' +
        svgIcon("/img/bx-plus.svg") +
        " Tambah</button></div>" +
        '<div class="resize-item"><span class="rlabel">Disk</span>' +
        '<span class="rcurrent">' +
        fmtBytes(curDiskGB * 1024 * 1024 * 1024) +
        "</span>" +
        '<button data-resize="disk">' +
        svgIcon("/img/bx-plus.svg") +
        " Tambah</button></div></div>";
    }

    var detailActions =
      '<div class="detail-actions">' +
      (!running
        ? '<button data-action="start">' +
          svgIcon("/img/bx-play.svg") +
          " Start</button>"
        : "") +
      (running
        ? '<button data-action="reboot">' +
          svgIcon("/img/bx-rotate-cw.svg") +
          " Reboot</button>"
        : "") +
      (running
        ? '<button class="danger" data-action="stop">' +
          svgIcon("/img/bx-stop.svg") +
          " Stop</button>"
        : "") +
      '<button data-action="reset" hidden>' +
      svgIcon("/img/bx-rotate-ccw-dot.svg") +
      " Reset</button>" +
      '<button class="danger" data-action="delete">' +
      svgIcon("/img/bx-trash.svg") +
      " Hapus</button>" +
      "</div>";

    detailEl.innerHTML = specs + graphs + detailActions;

    if (running) {
      pushGraphData(key, s);
      drawAllCanvases(key);
    }
  } catch (err) {
    detailEl.innerHTML = '<div class="error">' + err.message + "</div>";
  }
}

function pushGraphData(key, s) {
  if (!graphData[key]) graphData[key] = [];
  graphData[key].push({
    cpu: Math.round((s.cpuUsage || 0) * 100),
    mem: pct(s.mem, s.maxmem),
  });
  if (graphData[key].length > MAX_POINTS) graphData[key].shift();
}

function drawAllCanvases(key) {
  var data = graphData[key];
  if (!data || data.length < 2) return;

  var row = listEl.querySelector('.row[data-key="' + key + '"]');
  if (!row) return;

  var canvases = row.querySelectorAll("canvas.graph-canvas");
  canvases.forEach(function (c) {
    drawLineGraph(c, data, c.dataset.metric);
  });
}

function drawLineGraph(canvas, data, metric) {
  var ctx = canvas.getContext("2d");
  var dpr = window.devicePixelRatio || 1;
  var w = canvas.offsetWidth;
  var h = canvas.offsetHeight;
  canvas.width = w * dpr;
  canvas.height = h * dpr;
  ctx.scale(dpr, dpr);
  ctx.clearRect(0, 0, w, h);

  if (data.length < 2) return;

  var vals = data.map(function (d) {
    return d[metric];
  });
  var max = 100;
  var color = GRAPH_COLORS[metric] || GRAPH_COLORS.cpu;
  var pad = 2;

  ctx.beginPath();
  ctx.strokeStyle = color.line;
  ctx.lineWidth = 1.5;
  ctx.lineJoin = "round";

  for (var i = 0; i < vals.length; i++) {
    var x = (i / (MAX_POINTS - 1)) * w;
    var y = pad + ((max - vals[i]) / max) * (h - pad * 2);
    if (i === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  ctx.stroke();

  ctx.beginPath();
  for (var i = 0; i < vals.length; i++) {
    var x = (i / (MAX_POINTS - 1)) * w;
    var y = pad + ((max - vals[i]) / max) * (h - pad * 2);
    if (i === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  var lastX = ((vals.length - 1) / (MAX_POINTS - 1)) * w;
  ctx.lineTo(lastX, h);
  ctx.lineTo(0, h);
  ctx.closePath();
  ctx.fillStyle = color.fill;
  ctx.fill();

  ctx.beginPath();
  ctx.strokeStyle = color.line;
  ctx.lineWidth = 1;
  for (var y = 0; y <= 4; y++) {
    var py = pad + (y / 4) * (h - pad * 2);
    ctx.moveTo(0, py);
    ctx.lineTo(w, py);
  }
  ctx.globalAlpha = 0.15;
  ctx.stroke();
  ctx.globalAlpha = 1;
}

async function load() {
  try {
    var res = await fetch("/api/servers");
    if (!res.ok) throw new Error("gagal mengambil data (" + res.status + ")");
    servers = await res.json();
    render();
    updateExpandedGraphs();
  } catch (err) {
    listEl.innerHTML =
      '<div class="error">' +
      err.message +
      '<br><span style="color:var(--muted)">cek konfigurasi dan koneksi ke Proxmox</span></div>';
  }
}

function updateExpandedGraphs() {
  expanded.forEach(function (key) {
    var s = servers.find(function (s) {
      return s.node + "-" + s.type + "-" + s.id === key;
    });
    if (s && s.status === "running") {
      pushGraphData(key, s);
      drawAllCanvases(key);
    }
  });
}

function showDialog(id) {
  var dlg = document.getElementById(id);
  if (dlg) dlg.showModal();
}

function closeDialog(id) {
  var dlg = document.getElementById(id);
  if (dlg) dlg.close();
}

document.querySelectorAll("[data-close]").forEach(function (btn) {
  btn.addEventListener("click", function () {
    closeDialog(btn.dataset.close);
  });
});

document.querySelectorAll("dialog").forEach(function (dlg) {
  dlg.addEventListener("click", function (e) {
    if (e.target === dlg) dlg.close();
  });
});

searchEl.addEventListener("input", render);

listEl.addEventListener("click", async function (e) {
  var btn = e.target.closest("button[data-action]");
  if (btn) {
    e.stopPropagation();
    var row = btn.closest(".row");
    var key = row.dataset.key;
    var parts = key.split("-");
    var node = parts[0];
    var type = parts[1];
    var id = parts[2];
    var action = btn.dataset.action;

    if (action === "stop") {
      document.getElementById("dlgStopBody").textContent =
        "Yakin mau stop " + type.toUpperCase() + " #" + id + "?";
      document.getElementById("dlgStopShutdown").onclick = async function () {
        closeDialog("dlgStop");
        await doAction(node, type, id, "shutdown", row);
      };
      document.getElementById("dlgStopForce").onclick = async function () {
        closeDialog("dlgStop");
        await doAction(node, type, id, "stop", row);
      };
      showDialog("dlgStop");
      return;
    }

    if (action === "delete") {
      document.getElementById("dlgRemoveBody").textContent =
        "Yakin mau menghapus " + type.toUpperCase() + " #" + id + "?";
      document.getElementById("dlgRemoveOk").onclick = async function () {
        closeDialog("dlgRemove");
        try {
          var res = await fetch(
            "/api/servers/" + node + "/" + type + "/" + id,
            { method: "DELETE" },
          );
          if (!res.ok)
            throw new Error((await res.json()).error || "gagal menghapus");
          expanded.delete(key);
          delete graphData[key];
          setTimeout(load, 1000);
        } catch (err) {
          alert(err.message);
          render();
        }
      };
      showDialog("dlgRemove");
      return;
    }

    if (action === "reboot") {
      document.getElementById("dlgRebootBody").textContent =
        "Yakin mau reboot " + type.toUpperCase() + " #" + id + "?";
      document.getElementById("dlgRebootOk").onclick = async function () {
        closeDialog("dlgReboot");
        await doAction(node, type, id, "reboot", row);
      };
      showDialog("dlgReboot");
      return;
    }

    if (action === "reset") {
      document.getElementById("dlgResetBody").textContent =
        "Yakin mau reset " + type.toUpperCase() + " #" + id + "?";
      document.getElementById("dlgResetOk").onclick = async function () {
        closeDialog("dlgReset");
        try {
          var res = await fetch(
            "/api/servers/" + node + "/" + type + "/" + id + "/reset",
            { method: "POST" },
          );
          if (!res.ok)
            throw new Error((await res.json()).error || "gagal mereset");
          expanded.delete(key);
          delete graphData[key];
          setTimeout(load, 1000);
        } catch (err) {
          alert(err.message);
          render();
        }
      };
      showDialog("dlgReset");
      return;
    }

    await doAction(node, type, id, action, row);
    return;
  }

  var resizeBtn = e.target.closest("button[data-resize]");
  if (resizeBtn) {
    e.stopPropagation();
    var row = resizeBtn.closest(".row");
    var key = row.dataset.key;
    var parts = key.split("-");
    var node = parts[0];
    var type = parts[1];
    var id = parts[2];
    var kind = resizeBtn.dataset.resize;
    var s = servers.find(function (s) {
      return s.node + "-" + s.type + "-" + s.id === key;
    });
    openResizeDialog(s, kind, node, type, id);
    return;
  }

  var summary = e.target.closest(".row-summary");
  if (summary) {
    var row = summary.closest(".row");
    var key = row.dataset.key;
    if (expanded.has(key)) {
      expanded.delete(key);
      row.classList.remove("expanded");
      var detail = row.querySelector(".row-detail");
      if (detail) detail.innerHTML = "";
    } else {
      expanded.add(key);
      row.classList.add("expanded");
      renderDetail(row, key);
    }
    return;
  }
});

async function doAction(node, type, id, action, row) {
  row.querySelectorAll("button").forEach(function (b) {
    b.disabled = true;
  });
  try {
    var res = await fetch(
      "/api/servers/" + node + "/" + type + "/" + id + "/" + action,
      { method: "POST" },
    );
    if (!res.ok)
      throw new Error((await res.json()).error || "gagal menjalankan aksi");
    setTimeout(load, 1000);
  } catch (err) {
    alert(err.message);
    render();
  }
}

function openResizeDialog(s, kind, node, type, id) {
  var isMem = kind === "memory";
  var curBytes = isMem ? s.maxmem || 0 : s.maxdisk || 0;
  var curVal = isMem
    ? Math.round(curBytes / (1024 * 1024))
    : Math.round(curBytes / (1024 * 1024 * 1024));
  var unit = isMem ? "MB" : "GB";

  document.getElementById("dlgResizeTitle").textContent = isMem
    ? "Ubah RAM"
    : "Ubah Disk";
  document.getElementById("dlgResizeCurrent").textContent =
    "Saat ini: " + fmtBytes(curBytes);
  document.getElementById("dlgResizeLabel").textContent =
    "Ukuran baru (" + unit + ")";
  var input = document.getElementById("dlgResizeInput");
  input.value = "";
  input.placeholder = "contoh: " + (curVal + (isMem ? 256 : 10));
  input.min = isMem ? 64 : 1;
  input.dataset.kind = kind;
  input.dataset.node = node;
  input.dataset.type = type;
  input.dataset.id = id;
  input.dataset.curval = curVal;

  document.getElementById("dlgResizeOk").onclick = async function () {
    var newVal = parseInt(input.value);
    if (!newVal || newVal <= 0) {
      input.style.borderColor = "var(--down)";
      return;
    }
    var delta = newVal - curVal;
    if (delta <= 0) {
      input.style.borderColor = "var(--down)";
      return;
    }

    closeDialog("dlgResize");
    try {
      var res = await fetch(
        "/api/servers/" + node + "/" + type + "/" + id + "/resize/" + kind,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ delta: delta }),
        },
      );
      if (!res.ok) throw new Error((await res.json()).error || "gagal resize");
      setTimeout(load, 1000);
    } catch (err) {
      alert(err.message);
    }
  };

  showDialog("dlgResize");
}

document
  .getElementById("btnCreate")
  .addEventListener("click", async function () {
    var sel = document.getElementById("createNode");
    sel.innerHTML = '<option value="">memuat...</option>';
    showDialog("dlgCreate");

    try {
      var res = await fetch("/api/nodes");
      if (!res.ok) throw new Error("gagal mengambil node");
      var nodes = await res.json();
      sel.innerHTML = nodes
        .map(function (n) {
          return '<option value="' + n.node + '">' + n.node + "</option>";
        })
        .join("");
    } catch {
      sel.innerHTML = '<option value="">gagal memuat</option>';
    }
  });

document
  .getElementById("dlgCreateOk")
  .addEventListener("click", async function () {
    var node = document.getElementById("createNode").value;
    var type = document.getElementById("createType").value;
    var name = document.getElementById("createName").value.trim();
    var cores = parseInt(document.getElementById("createCores").value) || 1;
    var memory =
      parseInt(document.getElementById("createMemory").value) || 1024;
    var disk = parseInt(document.getElementById("createDisk").value) || 8;

    if (!node || !name) {
      alert("Node dan Nama wajib diisi");
      return;
    }

    closeDialog("dlgCreate");
    try {
      var res = await fetch("/api/servers", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          node: node,
          type: type,
          name: name,
          cores: cores,
          memory: memory,
          disk: disk,
        }),
      });
      if (!res.ok)
        throw new Error((await res.json()).error || "gagal membuat VM");
      setTimeout(load, 2000);
    } catch (err) {
      alert(err.message);
    }
  });

load();
setInterval(load, 1000);
