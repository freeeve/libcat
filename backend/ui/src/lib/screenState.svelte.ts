// Keyed module-level $state objects so list screens keep their query,
// results, and selection across route remounts -- drilling into a record and
// returning lands on the same row instead of a cold reload. Screens decide
// their own freshness policy (a loadedAt field and a background refetch).
// Reset wholesale on sign-out so nothing leaks across sessions.
const states = new Map<string, unknown>();

/** The state object for key, created from init() on first use. */
export function screenState<T extends object>(key: string, init: () => T): T {
  let s = states.get(key) as T | undefined;
  if (s === undefined) {
    const fresh = $state(init());
    states.set(key, fresh);
    s = fresh;
  }
  return s;
}

/** Drops every screen's kept state (sign-out). */
export function resetScreenStates(): void {
  states.clear();
}
