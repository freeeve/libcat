// Client-side staging for review decisions: queue rows accumulate
// approve/reject choices locally and the publish bar ships them as one
// POST /v1/review batch. A decision is keyed by suggestion identity
// (work + term + type): staging a different action replaces the entry,
// re-staging the identical action toggles it off.
import { writable, type Readable } from "svelte/store";
import type { Decision, SuggType, TermRef } from "./types";

export interface DecisionStore extends Readable<Decision[]> {
  /** Stages d, replacing any prior decision for the same suggestion; the
   *  identical action twice unstages (toggle). */
  stage(d: Decision): void;
  unstage(workId: string, term: TermRef, type: SuggType): void;
  clear(): void;
  /** The exact wire-shape decision list for POST /v1/review. */
  payload(): Decision[];
}

/** Identity of one suggestion within the staging map. */
export function decisionKey(workId: string, term: TermRef, type: SuggType): string {
  return [workId, term.scheme, term.id, type].join("\u0000");
}

function sameAction(a: Decision, b: Decision): boolean {
  return (
    a.approve === b.approve &&
    !!a.tombstone === !!b.tombstone &&
    (a.substituteTerm?.scheme ?? "") === (b.substituteTerm?.scheme ?? "") &&
    (a.substituteTerm?.id ?? "") === (b.substituteTerm?.id ?? "")
  );
}

/** Wire shape for one decision: optional fields present only when set. */
function shapeWire(d: Decision): Decision {
  const out: Decision = { workId: d.workId, term: d.term, type: d.type, approve: d.approve };
  if (d.substituteTerm) out.substituteTerm = d.substituteTerm;
  if (d.note) out.note = d.note;
  if (d.tombstone) out.tombstone = true;
  return out;
}

/** A fresh staging store (one per queue screen mount). */
export function createDecisionStore(): DecisionStore {
  const staged = new Map<string, Decision>();
  const { subscribe, set } = writable<Decision[]>([]);
  const sync = (): void => set([...staged.values()]);
  return {
    subscribe,
    stage(d: Decision): void {
      const key = decisionKey(d.workId, d.term, d.type);
      const prev = staged.get(key);
      if (prev && sameAction(prev, d)) staged.delete(key);
      else staged.set(key, shapeWire(d));
      sync();
    },
    unstage(workId: string, term: TermRef, type: SuggType): void {
      staged.delete(decisionKey(workId, term, type));
      sync();
    },
    clear(): void {
      staged.clear();
      sync();
    },
    payload(): Decision[] {
      return [...staged.values()];
    },
  };
}
