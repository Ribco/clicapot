const savedTheme = localStorage.getItem("clicapot-theme") || "light";

function applyTheme(theme) {
  document.body.classList.toggle("dark", theme === "dark");

  document.querySelectorAll(".theme-btn").forEach(button => {
    button.classList.toggle(
      "selected",
      button.dataset.theme === theme
    );
  });

  localStorage.setItem("clicapot-theme", theme);
}

document.querySelectorAll(".theme-btn").forEach(button => {
  button.addEventListener("click", () => {
    applyTheme(button.dataset.theme);
  });
});

document.getElementById("menu").addEventListener("click", () => {
  document.getElementById("sidebar").classList.toggle("open");
});

document.getElementById("logout").addEventListener("click", async () => {
  await fetch("/api/v1/auth/logout", {
    method: "POST",
    credentials: "same-origin"
  });

  location.href = "/";
});

applyTheme(savedTheme);
