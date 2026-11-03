// Something in the understanding still waits for your answer: a case, a
// proposed mark or the plan. Each state comes from Go; this only adds them up.
export function understandingUnconfirmed(view: {
  plan_confirmed: boolean;
  cases: { state: string }[];
  sections: { role_state: string }[];
}): boolean {
  return (
    view.cases.some((c) => c.state === "unconfirmed") ||
    view.sections.some((s) => s.role_state === "proposed") ||
    (!view.plan_confirmed && view.sections.length > 0)
  );
}
