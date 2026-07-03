import { describe, expect, it } from "vitest";
import { locFieldHelpUrl } from "./lochelp";

describe("locFieldHelpUrl", () => {
  it("links documented bibliographic tags to their bd page", () => {
    expect(locFieldHelpUrl("246")).toBe("https://www.loc.gov/marc/bibliographic/bd246.html");
    expect(locFieldHelpUrl("008")).toBe("https://www.loc.gov/marc/bibliographic/bd008.html");
    expect(locFieldHelpUrl("856")).toBe("https://www.loc.gov/marc/bibliographic/bd856.html");
  });

  it("routes the grouped local ranges to their shared pages", () => {
    expect(locFieldHelpUrl("092")).toBe("https://www.loc.gov/marc/bibliographic/bd09x.html");
    expect(locFieldHelpUrl("599")).toBe("https://www.loc.gov/marc/bibliographic/bd59x.html");
  });

  it("links the leader", () => {
    expect(locFieldHelpUrl("LDR")).toBe("https://www.loc.gov/marc/bibliographic/bdleader.html");
    expect(locFieldHelpUrl("LDR", "authority")).toBe("https://www.loc.gov/marc/authority/adleader.html");
  });

  it("links authority tags to their ad page", () => {
    expect(locFieldHelpUrl("150", "authority")).toBe("https://www.loc.gov/marc/authority/ad150.html");
    expect(locFieldHelpUrl("450", "authority")).toBe("https://www.loc.gov/marc/authority/ad450.html");
  });

  it("returns undefined for local, undefined, and malformed tags", () => {
    expect(locFieldHelpUrl("999")).toBeUndefined();
    expect(locFieldHelpUrl("249")).toBeUndefined();
    expect(locFieldHelpUrl("59")).toBeUndefined();
    expect(locFieldHelpUrl("abc")).toBeUndefined();
    expect(locFieldHelpUrl("945", "authority")).toBeUndefined();
    expect(locFieldHelpUrl("650", "authority")).toBeUndefined();
  });
});
