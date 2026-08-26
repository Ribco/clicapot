async function api(path) {
  const response = await fetch(path, {credentials: "same-origin"});

  if (response.status === 401) {
    location.href = "/";
    throw new Error("unauthorized");
  }

  return response.json();
}

async function loadDashboard() {
  const [me, status, projects] = await Promise.all([
    api("/api/v1/me"),
    api("/api/v1/status"),
    api("/api/v1/projects")
  ]);

  document.getElementById("user").textContent = me.user.username;
  document.getElementById("welcome").textContent =
    `Welcome back, ${me.user.username}`;

  document.getElementById("apiStatus").textContent =
    status.status === "operational" ? "Online" : "Offline";

  document.getElementById("projectCount").textContent =
    projects.projects.length;

  const list = document.getElementById("projects");

  if (!projects.projects.length) {
    list.innerHTML = '<div class="loading">No projects yet.</div>';
    return;
  }

  list.innerHTML = projects.projects.map(project => `
    <div class="project">
      <div class="project-name">${escapeHTML(project.name)}</div>
      <div class="project-slug">${escapeHTML(project.slug)}</div>
    </div>
  `).join("");
}

function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, c => ({
    "&":"&amp;",
    "<":"&lt;",
    ">":"&gt;",
    '"':"&quot;",
    "'":"&#039;"
  }[c]));
}

document.getElementById("logout").addEventListener("click", async () => {
  await fetch("/api/v1/auth/logout", {
    method: "POST",
    credentials: "same-origin"
  });
  location.href = "/";
});

document.getElementById("menu").addEventListener("click", () => {
  document.getElementById("sidebar").classList.toggle("open");
});

loadDashboard().catch(console.error);
