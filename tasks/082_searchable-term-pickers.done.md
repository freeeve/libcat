# 082: Searchable term pickers in the work editor

The Language field's native `<select>` carries ~490 options (and carrier
types ~60), which is unusable to scan. Replace the options-driven pickers
in ProfileForm with a searchable combobox.

- New reusable `SearchSelect.svelte`: text input filters a shipped closed
  list (label + MARC code, diacritic-insensitive); entries render as
  buttons in a dropdown menu (TagInput/CommandPalette house pattern, no
  aria-activedescendant); `bind:value` carries the picked entry's value.
- Group headers ("common" / "all languages", carrier media categories)
  survive when unfiltered; filtering flattens and dedupes (common+all
  overlap).
- Pinned entries ("Other IRI…", "no language tag") stay visible under any
  query so free entry keeps working.
- Wire into ProfileForm: language / media / carrier pickers and the
  summary language-tag box.
- Unit tests + keep the axe audit green.
