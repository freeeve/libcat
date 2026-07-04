import { describe, expect, it } from "vitest";
import { CARRIER_TYPES, MEDIA_TYPES, rdaTerm } from "./rdaterms";

describe("rdaterms", () => {
  it("labels the LOC media and carrier IRIs the crosswalk emits", () => {
    expect(rdaTerm("http://id.loc.gov/vocabulary/carriers/cr")?.label).toBe("online resource");
    expect(rdaTerm("http://id.loc.gov/vocabulary/carriers/sz")?.label).toBe("other audio carrier");
    expect(rdaTerm("http://id.loc.gov/vocabulary/mediaTypes/c")?.label).toBe("computer");
    expect(rdaTerm("http://id.loc.gov/vocabulary/mediaTypes/s")?.label).toBe("audio");
  });

  it("returns undefined for unknown IRIs (generic display)", () => {
    expect(rdaTerm("http://rdaregistry.info/termList/RDAMediaType/1003")).toBeUndefined();
    expect(rdaTerm("not-an-iri")).toBeUndefined();
  });

  it("ships closed, unique lists with codes", () => {
    const all = [...MEDIA_TYPES, ...CARRIER_TYPES];
    expect(MEDIA_TYPES).toHaveLength(10);
    expect(new Set(all.map((t) => t.iri)).size).toBe(all.length);
    for (const t of all) {
      expect(t.iri.endsWith("/" + t.code)).toBe(true);
      expect(t.label).not.toBe("");
    }
  });
});
