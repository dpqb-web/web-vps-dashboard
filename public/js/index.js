import { fmtBytes, pct, barClass, fmtUptime } from "./app.js";

var listEl = document.getElementById("list");
var searchEl = document.getElementById("searchInput");
var servers = [];
var prevLen = -1;
var prevQ = "";
var prevFilteredLen = -1;
var expanded = new Set();
var prevRunning = {};
var graphData = {};
var detailCache = {};
var MAX_POINTS = 60;
var GRAPH_COLORS = {
  cpu: { line: "#0d9488", fill: "rgba(13,148,136,0.12)" },
  mem: { line: "#6366f1", fill: "rgba(99,102,241,0.12)" },
  diskread: { line: "#f59e0b", fill: "rgba(245,158,11,0.12)" },
  diskwrite: { line: "#ef4444", fill: "rgba(239,68,68,0.12)" },
  netin: { line: "#3b82f6", fill: "rgba(59,130,246,0.12)" },
  netout: { line: "#ec4899", fill: "rgba(236,72,153,0.12)" },
};

function icon(name, cls) {
  return '<i class="bxf bx-' + name + (cls ? " " + cls : "") + '"></i>';
}

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
    if (prevFilteredLen !== 0 || servers.length === 0) {
      listEl.innerHTML = servers.length
        ? '<div class="empty">tidak ada hasil cocok</div>'
        : '<div class="empty">belum ada VM atau container ditemukan</div>';
      prevFilteredLen = 0;
    }
    return;
  }

  filtered.sort(function (a, b) {
    return a.id - b.id;
  });

  if (servers.length !== prevLen || q !== prevQ) {
    buildList(filtered);
    prevLen = servers.length;
    prevFilteredLen = filtered.length;
    prevQ = q;
  } else {
    updateList(filtered);
    prevFilteredLen = filtered.length;
  }
}

function buildList(filtered) {
  var oldExpanded = new Set(expanded);
  expanded.clear();
  detailCache = {};

  listEl.innerHTML = filtered
    .map(function (s) {
      return rowHtml(s);
    })
    .join("");

  oldExpanded.forEach(function (key) {
    var row = listEl.querySelector('.row[data-key="' + key + '"]');
    if (row) {
      expanded.add(key);
      buildDetail(row, key);
    }
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

    var prev = prevRunning[key];
    if (prev !== running) {
      var actionsEl = row.querySelector(".actions");
      if (actionsEl) {
        actionsEl.innerHTML = "";
        if (!running) {
          actionsEl.innerHTML +=
            '<button title="Start" data-action="start">' +
            icon("play") +
            "</button>";
        }
        if (running) {
          actionsEl.innerHTML +=
            '<button title="Reboot" data-action="reboot">' +
            icon("rotate-cw") +
            "</button>";
          actionsEl.innerHTML +=
            '<button class="danger" title="Stop" data-action="stop">' +
            icon("stop") +
            "</button>";
        }
      }
      if (wasExpanded) {
        delete detailCache[key];
        buildDetail(row, key);
      }
    }

    prevRunning[key] = running;
  });
}

function rowHtml(s) {
  var running = s.status === "running";
  var key = s.node + "-" + s.type + "-" + s.id;
  var isExpanded = expanded.has(key);
  prevRunning[key] = running;

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
    icon("chevron-right") +
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
        icon("play") +
        "</button>"
      : "") +
    (running
      ? '<button title="Reboot" data-action="reboot">' +
        icon("rotate-cw") +
        "</button>"
      : "") +
    (running
      ? '<button class="danger" title="Stop" data-action="stop">' +
        icon("stop") +
        "</button>"
      : "") +
    "</div>" +
    '<div class="row-detail"></div>' +
    "</div>"
  );
}

