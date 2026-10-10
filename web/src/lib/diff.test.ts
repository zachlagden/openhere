import { describe, expect, it } from "vitest";
import { changedFields, isDirty } from "./diff";

describe("changedFields", () => {
  it("returns only the fields that differ", () => {
    const original = { name: "Launch", description: "", spa: false };
    expect(changedFields(original, { name: "Launch", description: "Hello", spa: false })).toEqual({ description: "Hello" });
  });

  it("compares arrays by their items", () => {
    expect(changedFields({ list: ["a", "b"] }, { list: ["a", "b"] })).toEqual({});
    expect(changedFields({ list: ["a", "b"] }, { list: ["a", "c"] })).toEqual({ list: ["a", "c"] });
    expect(changedFields({ list: ["a"] }, { list: ["a", "b"] })).toEqual({ list: ["a", "b"] });
  });

  it("reports dirtiness", () => {
    expect(isDirty({ a: 1 }, { a: 1 })).toBe(false);
    expect(isDirty({ a: 1 }, { a: 2 })).toBe(true);
  });
});
