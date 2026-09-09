var rowsEl = document.getElementById("userRows");
var editingId = null;

function icon(name) {
  return '<i class="bxf bx-' + name + '"></i>';
}

function setBusy(busy) {
  rowsEl.innerHTML =
    '<tr><td colspan="5" class="users-empty">' +
    (busy ? "memuat pengguna..." : "gagal memuat, muat ulang halaman.") +
    "</td></tr>";
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

function rowHtml(u) {
  var isRoot = u.group === "root";
  var totpClass = u.totp ? "on" : "off";
  var totpText = u.totp ? "aktif" : "mati";
  return (
    '<tr data-id="' +
    u.id +
    '">' +
    "<td><b>" +
    u.username +
    "</b></td>" +
    "<td>" +
    (u.fullname || "—") +
    "</td>" +
    '<td><span class="group-pill ' +
    u.group +
    '">' +
    u.group +
    "</span></td>" +
    '<td><span class="totp-state ' +
    totpClass +
    '">' +
    totpText +
    "</span></td>" +
    '<td><div class="users-actions" style="justify-content:flex-end">' +
    '<button data-edit title="Ubah nama/username">' +
    icon("pencil") +
    " Ubah</button>" +
    '<button data-password title="Ganti kata sandi">' +
    icon("key") +
    " Kata Sandi</button>" +
    '<button data-totp title="Atur TOTP">' +
    icon("shield") +
    " TOTP</button>" +
    (isRoot
      ? "<button class=\"danger\" disabled title=\"grup root tidak bisa dihapus\">" +
        icon("trash") +
        " Hapus</button>"
      : '<button class="danger" data-delete title="Hapus">' +
        icon("trash") +
        " Hapus</button>") +
    "</div></td>" +
    "</tr>"
  );
}

function renderUser(users) {
  rowsEl.innerHTML = users.map(rowHtml).join("");
}

async function loadUsers() {
  setBusy(true);
  try {
    var res = await fetch("/api/users");
    if (res.status === 401) {
      location.href = "/signin";
      return;
    }
    if (!res.ok) throw new Error("gagal memuat pengguna");
    var users = await res.json();
    renderUser(users);
  } catch (err) {
    rowsEl.innerHTML =
      '<tr><td colspan="5" class="users-empty">' +
      err.message +
      "</td></tr>";
  }
}

async function api(path, opts) {
  var res = await fetch(path, opts);
  if (res.status === 401) {
    location.href = "/signin";
    throw new Error("sesi berakhir");
  }
  return res;
}

/* ==== Tambah pengguna ==== */
document.getElementById("btnAddUser").addEventListener("click", function () {
  editingId = null;
  document.getElementById("userFormTitle").textContent = "Tambah Pengguna";
  document.getElementById("userUsername").value = "";
  document.getElementById("userFullname").value = "";
  document.getElementById("userGroup").value = "admin";
  document.getElementById("userPassword").value = "";
  document.getElementById("userPasswordConfirm").value = "";
  document.getElementById("userPassFields").hidden = false;
  showDialog("dlgUserForm");
});

/* ==== Edit pengguna ==== */
rowsEl.addEventListener("click", function (e) {
  var btn = e.target.closest("button[data-edit],[data-password],[data-totp],[data-delete]");
  if (!btn) return;

  var tr = btn.closest("tr");
  var id = parseInt(tr.dataset.id, 10);
  var username = tr.querySelector("b").textContent;
  var fullname = tr.children[1].textContent;
  if (fullname === "—") fullname = "";
  var group = tr.querySelector(".group-pill").textContent;

  if (btn.hasAttribute("data-edit")) {
    editingId = id;
    document.getElementById("userFormTitle").textContent = "Ubah Pengguna";
    document.getElementById("userUsername").value = username;
    document.getElementById("userFullname").value = fullname;
    document.getElementById("userGroup").value = group;
    document.getElementById("userPassFields").hidden = true;
    showDialog("dlgUserForm");
  } else if (btn.hasAttribute("data-password")) {
    openUserPasswordDialog(id, username);
  } else if (btn.hasAttribute("data-totp")) {
    openUserTOTPDialog(id, username);
  } else if (btn.hasAttribute("data-delete")) {
    openUserDeleteDialog(id, username);
  }
});

function selectedUserData() {
  var username = document.getElementById("userUsername").value.trim();
  var fullname = document.getElementById("userFullname").value.trim();
  var group = document.getElementById("userGroup").value;
  return { username: username, fullname: fullname, group: group };
}

document.getElementById("userFormOk").addEventListener("click", async function () {
  var data = selectedUserData();
  var password = document.getElementById("userPassword").value;
  var passwordConfirm = document.getElementById("userPasswordConfirm").value;

  if (editingId === null) {
    if (password !== passwordConfirm) {
      alert("konfirmasi kata sandi tidak cocok");
      return;
    }
    closeDialog("dlgUserForm");
    try {
      var res = await api("/api/users", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          username: data.username,
          fullname: data.fullname,
          group: data.group,
          password: password,
          passwordConfirm: passwordConfirm,
        }),
      });
      if (!res.ok)
        throw new Error((await res.json()).error || "gagal membuat pengguna");
      loadUsers();
    } catch (err) {
      alert(err.message);
    }
  } else {
    closeDialog("dlgUserForm");
    try {
      var res = await api("/api/users/" + editingId, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      if (!res.ok)
        throw new Error((await res.json()).error || "gagal mengubah pengguna");
      loadUsers();
    } catch (err) {
      alert(err.message);
    }
  }
});