function buildDetail(row, key) {
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

  if (running) {
    var graphs =
      '<div class="graphs">' +
      '<div class="graph-card"><h4>CPU</h4>' +
      '<div class="graph-val" data-dyn="cpu-pct">0%</div>' +
      '<canvas class="graph-canvas" data-metric="cpu" data-key="' +
      key +
      '"></canvas>' +
      '<div class="graph-canvas-label"><span>0%</span><span>100%</span></div>' +
      "</div>" +
      '<div class="graph-card"><h4>RAM</h4>' +
      '<div class="graph-val" data-dyn="mem-val">— / —</div>' +
      '<canvas class="graph-canvas" data-metric="mem" data-key="' +
      key +
      '"></canvas>' +
      '<div class="graph-canvas-label"><span>0</span><span data-dyn="mem-max">—</span></div>' +
      "</div>" +
      '<div class="graph-card" hidden><h4>Disk IO</h4>' +
      '<div class="graph-val" data-dyn="disk-io">—</div>' +
      '<canvas class="graph-canvas" data-metric="diskread" data-key="' +
      key +
      '"></canvas>' +
      '<canvas class="graph-canvas" data-metric="diskwrite" data-key="' +
      key +
      '"></canvas>' +
      '<div class="graph-canvas-label"><span>write</span><span>read</span></div>' +
      "</div>" +
      '<div class="graph-card" hidden><h4>Network</h4>' +
      '<div class="graph-val" data-dyn="net-io">—</div>' +
      '<canvas class="graph-canvas" data-metric="netin" data-key="' +
      key +
      '"></canvas>' +
      '<canvas class="graph-canvas" data-metric="netout" data-key="' +
      key +
      '"></canvas>' +
      '<div class="graph-canvas-label"><span>down</span><span>up</span></div>' +
      "</div></div>";
  } else {
    var curMemMB = s.maxmem ? Math.round(s.maxmem / (1024 * 1024)) : 0;
    var curDiskGB = s.maxdisk
      ? Math.round(s.maxdisk / (1024 * 1024 * 1024))
      : 0;
    graphs =
      '<div class="resize-panel">' +
      '<div class="resize-item"><span class="rlabel">RAM</span>' +
      '<span class="rcurrent">' +
      fmtBytes(s.maxmem) +
      "</span>" +
      '<button data-resize="memory">' +
      icon("plus") +
      " Tambah</button></div>" +
      '<div class="resize-item"><span class="rlabel">Disk</span>' +
      '<span class="rcurrent">' +
      fmtBytes(s.maxdisk) +
      "</span>" +
      '<button data-resize="disk">' +
      icon("plus") +
      " Tambah</button></div></div>";
  }

  var detailActions =
    '<div class="detail-actions">' +
    (!running
      ? '<button data-action="start">' + icon("play") + " Start</button>"
      : "") +
    (running
      ? '<button data-action="reboot">' + icon("rotate-cw") + " Reboot</button>"
      : "") +
    (running
      ? '<button class="danger" data-action="stop">' +
        icon("stop") +
        " Stop</button>"
      : "") +
    '<button data-action="reset" hidden>' +
    icon("rotate-ccw-dot") +
    " Reset</button>" +
    '<button class="danger" data-action="delete">' +
    icon("trash") +
    " Hapus</button>" +
    "</div>";

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
    '<div class="spec-item"><div class="spec-label">CPU</div><div class="spec-value" data-dyn="cores">—</div></div>' +
    '<div class="spec-item"><div class="spec-label">RAM</div><div class="spec-value" data-dyn="maxmem">—</div></div>' +
    '<div class="spec-item"><div class="spec-label">Disk</div><div class="spec-value" data-dyn="maxdisk">—</div></div>' +
    '<div class="spec-item"><div class="spec-label">Uptime</div><div class="spec-value" data-dyn="uptime">—</div></div>' +
    "</div>";

  detailEl.innerHTML = specs + graphs + detailActions;

  if (running) {
    fetchConfigAndDraw(key, node, type, vmid, s);
  }
}

