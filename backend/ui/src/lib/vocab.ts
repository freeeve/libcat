// Display selection over a full vocabulary Term's language maps, mirroring
// vocab.Term.Label in Go: exact language, then English, then untagged, then
// anything.
import type { Term } from "./types";

/** The term's best prefLabel for lang; the URI when it has no labels. */
export function bestLabel(t: Term, lang = "en"): string {
  for (const k of [lang, "en", ""]) {
    const l = t.labels?.[k];
    if (l) return l;
  }
  for (const l of Object.values(t.labels ?? {})) return l;
  return t.id;
}

/** The term's best definition/scope note for lang; "" when absent. */
export function bestDefinition(t: Term, lang = "en"): string {
  for (const k of [lang, "en", ""]) {
    const d = t.definition?.[k];
    if (d) return d;
  }
  for (const d of Object.values(t.definition ?? {})) return d;
  return "";
}

/** Every alt (used-for) label across languages, deduplicated. */
export function allAltLabels(t: Term): string[] {
  const out = new Set<string>();
  for (const labels of Object.values(t.altLabels ?? {})) {
    for (const l of labels) out.add(l);
  }
  return [...out];
}
