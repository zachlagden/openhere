import { describe, expect, it } from "vitest";
import { splitBytes, toBytes } from "./units";

describe("byte units", () => {
  it("uses gigabytes only when the value is a whole number of them", () => {
    expect(splitBytes(1_073_741_824)).toEqual({ amount: 1, unit: "GB" });
    expect(splitBytes(1_610_612_736)).toEqual({ amount: 1536, unit: "MB" });
    expect(splitBytes(268_435_456)).toEqual({ amount: 256, unit: "MB" });
  });

  it("round-trips", () => {
    for (const bytes of [268_435_456, 1_073_741_824, 5_368_709_120, 10_485_760]) {
      const { amount, unit } = splitBytes(bytes);
      expect(toBytes(amount, unit)).toBe(bytes);
    }
  });
});
