// screenState keeps one keyed object per screen across remounts and drops
// everything on reset (sign-out).
import { describe, expect, it } from "vitest";
import { resetScreenStates, screenState } from "./screenState.svelte";

describe("screenState", () => {
  it("returns the same object for repeated calls", () => {
    const a = screenState("t1", () => ({ n: 1 }));
    a.n = 42;
    const b = screenState("t1", () => ({ n: 1 }));
    expect(b.n).toBe(42);
  });

  it("keys are independent", () => {
    const a = screenState("t2", () => ({ n: 1 }));
    const b = screenState("t3", () => ({ n: 2 }));
    a.n = 9;
    expect(b.n).toBe(2);
  });

  it("reset drops every kept state", () => {
    const a = screenState("t4", () => ({ n: 1 }));
    a.n = 5;
    resetScreenStates();
    const b = screenState("t4", () => ({ n: 1 }));
    expect(b.n).toBe(1);
  });
});
