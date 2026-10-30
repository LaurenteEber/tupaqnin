import type { SummaryView } from "../api/types";
import { modelName, numberLocale, translate, type Lang } from "../i18n";

// U-2: the model data in the conversation header, as Go sends it:
// provider · model · effort · context used · spent of the cap. Only the
// formatting of names and numbers happens here.
export function ModelLine({ lang, summary }: { lang: Lang; summary: SummaryView }) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const locale = numberLocale(lang);
  const money = (n: number, min: number) =>
    new Intl.NumberFormat(locale, { minimumFractionDigits: min, maximumFractionDigits: 2 }).format(n);

  const parts = [
    t(`summary.provider.${summary.provider}`),
    modelName(summary.model),
    t(`summary.effort.${summary.effort}`),
  ];
  if (summary.context_window > 0) {
    parts.push(
      t("summary.context", {
        pct: new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 0 }).format(
          summary.context_used_tokens / summary.context_window,
        ),
        window: new Intl.NumberFormat(locale, { notation: "compact" }).format(summary.context_window),
      }),
    );
  }
  parts.push(t("summary.spent_of", { spent: money(summary.spent_usd, 2), cap: money(summary.cap_usd, 0) }));

  return <span className="meta model-line">{parts.join(" · ")}</span>;
}
