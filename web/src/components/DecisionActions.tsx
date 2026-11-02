import { useState } from "react";
import { objectDecision, revokeDecision } from "../api/client";
import { translate, type Lang } from "../i18n";

// «Objetar» and «Revocar» on an expanded decision (E-1, B-81, B-82), in the
// frame and on its card. Go says which acts it has and whether the reason counts.
export function DecisionActions({ lang, projectID, decision, canObject, canRevoke, onActed }: {
  lang: Lang;
  projectID: string;
  decision: string;
  canObject?: boolean;
  canRevoke?: boolean;
  onActed: () => void;
}) {
  const t = (key: string, params?: Record<string, string>) => translate(lang, key, params);
  const [act, setAct] = useState<"object" | "revoke" | null>(null);
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");

  if (!canObject && !canRevoke) return null;

  async function apply() {
    try {
      if (act === "object") await objectDecision(projectID, decision, reason);
      else await revokeDecision(projectID, decision, reason);
      setAct(null);
      setReason("");
      setError("");
      onActed();
    } catch (e) {
      const status = e instanceof Error ? e.message : "";
      setError(status === "http_422" ? "invalid_reason" : status === "http_409" ? "closed" : "failed");
      if (status === "http_409") onActed();
    }
  }

  return (
    <div className="decision-acts">
      {canObject && (
        <button type="button" className="link" onClick={() => { setAct("object"); setError(""); }}>
          {t("decision.object")}
        </button>
      )}
      {canRevoke && (
        <button type="button" className="link" onClick={() => { setAct("revoke"); setError(""); }}>
          {t("decision.revoke")}
        </button>
      )}
      {act && (
        <div>
          <label>
            {t(act === "object" ? "decision.object_reason" : "decision.revoke_reason")}
            <input value={reason} onChange={(e) => setReason(e.target.value)} />
          </label>
          <button type="button" className="small" onClick={() => void apply()}>
            {t(act === "object" ? "decision.object" : "decision.revoke")}
          </button>
          <button type="button" className="link" onClick={() => { setAct(null); setError(""); }}>
            {t("decision.cancel")}
          </button>
        </div>
      )}
      {error && <p className="turn-error">{t(`decision.error.${error}`)}</p>}
    </div>
  );
}
