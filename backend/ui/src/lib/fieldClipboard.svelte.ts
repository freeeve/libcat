// App-level MARC field clipboard (tasks/075): cut/copied fields accumulate
// newest-first and paste back into any editing surface (grid now, text mode
// with tasks/076). Module $state so every consumer shares one clipboard;
// deliberately not persisted -- fields are working material, not documents.
import type { MarcField } from "./types";

const MAX_ENTRIES = 20;

export const fieldClipboard = $state<{ entries: MarcField[] }>({ entries: [] });

/** Adds a field to the top of the clipboard (a plain deep copy, so later
 *  grid edits cannot mutate it). */
export function clipPush(f: MarcField): void {
  fieldClipboard.entries = [structuredClone(f), ...fieldClipboard.entries].slice(0, MAX_ENTRIES);
}

/** The most recent entry, copied for insertion; undefined when empty. */
export function clipPeek(): MarcField | undefined {
  const f = fieldClipboard.entries[0];
  return f ? structuredClone(f) : undefined;
}

/** A specific entry, copied for insertion (the tasks/076 pane picks). */
export function clipAt(i: number): MarcField | undefined {
  const f = fieldClipboard.entries[i];
  return f ? structuredClone(f) : undefined;
}

/** Removes one entry. */
export function clipRemove(i: number): void {
  fieldClipboard.entries = fieldClipboard.entries.filter((_, j) => j !== i);
}

/** Empties the clipboard. */
export function clipClear(): void {
  fieldClipboard.entries = [];
}
