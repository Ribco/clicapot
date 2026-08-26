const form = document.getElementById("loginForm");
const error = document.getElementById("error");

form.addEventListener("submit", async (e) => {
  e.preventDefault();
  error.textContent = "";

  const response = await fetch("/api/v1/auth/login", {
    method: "POST",
    credentials: "same-origin",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify({
      email: document.getElementById("email").value,
      password: document.getElementById("password").value
    })
  });

  const data = await response.json();

  if (!response.ok) {
    error.textContent = data.error || "Login failed";
    return;
  }

  location.href = "/dashboard";
});
