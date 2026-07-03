import { afterEach, describe, expect, it, vi } from "vitest";
import { get } from "svelte/store";
import {
  activeBindings,
  bindKeys,
  GLOBAL_SCOPE,
  installKeyboard,
  keymapVersion,
  legendBindings,
  normalizeChord,
  popScope,
  pushScope,
  resetKeyboard,
  setHelpPresenter,
} from "./keyboard";

installKeyboard();

function press(key: string, target?: HTMLElement, init: KeyboardEventInit = {}): void {
  const ev = new KeyboardEvent("keydown", { key, bubbles: true, ...init });
  (target ?? document.body).dispatchEvent(ev);
}

afterEach(() => {
  resetKeyboard();
  document.body.innerHTML = "";
});

describe("keyboard scopes", () => {
  it("fires only the top scope plus global", () => {
    const below = vi.fn();
    const top = vi.fn();
    const global = vi.fn();
    bindKeys("search", { a: { description: "below", handler: below } });
    bindKeys("modal", { a: { description: "top", handler: top } });
    bindKeys(GLOBAL_SCOPE, { g: { description: "global", handler: global } });
    pushScope("search");
    pushScope("modal");
    press("a");
    press("g");
    expect(top).toHaveBeenCalledOnce();
    expect(below).not.toHaveBeenCalled();
    expect(global).toHaveBeenCalledOnce();
  });

  it("restores the scope below on pop", () => {
    const below = vi.fn();
    bindKeys("search", { a: { description: "below", handler: below } });
    pushScope("search");
    pushScope("modal");
    popScope("modal");
    press("a");
    expect(below).toHaveBeenCalledOnce();
  });

  it("ignores keys typed into form controls", () => {
    const fn = vi.fn();
    bindKeys(GLOBAL_SCOPE, { a: { description: "x", handler: fn } });
    const input = document.createElement("input");
    document.body.appendChild(input);
    press("a", input);
    expect(fn).not.toHaveBeenCalled();
  });

  it("normalizes meta and ctrl to the same mod chord", () => {
    const metaEv = new KeyboardEvent("keydown", { key: "S", metaKey: true });
    const ctrlEv = new KeyboardEvent("keydown", { key: "s", ctrlKey: true });
    const plainEv = new KeyboardEvent("keydown", { key: "?", shiftKey: true });
    expect(normalizeChord(metaEv)).toBe("mod+s");
    expect(normalizeChord(ctrlEv)).toBe("mod+s");
    expect(normalizeChord(plainEv)).toBe("?");
  });

  it("fires mod chords even while a form control has focus", () => {
    const save = vi.fn();
    const plain = vi.fn();
    bindKeys(GLOBAL_SCOPE, {
      "mod+s": { description: "save", handler: save },
      s: { description: "plain", handler: plain },
    });
    const input = document.createElement("input");
    document.body.appendChild(input);
    press("s", input, { metaKey: true });
    press("s", input);
    expect(save).toHaveBeenCalledOnce();
    expect(plain).not.toHaveBeenCalled();
  });

  it("fires a two-step sequence and swallows the prefix key", () => {
    const go = vi.fn();
    bindKeys(GLOBAL_SCOPE, { "g w": { description: "go to works", handler: go } });
    press("g");
    expect(go).not.toHaveBeenCalled();
    press("w");
    expect(go).toHaveBeenCalledOnce();
  });

  it("drops a sequence prefix after the timeout", () => {
    vi.useFakeTimers();
    const go = vi.fn();
    bindKeys(GLOBAL_SCOPE, { "g w": { description: "go to works", handler: go } });
    press("g");
    vi.advanceTimersByTime(1200);
    press("w");
    expect(go).not.toHaveBeenCalled();
    vi.useRealTimers();
  });

  it("lets an unmatched second key act normally", () => {
    const go = vi.fn();
    const other = vi.fn();
    bindKeys(GLOBAL_SCOPE, {
      "g w": { description: "go to works", handler: go },
      x: { description: "other", handler: other },
    });
    press("g");
    press("x");
    expect(go).not.toHaveBeenCalled();
    expect(other).toHaveBeenCalledOnce();
  });

  it("typing in a form control disarms a pending prefix", () => {
    const go = vi.fn();
    bindKeys(GLOBAL_SCOPE, { "g w": { description: "go to works", handler: go } });
    const input = document.createElement("input");
    document.body.appendChild(input);
    press("g");
    press("a", input);
    press("w");
    expect(go).not.toHaveBeenCalled();
  });

  it("hides alias keys from the legend and collapses sequences", () => {
    bindKeys("works", {
      j: { description: "next", legend: "j/k move", handler: () => {} },
      k: { description: "previous", hidden: true, handler: () => {} },
    });
    bindKeys(GLOBAL_SCOPE, {
      "g w": { description: "go to works", legend: "go to screen", handler: () => {} },
      "g q": { description: "go to the queue", legend: "go to screen", handler: () => {} },
    });
    pushScope("works");
    const keys = legendBindings().map((b) => b.key);
    expect(keys).toEqual(["j", "g …"]);
  });

  it("bumps keymapVersion on scope and binding changes", () => {
    const before = get(keymapVersion);
    const unbind = bindKeys("works", { j: { description: "next", handler: () => {} } });
    pushScope("works");
    popScope("works");
    unbind();
    expect(get(keymapVersion)).toBe(before + 4);
  });

  it("? presents help with the active bindings", () => {
    const shown = vi.fn();
    setHelpPresenter(shown);
    bindKeys("works", { j: { description: "next", handler: () => {} } });
    bindKeys(GLOBAL_SCOPE, { g: { description: "go", handler: () => {} } });
    pushScope("works");
    press("?");
    expect(shown).toHaveBeenCalledOnce();
    const keys = (shown.mock.calls[0][0] as { key: string }[]).map((b) => b.key);
    expect(keys).toEqual(["j", "g"]);
    expect(activeBindings().length).toBe(2);
  });
});
