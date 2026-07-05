import { describe, expect, it } from "vitest";
import { sequencer } from "./sequence";

describe("sequencer", () => {
  it("keeps the only ticket fresh", () => {
    const seq = sequencer();
    const t = seq.take();
    expect(t.stale).toBe(false);
  });

  it("marks earlier tickets stale once a later one is taken", () => {
    const seq = sequencer();
    const a = seq.take();
    const b = seq.take();
    expect(a.stale).toBe(true);
    expect(b.stale).toBe(false);
  });

  it("guards out-of-order async completions", async () => {
    const seq = sequencer();
    const applied: string[] = [];
    const request = async (label: string, delay: number, t = seq.take()): Promise<void> => {
      await new Promise((r) => setTimeout(r, delay));
      if (t.stale) return;
      applied.push(label);
    };
    // The older request resolves after the newer one: only "new" applies.
    await Promise.all([request("old", 20), request("new", 5)]);
    expect(applied).toEqual(["new"]);
  });

  it("scopes independently per sequencer", () => {
    const a = sequencer();
    const b = sequencer();
    const ta = a.take();
    b.take();
    expect(ta.stale).toBe(false);
  });
});
