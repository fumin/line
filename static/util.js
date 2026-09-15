let util = {};

util.TimeFormat = function(d) {
  var dtStr = `${d.getFullYear()}-${(d.getMonth()+1).toString().padStart(2, "0")}-${d.getDate().toString().padStart(2, "0")} ${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}:${d.getSeconds().toString().padStart(2, "0")}`
  return dtStr;
}

util.Base64URLSafe = function(s) {
  let res = btoa(unescape(encodeURIComponent(s)));
  res = res.replaceAll("+", "-");
  res = res.replaceAll("/", "_");
  res = res.replaceAll("=", "");
  return res;
}
