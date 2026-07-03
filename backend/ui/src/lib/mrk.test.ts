import { describe, expect, it } from "vitest";
import { parseRecord, serializeField, serializeRecord } from "./mrk";
import type { MarcRecordDoc } from "./types";

const doc: MarcRecordDoc = {
  node: "#w1Instance",
  leader: "00000nam a2200000 a 4500",
  fields: [
    { tag: "001", value: "X1" },
    { tag: "008", value: "210423s2021    nyu           000 1 eng  " },
    { tag: "245", ind1: "1", ind2: "4", subfields: [{ code: "a", value: "The Dutch house :" }, { code: "b", value: "a novel /" }] },
    { tag: "500", ind1: " ", ind2: " ", subfields: [{ code: "a", value: "Note." }], lossy: "kept verbatim" },
  ],
};

describe("mrk serializer/parser", () => {
  it("serializes leader, control, and data lines", () => {
    const text = serializeRecord(doc);
    expect(text.split("\n")[0]).toBe("LDR 00000nam a2200000 a 4500");
    expect(text).toContain("001 X1");
    expect(text).toContain("245 14 $a The Dutch house : $b a novel /");
    expect(text).toContain("500    $a Note.");
  });

  it("round-trips byte-identically, trailing 008 blanks and order included", () => {
    const text = serializeRecord(doc);
    const { record, errors } = parseRecord(text, doc, { "500": "kept verbatim" });
    expect(errors).toEqual([]);
    expect(record).toEqual(doc);
    expect(serializeRecord(record!)).toBe(text);
  });

  it("keeps the node and leader from base when the buffer has no LDR", () => {
    const { record, errors } = parseRecord("245 10 $a T", doc);
    expect(errors).toEqual([]);
    expect(record?.node).toBe(doc.node);
    expect(record?.leader).toBe(doc.leader);
  });

  it("reports line-anchored errors without returning a record", () => {
    const text = ["LDR x", "24 $a short tag", "245 1! $a bad ind", "246 10", "008 ok", "LDR twice"].join("\n");
    const { record, errors } = parseRecord(text, doc);
    expect(record).toBeUndefined();
    expect(errors.map((e) => e.line)).toEqual([2, 3, 4, 6]);
    expect(errors[0].message).toContain("bad tag");
    expect(errors[1].message).toContain("indicators");
    expect(errors[2].message).toContain("subfield");
    expect(errors[3].message).toContain("duplicate LDR");
  });

  it("skips blank lines and accepts a bare control tag as an empty value", () => {
    const { record, errors } = parseRecord("\n001\n\n245 10 $a T\n", doc);
    expect(errors).toEqual([]);
    expect(record?.fields[0]).toEqual({ tag: "001", value: "" });
    expect(record?.fields).toHaveLength(2);
  });

  it("serializes a single field for clipboard pastes", () => {
    expect(serializeField(doc.fields[2])).toBe("245 14 $a The Dutch house : $b a novel /");
    expect(serializeField({ tag: "007", value: "cr" })).toBe("007 cr");
  });
});
