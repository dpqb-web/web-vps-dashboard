var form = document.getElementById("signinForm");
var errorEl = document.getElementById("authError");
var usernameEl = document.getElementById("signinUsername");
var passwordEl = document.getElementById("signinPassword");
var totpStep = document.getElementById("totpStep");
var totpEl = document.getElementById("signinTOTP");
var submitBtn = document.getElementById("signinSubmit");

var tempToken = null;

function setBusy(busy) {
  submitBtn.disabled = busy;
  submitBtn.textContent = busy
    ? "Memproses..."
    : tempToken
      ? "Verifikasi"
      : "Masuk";
}

function showError(msg) {
  errorEl.textContent = msg;
  errorEl.classList.add("show");
}

function clearError() {
  errorEl.classList.remove("show");
}

form.addEventListener("submit", async function (e) {
  e.preventDefault();
  clearError();

  if (tempToken) {
    var code = totpEl.value.trim();
    if (!code) {
      showError("Masukkan kode TOTP 6 digit.");
      return;
    }
    setBusy(true);
    try {
      var res = await fetch("/api/auth/signin/totp", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ tempToken: tempToken, code: code }),
      });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || "kode TOTP salah");
      location.href = "/";
    } catch (err) {
      showError(err.message);
      setBusy(false);
    }
    return;
  }

  var username = usernameEl.value.trim();
  var password = passwordEl.value;
  if (!username || !password) {
    showError("Username dan kata sandi wajib diisi.");
    return;
  }

  setBusy(true);
  try {
    var res = await fetch("/api/auth/signin", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: username, password: password }),
    });
    var data = await res.json();
    if (!res.ok) throw new Error(data.error || "gagal masuk");

    if (data.totpRequired) {
      tempToken = data.tempToken;
      totpStep.hidden = false;
      totpEl.value = "";
      totpEl.focus();
      setBusy(false);
      usernameEl.disabled = true;
      passwordEl.disabled = true;
      return;
    }

    location.href = "/";
  } catch (err) {
    showError(err.message);
    setBusy(false);
  }
});