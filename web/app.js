// Sinxronlash sahifasi. Faqat bizning API: POST /sync va GET /sync/status.
// DOM'ga ma'lumot faqat textContent orqali yoziladi (API'dan kelgan matn HTML sifatida talqin qilinmaydi).
(function () {
  "use strict";

  const POLL_MS = 2000;

  const $ = (id) => document.getElementById(id);
  const btn = $("sync-btn");
  const btnLabel = btn.querySelector(".btn__label");
  const alertBox = $("sync-alert");
  const resultBox = $("sync-result");
  const emptyBox = $("sync-empty");
  const badge = $("status-badge");
  const statusTime = $("status-time");

  let requestInFlight = false; // shu sahifadan yuborilgan POST /sync
  let pollTimer = null;

  const ENTITY_NAMES = {
    orders: "Buyurtmalar",
    packages: "Qutilar",
    handovers: "Handover'lar",
    shelves: "Javonlar",
  };

  function fmtTime(iso) {
    if (!iso) return "";
    return new Date(iso).toLocaleString("uz-UZ", { dateStyle: "medium", timeStyle: "medium" });
  }

  function fmtDuration(startIso, endIso) {
    if (!startIso || !endIso) return "";
    const sec = Math.max(0, (new Date(endIso) - new Date(startIso)) / 1000);
    return sec < 60 ? `${sec.toFixed(1)} s` : `${Math.floor(sec / 60)} daq ${Math.round(sec % 60)} s`;
  }

  function setBadge(kind, text) {
    badge.className = `badge badge--${kind}`;
    badge.textContent = text;
  }

  function setBusy(busy) {
    btn.disabled = busy;
    btn.classList.toggle("is-loading", busy);
    btnLabel.textContent = busy ? "Sinxronlanmoqda…" : "Sinxronlash";
  }

  function showAlert(kind, text) {
    alertBox.className = `alert alert--${kind}`;
    alertBox.textContent = text;
    alertBox.hidden = false;
  }

  function hideAlert() {
    alertBox.hidden = true;
  }

  function cell(text, className) {
    const td = document.createElement("td");
    td.textContent = text;
    if (className) td.className = className;
    return td;
  }

  function renderRun(run) {
    if (!run) {
      resultBox.hidden = true;
      emptyBox.hidden = false;
      return;
    }
    emptyBox.hidden = true;
    resultBox.hidden = false;

    $("stat-inserted").textContent = run.inserted;
    $("stat-updated").textContent = run.updated;
    $("stat-unchanged").textContent = run.unchanged;

    const tbody = $("entities");
    tbody.replaceChildren();
    for (const e of run.entities || []) {
      const tr = document.createElement("tr");
      const hasTotal = e.api_total !== null && e.api_total !== undefined;
      // Olingan son API total'dan farq qilsa: sync paytida sahifalar siljigan bo'lishi mumkin.
      const mismatch = hasTotal && e.fetched !== e.api_total;
      tr.append(
        cell(ENTITY_NAMES[e.entity] || e.entity, "entity"),
        cell(e.fetched, "num" + (mismatch ? " mismatch" : "")),
        cell(hasTotal ? e.api_total : "—", "num" + (mismatch ? " mismatch" : "")),
        cell(e.inserted, "num"),
        cell(e.updated, "num"),
        cell(e.unchanged, "num"),
        cell(e.skipped, "num"),
      );
      tbody.append(tr);
    }

    const parts = [`Sync #${run.id}`, `boshlandi: ${fmtTime(run.started_at)}`];
    if (run.finished_at) parts.push(`davomiyligi: ${fmtDuration(run.started_at, run.finished_at)}`);
    $("run-meta").textContent = parts.join(" · ");
  }

  function renderStatus(running, run) {
    if (running) {
      setBadge("running", "Sync ketmoqda");
    } else if (!run) {
      setBadge("idle", "Hali sync qilinmagan");
    } else if (run.status === "failed") {
      setBadge("failed", "Oxirgi sync xato bilan tugagan");
    } else {
      setBadge("ok", "Tayyor");
    }
    statusTime.textContent = run && run.finished_at ? `Oxirgi sync: ${fmtTime(run.finished_at)}` : "";
  }

  async function loadStatus() {
    try {
      const res = await fetch("/sync/status");
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();

      renderStatus(data.running, data.last_run);
      setBusy(data.running || requestInFlight);
      // Shu sahifadagi so'rov natijasini kutayotganda, oraliq holat bilan ustidan yozmaymiz.
      if (!requestInFlight) {
        renderRun(data.last_run);
        if (data.last_run && data.last_run.status === "failed" && !data.running) {
          showAlert("error", data.last_run.error || "Sync xato bilan tugadi");
        }
      }
      schedulePoll(data.running);
    } catch (err) {
      setBadge("failed", "Server bilan aloqa yo'q");
      schedulePoll(true);
    }
  }

  // Sync ketayotganda holat tez-tez yangilanadi; aks holda so'rov yuborilmaydi.
  function schedulePoll(active) {
    clearTimeout(pollTimer);
    if (active) pollTimer = setTimeout(loadStatus, POLL_MS);
  }

  async function startSync() {
    hideAlert();
    requestInFlight = true;
    setBusy(true);
    setBadge("running", "Sync ketmoqda");
    schedulePoll(true);

    try {
      const res = await fetch("/sync", { method: "POST" });
      const data = await res.json().catch(() => ({}));

      if (res.status === 409) {
        showAlert("warn", "Sync allaqachon ketmoqda. Tugashini kuting.");
      } else if (res.ok) {
        renderRun(data);
      } else {
        // 502: sync xato bilan tugadi, lekin shu paytgacha saqlangan qism bazada qoladi.
        if (data && data.id) renderRun(data);
        showAlert("error", (data && data.error) || `Xato: HTTP ${res.status}`);
      }
    } catch (err) {
      showAlert("error", "Server bilan aloqa uzildi. Sync serverda davom etayotgan bo'lishi mumkin.");
    } finally {
      requestInFlight = false;
      loadStatus();
    }
  }

  btn.addEventListener("click", startSync);
  loadStatus();
})();
