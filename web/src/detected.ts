import type { DetectedView } from "./api/types";

// One line of what was detected (B-48 b): who, where and what, from
// Go's fields; sections by title, never a label.
export function detectedLine(t: (key: string, params?: Record<string, string>) => string, d: DetectedView): string {
  const section = d.section ? d.section.title || t("section.untitled") : "";
  const who = t(`detected.${d.actor === "human" ? "you" : d.actor}`);
  switch (d.kind) {
    case "edit":
      return d.count > 1
        ? t("detected.edit_n", { who, section, n: String(d.count) })
        : t("detected.edit", { who, section });
    case "section_state":
      return t("detected.section_state", { section, state: t(`section.status.${d.to ?? ""}`) });
    case "decision_state":
      return t("detected.decision_state", { text: d.text ?? "", state: t(`decision.state.${d.to ?? ""}`) });
    default:
      return t(`detected.${d.kind}`, { section, text: d.text ?? "" });
  }
}
