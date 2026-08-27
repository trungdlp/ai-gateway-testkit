(() => {
  "use strict";

  const root = document.documentElement;
  const themeToggle = document.getElementById("theme-toggle");
  const printButton = document.getElementById("print-report");
  const filterButtons = Array.from(document.querySelectorAll("[data-filter]"));
  const searchInput = document.getElementById("scenario-search");
  const scenarios = Array.from(document.querySelectorAll(".scenario-card"));
  const visibleCount = document.getElementById("visible-count");
  const emptyState = document.getElementById("empty-state");
  const expandButton = document.getElementById("expand-all");
  const toast = document.getElementById("toast");
  let activeFilter = "all";
  let toastTimer;

  const storedTheme = (() => {
    try {
      return localStorage.getItem("agtk-report-theme");
    } catch (_) {
      return null;
    }
  })();
  setTheme(storedTheme === "light" || storedTheme === "dark" ? storedTheme : "light");

  function setTheme(theme) {
    root.setAttribute("data-bs-theme", theme);
    if (themeToggle) {
      themeToggle.setAttribute("aria-pressed", String(theme === "light"));
      themeToggle.setAttribute("aria-label", theme === "dark" ? "Switch to light theme" : "Switch to dark theme");
    }
    const themeMeta = document.querySelector('meta[name="theme-color"]');
    if (themeMeta) {
      themeMeta.setAttribute("content", theme === "dark" ? "#07111f" : "#f3f7fb");
    }
  }

  if (themeToggle) {
    themeToggle.addEventListener("click", () => {
      const nextTheme = root.getAttribute("data-bs-theme") === "dark" ? "light" : "dark";
      setTheme(nextTheme);
      try {
        localStorage.setItem("agtk-report-theme", nextTheme);
      } catch (_) {
        // Theme persistence is optional for file:// reports.
      }
    });
  }

  function matchesFilter(status, filter) {
    if (filter === "all") return true;
    if (filter === "pass") return status === "pass";
    if (filter === "issues") return status === "fail" || status === "error";
    if (filter === "unavailable") return status === "blocked" || status === "skipped" || status === "not_applicable";
    return true;
  }

  function applyFilters() {
    const query = searchInput ? searchInput.value.trim().toLowerCase() : "";
    let count = 0;
    scenarios.forEach((scenario) => {
      const status = scenario.dataset.status || "";
      const searchable = scenario.dataset.search || "";
      const visible = matchesFilter(status, activeFilter) && (!query || searchable.includes(query));
      scenario.hidden = !visible;
      if (visible) count += 1;
    });
    if (visibleCount) visibleCount.textContent = String(count);
    if (emptyState) emptyState.hidden = count !== 0;
    updateExpandLabel();
  }

  filterButtons.forEach((button) => {
    button.addEventListener("click", () => {
      activeFilter = button.dataset.filter || "all";
      filterButtons.forEach((item) => {
        const selected = item === button;
        item.classList.toggle("active", selected);
        item.setAttribute("aria-pressed", String(selected));
      });
      applyFilters();
    });
  });

  if (searchInput) {
    searchInput.addEventListener("input", applyFilters);
  }

  function visibleScenarios() {
    return scenarios.filter((scenario) => !scenario.hidden);
  }

  function updateExpandLabel() {
    if (!expandButton) return;
    const visible = visibleScenarios();
    const allExpanded = visible.length > 0 && visible.every((scenario) => scenario.open);
    expandButton.textContent = allExpanded ? "Collapse all" : "Expand all";
    expandButton.setAttribute("aria-label", allExpanded ? "Collapse all visible scenarios" : "Expand all visible scenarios");
  }

  scenarios.forEach((scenario) => scenario.addEventListener("toggle", updateExpandLabel));

  if (expandButton) {
    expandButton.addEventListener("click", () => {
      const visible = visibleScenarios();
      const shouldOpen = !visible.every((scenario) => scenario.open);
      visible.forEach((scenario) => {
        scenario.open = shouldOpen;
      });
      updateExpandLabel();
    });
  }

  function showToast(message) {
    if (!toast) return;
    toast.textContent = message;
    toast.classList.add("visible");
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(() => toast.classList.remove("visible"), 2600);
  }

  async function copyText(value) {
    try {
      await navigator.clipboard.writeText(value);
      return true;
    } catch (_) {
      const textArea = document.createElement("textarea");
      textArea.value = value;
      textArea.setAttribute("readonly", "");
      textArea.style.position = "fixed";
      textArea.style.opacity = "0";
      document.body.appendChild(textArea);
      textArea.select();
      const copied = document.execCommand("copy");
      textArea.remove();
      return copied;
    }
  }

  document.querySelectorAll("[data-copy]").forEach((button) => {
    button.addEventListener("click", async () => {
      const copied = await copyText(button.dataset.copy || "");
      showToast(copied ? "Fingerprint copied" : "Unable to copy fingerprint");
    });
  });

  if (printButton) {
    printButton.addEventListener("click", () => window.print());
  }

  applyFilters();
})();
