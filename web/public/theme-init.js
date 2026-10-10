(function () {
  var stored = null;
  try {
    stored = window.localStorage.getItem("theme");
  } catch (error) {
    stored = null;
  }
  var dark = stored === "dark" || ((stored === null || stored === "system") && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.classList.toggle("dark", dark);
  document.documentElement.style.colorScheme = dark ? "dark" : "light";
})();
