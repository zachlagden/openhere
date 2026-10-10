export type ByteUnit = "MB" | "GB";

const FACTORS: Record<ByteUnit, number> = { MB: 1024 ** 2, GB: 1024 ** 3 };

export interface ByteAmount {
  amount: number;
  unit: ByteUnit;
}

export function splitBytes(bytes: number): ByteAmount {
  if (bytes >= FACTORS.GB && bytes % FACTORS.GB === 0) {
    return { amount: bytes / FACTORS.GB, unit: "GB" };
  }
  return { amount: Number((bytes / FACTORS.MB).toFixed(2)), unit: "MB" };
}

export function toBytes(amount: number, unit: ByteUnit): number {
  return Math.round(amount * FACTORS[unit]);
}
