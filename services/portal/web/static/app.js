// Clock in the header.
(function () {
  var el = document.getElementById("clock");
  if (!el) return;
  function tick() {
    el.textContent = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false });
  }
  tick();
  setInterval(tick, 15000);
})();

// Fallback refresh: if HTMX failed to load (for example before `make vendor`
// has been run), poll the services fragment ourselves so the dashboard still
// updates. With HTMX present this never runs.
window.addEventListener("load", function () {
  if (window.htmx) return;
  setInterval(function () {
    fetch("/ui/services", { cache: "no-store" })
      .then(function (r) { return r.ok ? r.text() : Promise.reject(r.status); })
      .then(function (html) {
        var cur = document.getElementById("services");
        if (cur) cur.outerHTML = html;
      })
      .catch(function () {});
  }, 5000);
});
