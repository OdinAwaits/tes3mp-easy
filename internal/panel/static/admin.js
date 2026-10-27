(function () {
  "use strict";

  var status = document.getElementById("server-status");
  var health = document.getElementById("server-health");
  var timer = document.getElementById("backup-timer");
  var started = document.getElementById("server-started");

  var setBadge = function (node, value, fallback) {
    if (!node) {
      return;
    }
    var next = value || fallback;
    node.textContent = next;
    node.setAttribute("data-status", next);
  };

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
        setBadge(status, data.status, "unknown");
        setBadge(health, data.health, "unknown");
        setBadge(timer, data.backup_timer, "unknown");
        if (started && data.started_at) {
          started.textContent = data.started_at;
          started.setAttribute("datetime", data.started_at);
        }
      }).catch(function () {
        setBadge(status, "service_unavailable", "service_unavailable");
      });
    };
    refreshStatus();
    window.setInterval(refreshStatus, 5000);
  }

  var resources = document.getElementById("resources");
  var chartNodes = {
    ram: document.querySelector("[data-chart-lines='ram']"),
    cpu: document.querySelector("[data-chart-lines='cpu']"),
    disk: document.querySelector("[data-chart-lines='disk']")
  };

  var formatBytes = function (value) {
    if (value === null || value === undefined) {
      return "—";
    }
    var units = ["B", "KiB", "MiB", "GiB", "TiB"];
    var current = Number(value);
    var unit = 0;
    while (current >= 1024 && unit < units.length - 1) {
      current /= 1024;
      unit += 1;
    }
    return (unit === 0 ? Math.round(current) : current.toFixed(1)) + " " + units[unit];
  };

  var formatPercent = function (value) {
    return value === null || value === undefined ? "—" : Number(value).toFixed(1) + " %";
  };

  var setMetric = function (name, value) {
    var node = document.querySelector("[data-metric='" + name + "']");
    if (node) {
      node.textContent = value;
    }
  };

  var chartLines = function (samples, field, max, css) {
    var result = [];
    var points = [];
    var flush = function () {
      if (points.length) {
        result.push({ css: css, points: points.join(" ") });
        points = [];
      }
    };
    var first = samples.length ? Date.parse(samples[0].ts) : 0;
    var last = samples.length ? Date.parse(samples[samples.length - 1].ts) : 0;
    var span = last - first;
    samples.forEach(function (sample) {
      var value = sample[field];
      if (value === null || value === undefined) {
        flush();
        return;
      }
      var ratioX = span > 0 ? (Date.parse(sample.ts) - first) / span : 0.5;
      ratioX = Math.max(0, Math.min(1, ratioX));
      var ratioY = Math.max(0, Math.min(1, Number(value) / max));
      var x = 40 + ratioX * 550;
      var y = 130 - ratioY * 110;
      points.push(Math.round(x) + "," + Math.round(y));
    });
    flush();
    return result;
  };

  var draw = function (target, lines) {
    if (!target) {
      return;
    }
    while (target.firstChild) {
      target.removeChild(target.firstChild);
    }
    lines.forEach(function (line) {
      var node = document.createElementNS("http://www.w3.org/2000/svg", "polyline");
      node.setAttribute("class", line.css);
      node.setAttribute("points", line.points);
      target.appendChild(node);
    });
  };

  var refreshMetrics = function () {
    if (!resources || !window.fetch) {
      return;
    }
    var windowName = resources.getAttribute("data-window") || "1h";
    fetch("/api/v1/admin/metrics?window=" + encodeURIComponent(windowName), {
      credentials: "same-origin",
      headers: { "Accept": "application/json" },
      cache: "no-store"
    }).then(function (response) {
      if (!response.ok) {
        throw new Error("metrics request failed");
      }
      return response.json();
    }).then(function (payload) {
      var data = payload.data || {};
      var samples = data.samples || [];
      if (!samples.length) {
        return;
      }
      var last = samples[samples.length - 1];
      setMetric("ram", formatBytes(last.game_ram));
      setMetric("game-cpu", formatPercent(last.game_cpu));
      setMetric("sys-cpu", formatPercent(last.sys_cpu));
      setMetric("sys-ram", formatBytes(last.sys_ram_used));
      setMetric("swap", formatBytes(last.swap_used));
      setMetric("disk", formatBytes(last.disk_free));

      draw(chartNodes.ram, chartLines(samples, "game_ram", 448 * 1024 * 1024, "chart-line"));
      draw(chartNodes.cpu, chartLines(samples, "game_cpu", 100, "chart-line").concat(
        chartLines(samples, "sys_cpu", 100, "chart-line chart-line--info")));
      var diskMax = 7 * 1024 * 1024 * 1024;
      samples.forEach(function (sample) {
        if (sample.disk_free && sample.disk_free * 1.4 > diskMax) {
          diskMax = sample.disk_free * 1.4;
        }
      });
      draw(chartNodes.disk, chartLines(samples, "disk_free", diskMax,
        "chart-line chart-line--muted"));
    }).catch(function () {
      /* Keep the last successful snapshot on transient failures. */
    });
  };
  if (resources) {
    refreshMetrics();
    window.setInterval(refreshMetrics, 15000);
  }

  var configForms = document.querySelectorAll("[data-config-form]");
  configForms.forEach(function (form) {
    var submit = form.querySelector("[data-config-submit]");
    var statusNode = form.querySelector("[data-config-status]");
    var fields = Array.prototype.slice.call(
      form.querySelectorAll("input:not([type='hidden']), select, textarea"));
    fields.forEach(function (field) {
      field.setAttribute("data-initial", field.value);
      field.addEventListener("input", update);
      field.addEventListener("change", update);
    });
    form.addEventListener("reset", function () {
      window.setTimeout(update, 0);
    });

    function update() {
      var changed = fields.filter(function (field) {
        return field.value !== field.getAttribute("data-initial");
      }).length;
      if (statusNode) {
        statusNode.textContent = changed ? "Изменено: " + changed : "Изменений нет";
      }
      if (submit) {
        submit.disabled = changed === 0;
      }
    }
    update();
  });

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
  var mismatchForms = document.querySelectorAll("[data-mismatch-form]");
  mismatchForms.forEach(function (form) {
    var checkbox = form.querySelector("[data-mismatch-checkbox]");
    var submit = form.querySelector("[data-mismatch-submit]");
    if (!checkbox || !submit) {
      return;
    }
    var update = function () {
      submit.disabled = !checkbox.checked;
    };
    checkbox.addEventListener("change", update);
    update();
  });

  var deleteForms = document.querySelectorAll("[data-delete-form]");
  deleteForms.forEach(function (form) {
    var expected = form.getAttribute("data-confirm-name") || "";
    var input = form.querySelector("input[name='confirm']");
    var submit = form.querySelector("[data-delete-submit]");
    if (!input || !submit) {
      return;
    }
    var update = function () {
      submit.disabled = input.value !== expected;
    };
    input.addEventListener("input", update);
    input.addEventListener("change", update);
    update();
  });
})();
