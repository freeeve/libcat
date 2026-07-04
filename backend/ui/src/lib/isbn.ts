// ISBN handling for the quick-add path: cataloger input (scanned, pasted, or
// hyphenated) reduced to a bare search term. We do not enforce the check
// digit -- external targets tolerate a loose match and a wrong digit is the
// cataloger's to notice; we only want a clean token to hand the isbn index.

/**
 * Normalizes a raw ISBN string to bare digits (with a trailing X for ISBN-10
 * check digits), uppercased. Returns "" when the result is not a plausible
 * ISBN-10 or ISBN-13 length, so callers can reject non-ISBN input.
 */
export function normalizeIsbn(raw: string): string {
  const cleaned = raw.replace(/[^0-9Xx]/g, "").toUpperCase();
  if (cleaned.length !== 10 && cleaned.length !== 13) return "";
  // An X may only stand in for an ISBN-10 check digit, i.e. the final char.
  if (cleaned.slice(0, -1).includes("X")) return "";
  if (cleaned.length === 13 && cleaned.includes("X")) return "";
  return cleaned;
}
