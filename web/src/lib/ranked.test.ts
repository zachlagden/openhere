import { describe, expect, it } from "vitest";
import { barShares } from "./ranked";

describe("barShares", () => {
  it("scales to the largest value", () => {
    expect(barShares([50, 25, 5])).toEqual([100, 50, 10]);
  });

  it("keeps a visible sliver for small values and handles zeros", () => {
    expect(barShares([1000, 1])).toEqual([100, 2]);
    expect(barShares([0, 0])).toEqual([0, 0]);
    expect(barShares([])).toEqual([]);
  });
});
