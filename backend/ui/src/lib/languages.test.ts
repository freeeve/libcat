import { describe, expect, it } from "vitest";
import { LANGUAGES, LANG_TAGS, languageTerm } from "./languages";

describe("languages", () => {
  it("labels the LOC language IRIs the crosswalk emits", () => {
    expect(languageTerm("http://id.loc.gov/vocabulary/languages/eng")?.label).toBe("English");
    expect(languageTerm("http://id.loc.gov/vocabulary/languages/hat")?.label).toBe("Haitian French Creole");
    expect(languageTerm("http://id.loc.gov/vocabulary/languages/zxx")?.label).toBe("No linguistic content");
    expect(languageTerm("http://id.loc.gov/vocabulary/languages/und")?.label).toBe("Undetermined");
  });

  it("returns undefined for unknown IRIs (generic display)", () => {
    expect(languageTerm("http://id.loc.gov/vocabulary/languages/nope")).toBeUndefined();
    expect(languageTerm("https://www.wikidata.org/wiki/Q1860")).toBeUndefined();
  });

  it("fronts a common group before the full alphabetical list", () => {
    const groups = [...new Set(LANGUAGES.map((t) => t.group))];
    expect(groups).toEqual(["common", "all languages"]);
    expect(LANGUAGES[0].code).toBe("eng");
    const all = LANGUAGES.filter((t) => t.group === "all languages");
    expect(all.length).toBeGreaterThan(450);
    const labels = all.map((t) => t.label.toLowerCase());
    expect(labels).toEqual([...labels].sort());
  });

  it("ships well-formed terms and lang tags", () => {
    for (const t of LANGUAGES) {
      expect(t.iri).toBe("http://id.loc.gov/vocabulary/languages/" + t.code);
      expect(t.code).toMatch(/^[a-z]{3}$/);
      expect(t.label).not.toBe("");
    }
    for (const lt of LANG_TAGS) {
      expect(lt.tag).toMatch(/^[a-z]{2}$/);
      expect(lt.label).not.toBe("");
    }
  });
});
