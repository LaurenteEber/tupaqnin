import { useEffect, useState } from "react";
import { actOnProposal, listProposals } from "../api/client";
import type { ProposalView } from "../api/graph";
import { translate, type Lang } from "../i18n";
import { closedKey, heldBackKey, proposalsToShow, turnToWatch } from "../proposals";

// The proposals in the document (backlog-r4 4.1, pantallas «Dónde va cada
// vista de la rebanada 4»): with a section, the proposed text marked against
// the current one, its contrast folded, «Aceptar» and «Descartar», and a line
// for each closed one (B-76: a la vista); without one, the proposed sections
// in grey at the end of the document. Go computes all of it; what the agent
// wrote is shown literal (rule 8).
export function ProposalPanel({
  lang,
  projectID,
  section,
  refreshKey,
  readOnly,
  onChanged,
}: {
  lang: Lang;
  projectID: string;
  section?: string;
  refreshKey: string;
  readOnly: boolean;
  // Told what changed, with the turn Go opened for a new step, or "" (B-75).
  onChanged: (turn: string) => void;
}) {
  const t = (key: string, vars?: Record<string, string>) => translate(lang, key, vars);
  const [items, setItems] = useState<ProposalView[]>([]);
  const [failed, setFailed] = useState(false);

  async function load() {
    const all = await listProposals(projectID, section);
    setItems(proposalsToShow(all, section));
  }
  useEffect(() => {
    void load().catch(() => setItems([]));
  }, [projectID, section, refreshKey]);

  async function act(label: string, verb: "accept" | "discard") {
    setFailed(false);
    try {
      const res = await actOnProposal(projectID, label, verb);
      await load();
      onChanged(turnToWatch(res));
    } catch {
      setFailed(true);
    }
  }

  if (items.length === 0) return null;
  return (
    <div className="proposals">
      {failed && <p className="meta">{t("proposal.error")}</p>}
      {items.map((p) =>
        p.state !== "open" ? (
          <p key={p.label} className="proposal-closed">
            {t("proposal.title")} · {t(closedKey(p))}: {p.text}
          </p>
        ) : (
          <div key={p.label} className={p.new_section ? "proposal proposal-new" : "proposal"}>
            <b>{p.new_section ? t("proposal.new_section", { title: p.new_section.title }) : t("proposal.title")}</b>
            {/* B-99: why the app did not apply it. */}
            {heldBackKey(p) && <p className="meta" role="status">{t(heldBackKey(p))}</p>}
            {!p.new_section && <span className="meta">{t(`proposal.origin.${p.origin}`)}</span>}
            {p.version_changed && <span className="meta">{t("proposal.version_changed")}</span>}
            {p.contrast.removed.map((text, i) => (
              <p key={`r${i}`}>
                <span className="meta">{t("proposal.leaves")}: </span>
                <del>{text}</del>
              </p>
            ))}
            {p.contrast.added.map((text, i) => (
              <p key={`a${i}`}>
                <span className="meta">{t("proposal.enters")}: </span>
                <ins>{text}</ins>
              </p>
            ))}
            <details>
              <summary>{t("proposal.contrast")}</summary>
              {p.contrast.triage_line && <p>{p.contrast.triage_line}</p>}
              {p.contrast.note && <p>{p.contrast.note}</p>}
              {p.contrast.decisions.length > 0 && (
                <ul>
                  {p.contrast.decisions.map((d) => (
                    <li key={d.label}>
                      {d.statement} · {t("proposal.instead_of", { text: d.instead_of })}
                    </li>
                  ))}
                </ul>
              )}
            </details>
            {!readOnly && (
              <div className="proposal-actions">
                <button type="button" onClick={() => void act(p.label, "accept")}>
                  {t("proposal.accept")}
                </button>
                <button type="button" onClick={() => void act(p.label, "discard")}>
                  {t("proposal.discard")}
                </button>
              </div>
            )}
          </div>
        ),
      )}
    </div>
  );
}
