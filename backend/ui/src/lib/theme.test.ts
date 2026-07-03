import { beforeEach, describe, expect, it } from "vitest";
import { currentTheme, initTheme, toggleTheme } from "./theme";

describe("theme", () => {
  beforeEach(() => {
    localStorage.clear();
    delete document.documentElement.dataset.theme;
  });

  it("defaults to light without a stored choice (jsdom matchMedia absent/light)", () => {
    expect(initTheme()).toBe("light");
    expect(document.documentElement.dataset.theme).toBe("light");
  });

  it("honors a stored choice over the default", () => {
    localStorage.setItem("lcat-theme", "dark");
    expect(initTheme()).toBe("dark");
  });

  it("toggle flips, applies, and persists", () => {
    initTheme();
    expect(toggleTheme()).toBe("dark");
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(localStorage.getItem("lcat-theme")).toBe("dark");
    expect(toggleTheme()).toBe("light");
    expect(currentTheme()).toBe("light");
  });
});
