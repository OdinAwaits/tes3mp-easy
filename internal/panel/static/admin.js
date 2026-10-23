(function () {
  "use strict";

  var status = document.getElementById("server-status");
  var health = document.getElementById("server-health");
  var timer = document.getElementById("backup-timer");

  if (status && window.fetch) {
    var refreshStatus = function () {
      fetch("/api/v1/admin/status", {
        credentials: "same-origin",
        headers: { "Accept": "application/json" },
        cache: "no-store"
      }).then(function (response) {
        if (!response.ok) {
          throw new Error("status request failed");
        }
        return response.json();
      }).then(function (data) {
        status.textContent = data.status || "unknown";
        status.setAttribute("data-status", data.status || "unknown");
        if (health) {
          health.textContent = data.health || "unknown";
        }
        if (timer) {
          timer.textContent = data.backup_timer || "unknown";
        }
      }).catch(function () {
        status.textContent = "недоступен";
        status.setAttribute("data-status", "service_unavailable");
      });
    };
    refreshStatus();
    window.setInterval(refreshStatus, 5000);
  }

  var logs = document.getElementById("admin-logs");
  var toggle = document.getElementById("log-toggle");
  if (logs && window.fetch) {
    var paused = false;
    var refreshLogs = function () {
      if (paused) {
        return;
      }
      var tail = logs.getAttribute("data-tail") || "200";
      fetch("/api/v1/admin/logs?tail=" + encodeURIComponent(tail), {
        credentials: "same-origin",
        headers: { "Accept": "application/json" },
        cache: "no-store"
      }).then(function (response) {
        if (!response.ok) {
          throw new Error("logs request failed");
        }
        return response.json();
      }).then(function (data) {
        logs.textContent = (data.lines || []).join("\n") + "\n";
      }).catch(function () {
        /* Keep the last successful snapshot on transient failures. */
      });
    };
    if (toggle) {
      toggle.addEventListener("click", function () {
        paused = !paused;
        toggle.textContent = paused ? "Продолжить" : "Пауза";
      });
    }
    window.setInterval(refreshLogs, 5000);
  }
})();
