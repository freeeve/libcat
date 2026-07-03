import { describe, expect, it } from "vitest";
import { lineToSubfields, subfieldsToLine, slotValue, withSlotValue, F008_SLOTS } from "./marc";

describe("subfield line syntax", () => {
  it("round-trips structured subfields", () => {
    const subs = [
      { code: "a", value: "Gideon the Ninth" },
      { code: "b", value: "a novel /" },
      { code: "c", value: "Tamsyn Muir." },
    ];
    expect(lineToSubfields(subfieldsToLine(subs))).toEqual(subs);
  });

  it("treats delimiter-less text as $a", () => {
    expect(lineToSubfields("Plain title")).toEqual([{ code: "a", value: "Plain title" }]);
  });

  it("keeps leading text before the first delimiter as $a", () => {
    expect(lineToSubfields("Lead $b rest")).toEqual([
      { code: "a", value: "Lead" },
      { code: "b", value: "rest" },
    ]);
  });

  it("drops empty runs and handles empty input", () => {
    expect(lineToSubfields("")).toEqual([]);
    expect(lineToSubfields("$a $b kept")).toEqual([{ code: "b", value: "kept" }]);
  });
});

describe("fixed-field slots", () => {
  const lang = F008_SLOTS.find((s) => s.label === "Language")!;
  it("reads runs with padding", () => {
    expect(slotValue("240702s2026", lang)).toBe("   ");
    expect(slotValue("240702s2026    nyu           000 1 eng d".padEnd(40), lang)).toBe("eng");
  });
  it("writes runs preserving neighbors", () => {
    const v = "240702s2026    nyu           000 1 eng d";
    const out = withSlotValue(v, lang, "spa");
    expect(slotValue(out, lang)).toBe("spa");
    expect(out.slice(0, 35)).toBe(v.slice(0, 35));
    expect(out.slice(38)).toBe(v.slice(38));
  });
  it("clips overlong runs", () => {
    expect(slotValue(withSlotValue("", lang, "english"), lang)).toBe("eng");
  });
});