/* ==== Ganti kata sandi ==== */
function openUserPasswordDialog(id, username) {
  document.getElementById("userPasswordFor").textContent =
    "Ubah kata sandi untuk " + username + ".";
  document.getElementById("upPassword").value = "";
  document.getElementById("upPasswordConfirm").value = "";
  document.getElementById("upOk").dataset.id = id;
  showDialog("dlgUserPassword");
}

document.getElementById("upOk").addEventListener("click", async function () {
  var id = this.dataset.id;
  var password = document.getElementById("upPassword").value;
  var confirm = document.getElementById("upPasswordConfirm").value;
  if (password !== confirm) {
    alert("konfirmasi kata sandi tidak cocok");
    return;
  }
  closeDialog("dlgUserPassword");
  try {
    var res = await api("/api/users/" + id + "/password", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        current: "",
        password: password,
        passwordConfirm: confirm,
      }),
    });
    if (!res.ok)
      throw new Error((await res.json()).error || "gagal mengganti kata sandi");
  } catch (err) {
    alert(err.message);
  }
});

/* ==== TOTP ==== */
var totpTargetId = null;

function openUserTOTPDialog(id, username) {
  totpTargetId = id;
  document.getElementById("userTOTPFor").textContent =
    "Kelola TOTP untuk " + username + ".";
  var tr = rowsEl.querySelector('tr[data-id="' + id + '"]');
  var on = tr && tr.querySelector(".totp-state").classList.contains("on");
  document.getElementById("utOff").hidden = on;
  document.getElementById("utOn").hidden = !on;
  document.getElementById("utSetup").hidden = true;
  document.getElementById("utCode").value = "";
  document.getElementById("utQR").hidden = true;
  document.getElementById("utSecret").hidden = true;
  showDialog("dlgUserTOTP");
}

document.getElementById("utEnable").addEventListener("click", async function () {
  try {
    var res = await api("/api/users/" + totpTargetId + "/totp/setup", {
      method: "POST",
    });
    if (!res.ok) throw new Error((await res.json()).error || "gagal setup TOTP");
    var data = await res.json();
    var img = document.getElementById("utQR");
    if (data.qr) {
      img.src = data.qr;
      img.hidden = false;
    }
    var secret = document.getElementById("utSecret");
    secret.textContent = "Secret darurat: " + data.secret;
    secret.hidden = false;
    document.getElementById("utSetup").hidden = false;
  } catch (err) {
    alert(err.message);
  }
});

document.getElementById("utVerify").addEventListener("click", async function () {
  var code = document.getElementById("utCode").value.trim();
  if (!code) {
    alert("masukkan kode 6 digit");
    return;
  }
  try {
    var res = await api("/api/users/" + totpTargetId + "/totp/verify", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code: code }),
    });
    if (!res.ok) throw new Error((await res.json()).error || "kode TOTP salah");
    closeDialog("dlgUserTOTP");
    loadUsers();
  } catch (err) {
    alert(err.message);
  }
});

document.getElementById("utDisable").addEventListener("click", async function () {
  try {
    var res = await api("/api/users/" + totpTargetId + "/totp", {
      method: "DELETE",
    });
    if (!res.ok) throw new Error("gagal nonaktifkan TOTP");
    closeDialog("dlgUserTOTP");
    loadUsers();
  } catch (err) {
    alert(err.message);
  }
});

/* ==== Hapus pengguna ==== */
function openUserDeleteDialog(id, username) {
  document.getElementById("userDeleteBody").textContent =
    "Yakin mau menghapus pengguna " + username + "?";
  document.getElementById("userDeleteOk").dataset.id = id;
  showDialog("dlgUserDelete");
}

document.getElementById("userDeleteOk").addEventListener("click", async function () {
  var id = this.dataset.id;
  closeDialog("dlgUserDelete");
  try {
    var res = await api("/api/users/" + id, { method: "DELETE" });
    if (!res.ok)
      throw new Error((await res.json()).error || "gagal menghapus pengguna");
    loadUsers();
  } catch (err) {
    alert(err.message);
  }
});

loadUsers();