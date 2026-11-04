// The bar's spend notice (B3-20, B-89): Go computes warn and stopped_by;
// this only picks the text. What stops the agent comes before the 80 %.
export function spendNoticeKey(s: { warn: boolean; stopped_by: string }): string | null {
  if (s.stopped_by) return `summary.spend.${s.stopped_by}`;
  return s.warn ? "summary.spend.warn" : null;
}

// C.20: what «Auditar» should cost, as Go estimated it and the bar writes spend.
export function auditCostText(usd: number | undefined, locale: string): { key: string; params: Record<string, string> } | null {
  return usd === undefined ? null : { key: "doc.audit.cost", params: { cost: moneyText(usd, locale) } };
}

// C.26: money on screen, with the locale's decimal separator.
export function moneyText(usd: number, locale: string): string {
  return new Intl.NumberFormat(locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(usd);
}
