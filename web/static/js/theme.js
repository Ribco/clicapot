(function () {
  const theme = localStorage.getItem("clicapot-theme") || "light";

  if (theme === "dark") {
    document.documentElement.classList.add("dark");
  } else {
    document.documentElement.classList.remove("dark");
  }

  document.addEventListener("DOMContentLoaded", () => {
    document.body.classList.toggle("dark", theme === "dark");
  });
})();
