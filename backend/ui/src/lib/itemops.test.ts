import { describe, expect, it } from "vitest";
import { ITEM_FIELDS, fieldKey, guardValue, isItemField, itemOp, itemOpSummary, parseFieldKey } from "./itemops";

describe("field keys", () => {
  it("round-trips a resource and path", () => {
    expect(parseFieldKey(fieldKey("items", "location"))).toEqual({ resource: "items", path: "location" });
    expect(parseFieldKey(fieldKey("work", "tags"))).toEqual({ resource: "work", path: "tags" });
  });

  // Work field paths are dotted, not colon-separated, but a bare path from an
  // older saved row must still read as a work field rather than a resource.
  it("reads a bare path as a work field", () => {
    expect(parseFieldKey("tags")).toEqual({ resource: "work", path: "tags" });
    expect(isItemField("tags")).toBe(false);
    expect(isItemField("items:note")).toBe(true);
  });
});

describe("guardValue", () => {
  // The wire distinguishes "every item" (no guard) from "the items missing
  // this field" (guard of ""). Collapsing them would make a fill-in-the-blanks
  // edit overwrite every copy.
  it("tells 'every item' apart from 'the empty ones'", () => {
    expect(guardValue("all", "Stacks")).toBeUndefined();
    expect(guardValue("empty", "Stacks")).toBe("");
    expect(guardValue("eq", "Stacks")).toBe("Stacks");
  });
});

describe("itemOp", () => {
  it("builds a guarded relocation", () => {
    expect(itemOp("location", "set", "Annex", "eq", "Stacks")).toEqual({
      resource: "items",
      path: "location",
      action: "set",
      values: [{ v: "Annex" }],
      where: "Stacks",
    });
  });

  it("omits the guard when the edit reaches every item", () => {
    const op = itemOp("location", "set", "Annex", "all", "Stacks");
    expect(op).toEqual({ resource: "items", path: "location", action: "set", values: [{ v: "Annex" }] });
    expect(op && "where" in op).toBe(false);
  });

  it("clears without a value", () => {
    expect(itemOp("note", "clear", "", "all", "")).toEqual({ resource: "items", path: "note", action: "clear" });
    expect(itemOp("note", "clear", "", "empty", "")).toEqual({ resource: "items", path: "note", action: "clear", where: "" });
  });

  // A `set` with no value would clear the field. The server refuses it; the UI
  // declines to send it, so the row simply stays unstaged until it says
  // something.
  it("refuses a set with no value, and a row with no field", () => {
    expect(itemOp("location", "set", "", "all", "")).toBeNull();
    expect(itemOp("", "set", "Annex", "all", "")).toBeNull();
  });

  it("refuses add and remove, which an item field cannot mean", () => {
    expect(itemOp("location", "add", "Annex", "all", "")).toBeNull();
    expect(itemOp("location", "remove", "Annex", "all", "")).toBeNull();
  });
});

describe("itemOpSummary", () => {
  it("says which copies the edit reaches", () => {
    expect(itemOpSummary(itemOp("location", "set", "Annex", "eq", "Stacks")!)).toBe(
      "set shelving location to “Annex” on items where shelving location is “Stacks”",
    );
    expect(itemOpSummary(itemOp("location", "set", "Annex", "all", "")!)).toBe("set shelving location to “Annex” on every item");
    expect(itemOpSummary(itemOp("callNumber", "set", "FIC", "empty", "")!)).toBe("set call number to “FIC” on items with no call number");
    expect(itemOpSummary(itemOp("note", "clear", "", "all", "")!)).toBe("clear item note on every item");
  });
});

describe("ITEM_FIELDS", () => {
  // A barcode names one physical copy. Batch-assigning it would mint
  // duplicates; the server refuses it, and the picker must not offer it.
  it("does not offer barcode", () => {
    expect(ITEM_FIELDS.map((f) => f.path)).toEqual(["callNumber", "location", "note"]);
  });
});
