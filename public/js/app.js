function fmtBytes(b) {
  if (!b) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let i = 0;
  let v = b;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return (i === 0 ? v : v.toFixed(i > 2 ? 0 : 1)) + " " + units[i];
}

function pct(used, max) {
  return max ? Math.min(100, Math.round((used / max) * 100)) : 0;
}

function barClass(p) {
  if (p >= 90) return "crit";
  if (p >= 70) return "warn";
  return "";
}

function svgIcon(path, cls) {
  return `<img src="${path}" class="${cls || ""}" alt="" />`;
}

function fmtUptime(sec) {
  if (!sec) return "-";
  var y = Math.floor(sec / (365.25 * 86400));
  var rem = sec % Math.floor(365.25 * 86400);
  var mo = Math.floor(rem / (30.44 * 86400));
  rem = rem % Math.floor(30.44 * 86400);
  var d = Math.floor(rem / 86400);
  var h = Math.floor((rem % 86400) / 3600);
  var m = Math.floor((rem % 3600) / 60);
  var s = rem % 60;
  var parts = [];
  if (y) parts.push(y + "y");
  if (mo) parts.push(mo + "m");
  if (d) parts.push(d + "d");
  parts.push(
    h + ":" + String(m).padStart(2, "0") + ":" + String(s).padStart(2, "0"),
  );
  return parts.join("'");
}

export { fmtBytes, pct, barClass, svgIcon, fmtUptime };
