(function () {
  "use strict";

  var status = document.getElementById("server-status");
  var health = document.getElementById("server-health");
  var started = document.getElementById("server-started");
  var startedWrap = document.getElementById("server-started-wrap");

  // data-status keeps the machine-readable state for the stylesheet, while the
  // visible text comes from the localized label the API already renders.
  var setBadge = function (node, value, label, fallback) {
    if (!node) {
      return;
    }
    var next = value || fallback;
    node.textContent = label || next;
    node.setAttribute("data-status", next);
  };

  var actions = document.querySelectorAll("[data-server-action]");
  var syncActions = function (canStart, canStop) {
    actions.forEach(function (button) {
      var action = button.getAttribute("data-server-action");
      if (action === "start") {
        button.disabled = !canStart;
      } else if (action === "stop") {
        button.disabled = !canStop;
      }
    });
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
        var running = data.status === "running" || data.status === "starting";
        setBadge(status, data.status, data.status_label, "unknown");
        setBadge(health, data.health, data.health_label, "unknown");
        syncActions(
          data.can_start === undefined ? !running : data.can_start,
          data.can_stop === undefined ? running : data.can_stop);
        if (started && startedWrap) {
          if (data.started_at) {
            startedWrap.removeAttribute("hidden");
            started.textContent = data.started_at_label || data.started_at;
            started.setAttribute("datetime", data.started_at);
            started.setAttribute("title", data.started_at);
          } else {
            startedWrap.setAttribute("hidden", "");
          }
        }
      }).catch(function () {
        setBadge(status, "service_unavailable", "недоступен", "service_unavailable");
        syncActions(true, false);
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

  var draw = function (target, lines) {
    if (!target) {
      return;
    }
    while (target.firstChild) {
      target.removeChild(target.firstChild);
    }
    lines.forEach(function (line) {
      var node = document.createElementNS("http://www.w3.org/2000/svg", "polyline");
      node.setAttribute("class", line.class);
      node.setAttribute("points", line.points);
      target.appendChild(node);
    });
  };

  // The server renders the polylines and the axis labels for one maxValue, so
  // the browser only swaps text nodes and never re-implements the geometry.
  var applyChart = function (name, chart) {
    if (!chart) {
      return;
    }
    draw(chartNodes[name], chart.lines || []);
    var labels = {
      max: chart.max_label,
      mid: chart.mid_label,
      min: chart.min_label,
      start: chart.start_label,
      midtime: chart.mid_time,
      end: chart.end_label
    };
    Object.keys(labels).forEach(function (key) {
      var node = document.querySelector("[data-chart-label='" + name + ":" + key + "']");
      if (node && labels[key] !== undefined) {
        node.textContent = labels[key];
      }
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

      var charts = data.charts || {};
      applyChart("ram", charts.ram);
      applyChart("cpu", charts.cpu);
      applyChart("disk", charts.disk);
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
  document.querySelectorAll("form[data-confirm]").forEach(function (form) {
    form.addEventListener("submit", function (event) {
      if (!window.confirm(form.getAttribute("data-confirm") || "")) {
        event.preventDefault();
      }
    });
  });
})();
