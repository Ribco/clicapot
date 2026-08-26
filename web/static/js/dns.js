async function api(url, options = {}) {
  const response = await fetch(url, {
    ...options,
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {})
    }
  });

  const data = await response.json().catch(() => ({}));

  if (!response.ok) {
    throw new Error(data.error || `Request failed (${response.status})`);
  }

  return data;
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

async function loadZones() {
  const container = document.getElementById("zones");

  try {
    const data = await api("/api/v1/dns/zones");

    if (!data.zones.length) {
      container.innerHTML = `
        <div class="dns-card">
          <div class="dns-empty">
            No DNS zones yet.<br>
            Create your first zone to get started.
          </div>
        </div>`;
      return;
    }

    container.innerHTML = data.zones.map(zone => `
      <article class="dns-card">
        <div class="dns-card-head">
          <h2>${escapeHTML(zone.name)}</h2>
          <span class="dns-status">${escapeHTML(zone.status)}</span>
        </div>
        <div class="dns-records" id="records-${zone.id}">
          <div class="dns-empty">Loading records…</div>
        </div>
        <div class="dns-actions">
          <button class="dns-btn primary" onclick="openRecordModal(${zone.id})">+ Add record</button>
          <button class="dns-btn danger" onclick="deleteZone(${zone.id}, '${escapeHTML(zone.name)}')">Delete</button>
        </div>
      </article>
    `).join("");

    await Promise.all(data.zones.map(zone => loadRecords(zone.id)));
  } catch (error) {
    container.innerHTML = `<div class="dns-card"><div class="dns-empty">${escapeHTML(error.message)}</div></div>`;
  }
}

async function loadRecords(zoneID) {
  const container = document.getElementById(`records-${zoneID}`);

  try {
    const data = await api(`/api/v1/dns/zones/${zoneID}/records`);

    if (!data.records.length) {
      container.innerHTML = `<div class="dns-empty">No records</div>`;
      return;
    }

    container.innerHTML = data.records.map(record => `
      <div class="dns-record">
        <span class="dns-type">${escapeHTML(record.type)}</span>
        <span>${escapeHTML(record.name)}</span>
        <span class="dns-content">${escapeHTML(record.content)}</span>
        <span class="dns-ttl dns-muted">${record.ttl}</span>
        <button class="dns-btn danger dns-delete" onclick="deleteRecord(${zoneID}, ${record.id})">×</button>
      </div>
    `).join("");
  } catch (error) {
    container.innerHTML = `<div class="dns-empty">${escapeHTML(error.message)}</div>`;
  }
}

function openZoneModal() {
  document.getElementById("zoneModal").classList.add("open");
  document.getElementById("zoneName").focus();
}

function openRecordModal(zoneID) {
  document.getElementById("recordZone").value = zoneID;
  document.getElementById("recordModal").classList.add("open");
  document.getElementById("recordName").focus();
}

function closeModals() {
  document.querySelectorAll(".dns-modal").forEach(modal => modal.classList.remove("open"));
}

async function createZone() {
  const name = document.getElementById("zoneName").value.trim();

  if (!name) {
    alert("Enter a domain name.");
    return;
  }

  try {
    await api("/api/v1/dns/zones", {
      method: "POST",
      body: JSON.stringify({ name })
    });

    document.getElementById("zoneName").value = "";
    closeModals();
    await loadZones();
  } catch (error) {
    alert(error.message);
  }
}

async function createRecord() {
  const zoneID = document.getElementById("recordZone").value;

  const body = {
    type: document.getElementById("recordType").value,
    name: document.getElementById("recordName").value.trim(),
    content: document.getElementById("recordContent").value.trim(),
    ttl: Number(document.getElementById("recordTTL").value) || 300
  };

  if (!body.name || !body.content) {
    alert("Name and content are required.");
    return;
  }

  try {
    await api(`/api/v1/dns/zones/${zoneID}/records`, {
      method: "POST",
      body: JSON.stringify(body)
    });

    document.getElementById("recordName").value = "";
    document.getElementById("recordContent").value = "";
    closeModals();
    await loadRecords(zoneID);
  } catch (error) {
    alert(error.message);
  }
}

async function deleteZone(zoneID, name) {
  if (!confirm(`Delete DNS zone "${name}"?`)) return;

  try {
    await api(`/api/v1/dns/zones/${zoneID}`, { method: "DELETE" });
    await loadZones();
  } catch (error) {
    alert(error.message);
  }
}

async function deleteRecord(zoneID, recordID) {
  if (!confirm("Delete this DNS record?")) return;

  try {
    await api(`/api/v1/dns/zones/${zoneID}/records/${recordID}`, {
      method: "DELETE"
    });
    await loadRecords(zoneID);
  } catch (error) {
    alert(error.message);
  }
}

document.querySelectorAll(".dns-modal").forEach(modal => {
  modal.addEventListener("click", event => {
    if (event.target === modal) closeModals();
  });
});

loadZones();
