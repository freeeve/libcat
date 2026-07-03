// Light/dark theme: a data-theme attribute on <html> drives the app.css
// token overrides. First visit follows the OS preference; the toggle
// persists an explicit choice.
const KEY = "lcat-theme";

export type Theme = "light" | "dark";

export function initTheme(): Theme {
  const t = stored() ?? preferred();
  apply(t);
  return t;
}

export function toggleTheme(): Theme {
  const next: Theme = currentTheme() === "dark" ? "light" : "dark";
  try {
    localStorage.setItem(KEY, next);
  } catch {
    // storage unavailable (private mode): still toggle for this page life
  }
  apply(next);
  return next;
}

export function currentTheme(): Theme {
  return document.documentElement.dataset.theme === "dark" ? "dark" : "light";
}

function stored(): Theme | null {
  try {
    const v = localStorage.getItem(KEY);
    return v === "dark" || v === "light" ? v : null;
  } catch {
    return null;
  }
}

function preferred(): Theme {
  return typeof matchMedia === "function" && matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function apply(t: Theme) {
  document.documentElement.dataset.theme = t;
}
