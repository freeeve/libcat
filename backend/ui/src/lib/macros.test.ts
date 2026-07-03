import { describe, expect, it } from "vitest";
import { applyParams, hasParams } from "./macros";
import type { Macro } from "./types";

const base: Macro = {
  id: "m1",
  label: "Stamp",
  ops: [
    { resource: "work", path: "summary", action: "set", values: [{ v: "${text} (stamped)", lang: "en" }] },
    { resource: "work", path: "subjectLabels", action: "add", value: { v: "plain" } },
  ],
  params: [{ name: "text", label: "Text", default: "No summary" }],
  shared: false,
  owner: "lib@example.org",
  createdAt: "2026-07-01T00:00:00Z",
  updatedAt: "2026-07-01T00:00:00Z",
};

describe("applyParams", () => {
  it("substitutes provided values", () => {
    const ops = applyParams(base, { text: "A space opera" });
    expect(ops[0].values?.[0].v).toBe("A space opera (stamped)");
    expect(ops[1].value?.v).toBe("plain");
  });

  it("falls back to declared defaults and ignores empty inputs", () => {
    expect(applyParams(base, {})[0].values?.[0].v).toBe("No summary (stamped)");
    expect(applyParams(base, { text: "" })[0].values?.[0].v).toBe("No summary (stamped)");
  });

  it("never mutates the macro", () => {
    applyParams(base, { text: "X" });
    expect(base.ops[0].values?.[0].v).toBe("${text} (stamped)");
  });

  it("throws on an unresolved reference", () => {
    const orphan = { ...base, params: [] };
    expect(() => applyParams(orphan, {})).toThrow(/parameter "text"/);
  });

  it("hasParams reflects declarations", () => {
    expect(hasParams(base)).toBe(true);
    expect(hasParams({ ...base, params: [] })).toBe(false);
  });
});
