// C.12: a section with no text cannot be signed. Go refuses it (409 section_empty);
// the page shows the same reason without waiting for the refusal.
export function sectionHasNoText(section: { text?: string } | undefined): boolean {
  return section !== undefined && (section.text ?? "").trim() === "";
}

// What «Ask the agent to write it» sends: a message of yours, queued if a turn runs.
export function askToWriteParams(section: { title: string }): Record<string, string> {
  return { section: section.title };
}
