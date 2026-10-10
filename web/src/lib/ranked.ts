export function barShares(values: number[]): number[] {
  const max = values.reduce((best, value) => Math.max(best, value), 0);
  if (max <= 0) {
    return values.map(() => 0);
  }
  return values.map((value) => Math.max(2, Math.round((value / max) * 100)));
}
