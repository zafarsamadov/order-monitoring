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
      setBadge("failed", "Oxirgi sync bo'lmadi");
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
        const failed = data.last_run && data.last_run.status === "failed" && !data.running;
        if (failed) {
          // Oxirgi sync bo'lmagan, lekin bazadagi ma'lumot joyida: oxirgi muvaffaqiyatli sync'ni ko'rsatamiz.
          renderRun(data.last_success);
          showAlert("warn", `Oxirgi sinxronlash bo'lmadi: ${data.last_run.error || "noma'lum xato"}. ` +
            "Bazadagi ma'lumotlar saqlanib qolgan — quyida oxirgi muvaffaqiyatli sync; " +
            "anomaliyalar va hisobot shu ma'lumot bo'yicha ishlaydi.");
        } else {
          renderRun(data.last_run);
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

  // ---- Nomutanosibliklar kartasi --------------------------------------------
  // Sync qismidan mustaqil. GET /anomalies bir marta chaqiriladi; checkbox va severity
  // filtri natijani qayta so'ramasdan, sahifaning o'zida ko'rsatadi/yashiradi.

  const anomalyBtn = $("anomaly-btn");
  const anomalyLabel = anomalyBtn.querySelector(".btn__label");
  const anomalyAlert = $("anomaly-alert");
  const anomalyResult = $("anomaly-result");
  const anomalySeverity = $("anomaly-severity");

  let ruleItems = [];            // oxirgi javobdagi qoidalar
  const hiddenRules = new Set(); // checkbox'i olib tashlangan qoidalar

  function renderRuleChecks() {
    const box = $("anomaly-rules");
    box.replaceChildren(...ruleItems.map((item) => {
      const label = document.createElement("label");
      label.className = "rule-check";

      const input = document.createElement("input");
      input.type = "checkbox";
      input.checked = !hiddenRules.has(item.rule);
      input.addEventListener("change", () => {
        if (input.checked) hiddenRules.delete(item.rule);
        else hiddenRules.add(item.rule);
        renderRuleCards();
      });

      const name = document.createElement("span");
      name.textContent = item.rule;
      const count = document.createElement("span");
      count.className = "count";
      count.textContent = `(${item.total})`;

      label.append(input, name, count);
      return label;
    }));
  }

  function renderRuleCards() {
    const severity = anomalySeverity.value;
    const visible = ruleItems.filter((item) =>
      !hiddenRules.has(item.rule) && (!severity || item.severity === severity));

    const totalOrders = visible.reduce((sum, item) => sum + item.total, 0);
    $("anomaly-summary").textContent =
      `Ko'rsatilgan qoidalar: ${visible.length} / ${ruleItems.length} · buyurtmalar: ${totalOrders}`;

    $("anomaly-cards").replaceChildren(...visible.map((item) => {
      const card = document.createElement("div");
      card.className = "rule-card";

      const head = document.createElement("div");
      head.className = "rule-card__head";
      const name = document.createElement("span");
      name.className = "rule-card__name";
      name.textContent = item.rule;
      const sev = document.createElement("span");
      sev.className = `sev sev--${item.severity}`;
      sev.textContent = item.severity;
      const count = document.createElement("span");
      count.className = "rule-card__count";
      count.textContent = `${item.total} ta buyurtma`;
      head.append(name, sev, count);

      const desc = document.createElement("p");
      desc.className = "rule-card__desc";
      desc.textContent = item.description;

      // Hamma buyurtmalar jadvalda; 10 tadan oshsa jadval ichida scroll chiqadi.
      const wrap = document.createElement("div");
      wrap.className = "rule-table";
      const table = document.createElement("table");
      const thead = document.createElement("thead");
      thead.innerHTML = "<tr><th>#</th><th>Buyurtma</th><th>Sabab</th></tr>";
      const tbody = document.createElement("tbody");
      (item.items || []).forEach((ex, i) => {
        const tr = document.createElement("tr");
        for (const text of [i + 1, ex.order_code, ex.reason]) {
          const td = document.createElement("td");
          td.textContent = text;
          tr.append(td);
        }
        tbody.append(tr);
      });
      table.append(thead, tbody);
      wrap.append(table);

      card.append(head, desc, wrap);
      return card;
    }));
  }

  async function loadAnomalies() {
    anomalyAlert.hidden = true;
    anomalyBtn.disabled = true;
    anomalyBtn.classList.add("is-loading");
    anomalyLabel.textContent = "Aniqlanmoqda…";

    try {
      const res = await fetch("/anomalies");
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);

      ruleItems = data.items || [];
      renderRuleChecks();
      renderRuleCards();
      $("anomaly-empty").hidden = true;
      anomalyResult.hidden = false;
    } catch (err) {
      anomalyAlert.textContent = `Anomaliyalarni olib bo'lmadi: ${err.message}`;
      anomalyAlert.hidden = false;
    } finally {
      anomalyBtn.disabled = false;
      anomalyBtn.classList.remove("is-loading");
      anomalyLabel.textContent = "Nomutanosibliklarni aniqlash";
    }
  }

  anomalyBtn.addEventListener("click", loadAnomalies);
  anomalySeverity.addEventListener("change", renderRuleCards);
})();
