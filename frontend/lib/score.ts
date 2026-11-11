const SCORE_PATTERN = /^(0|[1-9]\d{0,5})\.\d{2}$/;
const MAX_SCORE_CENTS = 99_999_999;

export function toCents(value: string): number {
  if (!SCORE_PATTERN.test(value)) {
    throw new Error("分值须为不超过 999999.99 的两位小数字符串");
  }
  const [whole, fraction] = value.split(".");
  const cents = Number(whole) * 100 + Number(fraction);
  if (cents <= 0) throw new Error("分值必须大于 0");
  return cents;
}

export function fromCents(value: number): string {
  if (!Number.isSafeInteger(value) || value < 0 || value > MAX_SCORE_CENTS) {
    throw new Error("整数百分分值超出有效范围");
  }
  const whole = Math.floor(value / 100);
  const fraction = String(value % 100).padStart(2, "0");
  return `${whole}.${fraction}`;
}

export function sumScores(values: string[]): string {
  const cents = values.reduce((total, value) => total + toCents(value), 0);
  return fromCents(cents);
}