async function fetchConfigAndDraw(key, node, type, vmid, s) {
  try {
    var res = await fetch("/api/servers/" + node + "/" + type + "/" + vmid);
    if (!res.ok) throw new Error("gagal mengambil detail");
    var data = await res.json();
    var cfg = data.config || {};
    detailCache[key] = cfg;

    var row = listEl.querySelector('.row[data-key="' + key + '"]');
    if (!row) return;
    var detailEl = row.querySelector(".row-detail");
    if (!detailEl) return;

    var coresEl = detailEl.querySelector('[data-dyn="cores"]');
    var maxmemEl = detailEl.querySelector('[data-dyn="maxmem"]');
    var maxdiskEl = detailEl.querySelector('[data-dyn="maxdisk"]');
    var memMaxEl = detailEl.querySelector('[data-dyn="mem-max"]');

    if (coresEl)
      coresEl.textContent =
        (cfg.cores || s.cpu || "-") +
        " core" +
        ((cfg.cores || s.cpu) > 1 ? "s" : "");
    if (maxmemEl)
      maxmemEl.textContent = fmtBytes(
        cfg.memory ? cfg.memory * 1024 * 1024 : s.maxmem,
      );
    if (maxdiskEl) maxdiskEl.textContent = fmtBytes(s.maxdisk);
    if (memMaxEl)
      memMaxEl.textContent = fmtBytes(
        cfg.memory ? cfg.memory * 1024 * 1024 : s.maxmem,
      );

    pushGraphData(key, s);
    drawAllCanvases(key);
    fetchRRDData(key, node, type, vmid);
  } catch (err) {
    /* config fetch failed, retry next cycle */
  }
}

function updateDetailValues(key, s) {
  var row = listEl.querySelector('.row[data-key="' + key + '"]');
  if (!row) return;
  var detailEl = row.querySelector(".row-detail");
  if (!detailEl) return;

  var cpuPctEl = detailEl.querySelector('[data-dyn="cpu-pct"]');
  var memValEl = detailEl.querySelector('[data-dyn="mem-val"]');
  var uptimeEl = detailEl.querySelector('[data-dyn="uptime"]');

  if (cpuPctEl)
    cpuPctEl.textContent = Math.round((s.cpuUsage || 0) * 100) + "%";
  if (memValEl)
    memValEl.textContent = fmtBytes(s.mem) + " / " + fmtBytes(s.maxmem);
  if (uptimeEl) uptimeEl.textContent = fmtUptime(s.uptime);
}

function pushGraphData(key, s) {
  if (!graphData[key]) graphData[key] = {};
  if (!graphData[key].cpu) graphData[key].cpu = [];
  if (!graphData[key].mem) graphData[key].mem = [];
  graphData[key].cpu.push(Math.round((s.cpuUsage || 0) * 100));
  graphData[key].mem.push(pct(s.mem, s.maxmem));
  if (graphData[key].cpu.length > MAX_POINTS) graphData[key].cpu.shift();
  if (graphData[key].mem.length > MAX_POINTS) graphData[key].mem.shift();
}

function fetchRRDData(key, node, type, vmid) {
  if (graphData[key] && graphData[key]._rrdLoading) return;
  if (!graphData[key]) graphData[key] = {};
  graphData[key]._rrdLoading = true;

  fetch("/api/metrics/" + node + "/" + type + "/" + vmid)
    .then(function (res) {
      if (!res.ok) throw new Error("gagal");
      return res.json();
    })
    .then(function (raw) {
      if (!graphData[key]) return;
      graphData[key].diskread = (raw.diskread || []).map(function (d) {
        return d.v;
      });
      graphData[key].diskwrite = (raw.diskwrite || []).map(function (d) {
        return d.v;
      });
      graphData[key].netin = (raw.netin || []).map(function (d) {
        return d.v;
      });
      graphData[key].netout = (raw.netout || []).map(function (d) {
        return d.v;
      });

      while (graphData[key].diskread.length > MAX_POINTS)
        graphData[key].diskread.shift();
      while (graphData[key].diskwrite.length > MAX_POINTS)
        graphData[key].diskwrite.shift();
      while (graphData[key].netin.length > MAX_POINTS)
        graphData[key].netin.shift();
      while (graphData[key].netout.length > MAX_POINTS)
        graphData[key].netout.shift();

      var row = listEl.querySelector('.row[data-key="' + key + '"]');
      if (!row) return;
      var detailEl = row.querySelector(".row-detail");
      if (!detailEl) return;

      var diskEl = detailEl.querySelector('[data-dyn="disk-io"]');
      var netEl = detailEl.querySelector('[data-dyn="net-io"]');

      if (diskEl) {
        var lastDR = graphData[key].diskread.length
          ? graphData[key].diskread[graphData[key].diskread.length - 1]
          : 0;
        var lastDW = graphData[key].diskwrite.length
          ? graphData[key].diskwrite[graphData[key].diskwrite.length - 1]
          : 0;
        diskEl.textContent =
          fmtBytes(lastDW) + " W / " + fmtBytes(lastDR) + " R";
      }

      if (netEl) {
        var lastNI = graphData[key].netin.length
          ? graphData[key].netin[graphData[key].netin.length - 1]
          : 0;
        var lastNO = graphData[key].netout.length
          ? graphData[key].netout[graphData[key].netout.length - 1]
          : 0;
        netEl.textContent =
          fmtBytes(lastNI) + " in / " + fmtBytes(lastNO) + " out";
      }

      drawRRDCanvases(key);
    })
    .catch(function () {})
    .then(function () {
      if (graphData[key]) graphData[key]._rrdLoading = false;
    });
}

