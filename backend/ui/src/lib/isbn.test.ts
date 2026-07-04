import { describe, expect, it } from "vitest";
import { normalizeIsbn } from "./isbn";

describe("normalizeIsbn", () => {
  it("strips hyphens, spaces, and surrounding label text", () => {
    expect(normalizeIsbn("978-0-13-468599-1")).toBe("9780134685991");
    expect(normalizeIsbn("ISBN 0 306 40615 2")).toBe("0306406152");
  });

  it("keeps a trailing X check digit and uppercases it", () => {
    expect(normalizeIsbn("080442957x")).toBe("080442957X");
  });

  it("rejects non-ISBN lengths", () => {
    expect(normalizeIsbn("12345")).toBe("");
    expect(normalizeIsbn("")).toBe("");
    expect(normalizeIsbn("97801346859912345")).toBe("");
  });

  it("rejects an X anywhere but the ISBN-10 check digit", () => {
    expect(normalizeIsbn("08X442957X")).toBe("");
    expect(normalizeIsbn("978013468599X")).toBe("");
  });
});
