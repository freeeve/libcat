// SearchSelect behavior: open-on-click with group headers, label/code
// filtering with diacritic folding and cross-group dedupe, pinned entries
// under any query, keyboard picking, and the external form-reset sync.
// A .svelte.test.ts so props can be a $state proxy (reactive updates).
import { afterEach, describe, expect, it, vi } from "vitest";
import { flushSync, mount, unmount } from "svelte";
import SearchSelect, { type SearchOption } from "./SearchSelect.svelte";

// Mirrors the LANGUAGES shape: a "common" group repeated inside "all".
const OPTIONS: SearchOption[] = [
  { value: "iri/eng", label: "English", code: "eng", group: "common" },
  { value: "iri/spa", label: "Spanish", code: "spa", group: "common" },
  { value: "iri/eng", label: "English", code: "eng", group: "all languages" },
  { value: "iri/mos", label: "Mooré", code: "mos", group: "all languages" },
  { value: "iri/spa", label: "Spanish", code: "spa", group: "all languages" },
];
const PINNED: SearchOption[] = [{ value: "__custom__", label: "Other IRI…" }];

let app: Record<string, unknown> | null = null;

function mountSelect(extra: Partial<{ options: SearchOption[]; pinned: SearchOption[]; value: string; onchange: (v: string) => void }> = {}) {
  const host = document.createElement("div");
  document.body.appendChild(host);
  const props = $state({ options: OPTIONS, pinned: PINNED, ariaLabel: "Add a language", value: "", ...extra });
  app = mount(SearchSelect as never, { target: host, props }) as Record<string, unknown>;
  flushSync();
  const input = host.querySelector<HTMLInputElement>("input")!;
  return { host, input, props };
}

function type(input: HTMLInputElement, text: string): void {
  input.value = text;
  input.dispatchEvent(new Event("input", { bubbles: true }));
  flushSync();
}

function press(input: HTMLInputElement, key: string): void {
  input.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true }));
  flushSync();
}

function labels(host: HTMLElement): string[] {
  return [...host.querySelectorAll("button.opt .name")].map((n) => n.textContent ?? "");
}

afterEach(() => {
  if (app) unmount(app);
  app = null;
  document.body.innerHTML = "";
});

describe("SearchSelect", () => {
  it("opens on click with group headers, every entry, and the pinned tail", () => {
    const { host, input } = mountSelect();
    expect(host.querySelector(".menu")).toBeNull();
    input.click();
    flushSync();
    const heads = [...host.querySelectorAll(".grouphead")].map((h) => h.textContent);
    expect(heads).toEqual(["common", "all languages"]);
    expect(labels(host)).toEqual(["English", "Spanish", "English", "Mooré", "Spanish", "Other IRI…"]);
  });

  it("filters by label, dedupes across groups, and keeps the pinned entry", () => {
    const { host, input } = mountSelect();
    type(input, "span");
    expect(labels(host)).toEqual(["Spanish", "Other IRI…"]);
    expect(host.querySelector(".grouphead")).toBeNull();
  });

  it("matches the MARC code and folds diacritics", () => {
    const { host, input } = mountSelect();
    type(input, "mos");
    expect(labels(host)[0]).toBe("Mooré");
    type(input, "moore");
    expect(labels(host)[0]).toBe("Mooré");
  });

  it("Enter picks the highlighted match and closes", () => {
    const onchange = vi.fn();
    const { host, input, props } = mountSelect({ onchange });
    type(input, "spanish");
    press(input, "Enter");
    expect(onchange).toHaveBeenLastCalledWith("iri/spa");
    expect(props.value).toBe("iri/spa");
    expect(host.querySelector(".menu")).toBeNull();
    expect(input.value).toBe("Spanish");
  });

  it("arrows move the highlight before Enter picks", () => {
    const { input, props } = mountSelect();
    press(input, "ArrowDown"); // opens on the current (empty) pick
    press(input, "ArrowDown");
    press(input, "Enter");
    expect(props.value).toBe("iri/spa");
  });

  it("clicking an option picks it", () => {
    const { host, input, props } = mountSelect();
    input.click();
    flushSync();
    const other = [...host.querySelectorAll<HTMLButtonElement>("button.opt")].find((b) => b.textContent?.includes("Other IRI…"))!;
    other.click();
    flushSync();
    expect(props.value).toBe("__custom__");
    expect(input.value).toBe("Other IRI…");
  });

  it("typing clears a prior pick; Escape closes and reads the pick back", () => {
    const { host, input, props } = mountSelect({ value: "iri/eng" });
    expect(input.value).toBe("English");
    type(input, "Engl");
    expect(props.value).toBe("");
    press(input, "Escape");
    expect(host.querySelector(".menu")).toBeNull();
    expect(input.value).toBe(""); // the pick was cleared, so the box empties
  });

  it("reopening with a pick shows the full list, not a one-item filter", () => {
    const { host, input } = mountSelect({ value: "iri/eng" });
    input.click();
    flushSync();
    expect(labels(host).length).toBe(6);
  });

  it("an external reset (form submit) empties the box", () => {
    const { input, props } = mountSelect({ value: "iri/eng" });
    expect(input.value).toBe("English");
    props.value = "";
    flushSync();
    expect(input.value).toBe("");
  });

  it("blur closes the menu; a no-match query says so when nothing is pinned", () => {
    const { host, input } = mountSelect({ pinned: [] });
    type(input, "zzzz");
    expect(host.querySelector(".none")?.textContent).toBe("no matches");
    input.dispatchEvent(new FocusEvent("blur"));
    flushSync();
    expect(host.querySelector(".menu")).toBeNull();
  });
});