function drawRRDCanvases(key) {
  var data = graphData[key];
  if (!data) return;
  var row = listEl.querySelector('.row[data-key="' + key + '"]');
  if (!row) return;

  var canvases = row.querySelectorAll("canvas.graph-canvas");
  canvases.forEach(function (c) {
    var m = c.dataset.metric;
    if (m === "cpu" || m === "mem") return;
    var vals = data[m];
    if (vals && vals.length >= 2) {
      drawLineGraph(c, vals, m);
    }
  });
}

function drawAllCanvases(key) {
  var data = graphData[key];
  if (!data) return;

  var row = listEl.querySelector('.row[data-key="' + key + '"]');
  if (!row) return;

  var canvases = row.querySelectorAll("canvas.graph-canvas");
  canvases.forEach(function (c) {
    var m = c.dataset.metric;
    var vals = data[m];
    if (vals && vals.length >= 2) {
      drawLineGraph(c, vals, m);
    }
  });
}

function drawLineGraph(canvas, data, metric) {
  var ctx = canvas.getContext("2d");
  var dpr = window.devicePixelRatio || 1;
  var w = canvas.offsetWidth;
  var h = canvas.offsetHeight;
  if (w === 0 || h === 0) return;
  canvas.width = w * dpr;
  canvas.height = h * dpr;
  ctx.scale(dpr, dpr);
  ctx.clearRect(0, 0, w, h);

  if (data.length < 2) return;

  var isPercent = metric === "cpu" || metric === "mem";
  var max = 100;
  if (!isPercent) {
    max = 0;
    for (var i = 0; i < data.length; i++) {
      if (data[i] > max) max = data[i];
    }
    if (max === 0) max = 1;
    max = max * 1.1;
  }

  var color = GRAPH_COLORS[metric] || GRAPH_COLORS.cpu;
  var pad = 2;

  ctx.beginPath();
  ctx.strokeStyle = color.line;
  ctx.lineWidth = 1.5;
  ctx.lineJoin = "round";

  for (var i = 0; i < data.length; i++) {
    var x = (i / (MAX_POINTS - 1)) * w;
    var y = pad + ((max - data[i]) / max) * (h - pad * 2);
    if (i === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  ctx.stroke();

  ctx.beginPath();
  for (var i = 0; i < data.length; i++) {
    var x = (i / (MAX_POINTS - 1)) * w;
    var y = pad + ((max - data[i]) / max) * (h - pad * 2);
    if (i === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  var lastX = ((data.length - 1) / (MAX_POINTS - 1)) * w;
  ctx.lineTo(lastX, h);
  ctx.lineTo(0, h);
  ctx.closePath();
  ctx.fillStyle = color.fill;
  ctx.fill();

  if (isPercent) {
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
      updateDetailValues(key, s);
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
          delete prevRunning[key];
          delete detailCache[key];
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
          delete prevRunning[key];
          delete detailCache[key];
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
      buildDetail(row, key);
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
  var unit = isMem ? "MiB" : "GiB";

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

    closeDialog("dlgResize");
    try {
      var payload = isMem
        ? { delta: newVal }
        : { delta: newVal - curVal };
      var res = await fetch(
        "/api/servers/" + node + "/" + type + "/" + id + "/resize/" + kind,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
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
setInterval(function () {
  expanded.forEach(function (key) {
    var s = servers.find(function (s) {
      return s.node + "-" + s.type + "-" + s.id === key;
    });
    if (s && s.status === "running") {
      fetchRRDData(key, s.node, s.type, String(s.id));
    }
  });
}, 30000);
