<script lang="ts" module>
  /** One pickable entry: value is what binds out, code rides along muted
   *  (MARC code, BCP-47 tag), group renders as a header between runs. */
  export interface SearchOption {
    value: string;
    label: string;
    code?: string;
    group?: string;
  }

  let uid = 0;
</script>

<script lang="ts">
  // Searchable stand-in for a long <select>: the closed list ships with the
  // component, the input filters it by label and code (diacritic-
  // insensitive), and entries render as buttons in a dropdown menu -- the
  // TagInput/CommandPalette house pattern, keyboard-driven from the input.
  // The bound value is the picked entry's value; typing clears a prior pick
  // until a new one lands, and an external reset (form submit) clears the
  // box. Group headers survive the unfiltered view; filtering flattens and
  // dedupes (a term listed under "common" and "all" appears once). Pinned
  // entries ("Other IRI…") stay visible under any query.
  let {
    options,
    pinned = [],
    value = $bindable(""),
    placeholder = "",
    ariaLabel,
    onchange,
  }: {
    options: SearchOption[];
    pinned?: SearchOption[];
    value?: string;
    placeholder?: string;
    ariaLabel: string;
    onchange?: (value: string) => void;
  } = $props();

  type Row = { key: string; header: string } | { key: string; opt: SearchOption; index: number };

  const menuId = `searchselect-menu-${++uid}`;

  let q = $state("");
  let open = $state(false);
  let highlight = $state(0);
  let menuEl: HTMLElement | undefined = $state();

  /** Case- and diacritic-insensitive text for matching ("moore" ~ "Mooré"):
   *  NFD splits letters from their combining marks (U+0300-U+036F), which
   *  then drop. */
  const COMBINING = new RegExp("[\\u0300-\\u036f]", "g");
  function fold(s: string): string {
    return s.normalize("NFD").replace(COMBINING, "").toLowerCase();
  }

  const labelByValue = $derived(new Map([...options, ...pinned].map((o) => [o.value, o.label])));

  // Closed, the input reads back the picked entry's label (or empties on an
  // external reset); while open the user's typing owns the box.
  $effect(() => {
    if (!open) q = (value ? labelByValue.get(value) : "") ?? "";
  });

  const rows = $derived.by(() => {
    const query = fold(q.trim());
    const picked = value ? fold(labelByValue.get(value) ?? "") : "";
    const filtering = query.length > 0 && query !== picked;
    const out: Row[] = [];
    let index = 0;
    if (filtering) {
      const seen = new Set<string>();
      for (const o of options) {
        if (seen.has(o.value)) continue;
        if (fold(o.label).includes(query) || (o.code && fold(o.code).includes(query))) {
          seen.add(o.value);
          out.push({ key: "o:" + o.value, opt: o, index: index++ });
        }
      }
    } else {
      let lastGroup: string | undefined;
      for (const o of options) {
        if (o.group && o.group !== lastGroup) out.push({ key: "h:" + o.group, header: o.group });
        lastGroup = o.group;
        out.push({ key: "o:" + (o.group ?? "") + o.value, opt: o, index: index++ });
      }
    }
    for (const o of pinned) out.push({ key: "p:" + o.value, opt: o, index: index++ });
    return out;
  });

  const opts = $derived(rows.filter((r): r is Row & { opt: SearchOption } => "opt" in r).map((r) => r.opt));

  // Keep the keyboard highlight visible as it moves (no-op under jsdom).
  $effect(() => {
    if (!open || highlight < 0) return;
    const el = menuEl?.querySelector("li.highlight");
    if (el && typeof el.scrollIntoView === "function") el.scrollIntoView({ block: "nearest" });
  });

  function openMenu(): void {
    if (open) return;
    open = true;
    highlight = Math.max(
      0,
      opts.findIndex((o) => o.value === value),
    );
  }

  function commit(v: string): void {
    if (v !== value) {
      value = v;
      onchange?.(v);
    }
  }

  function choose(o: SearchOption): void {
    commit(o.value);
    open = false;
  }

  function onType(): void {
    open = true;
    highlight = 0;
    commit("");
  }

  function onKeydown(ev: KeyboardEvent): void {
    if (ev.key === "ArrowDown") {
      ev.preventDefault();
      if (!open) openMenu();
      else highlight = Math.min(opts.length - 1, highlight + 1);
    } else if (ev.key === "ArrowUp" && open) {
      ev.preventDefault();
      highlight = Math.max(0, highlight - 1);
    } else if (ev.key === "Enter" && open) {
      ev.preventDefault();
      ev.stopPropagation();
      const o = opts[highlight];
      if (o) choose(o);
    } else if (ev.key === "Escape" && open) {
      ev.stopPropagation();
      open = false;
    } else if (ev.key === "Tab") {
      open = false;
    }
  }
</script>

<div class="searchselect">
  <input
    type="text"
    role="combobox"
    bind:value={q}
    oninput={onType}
    onclick={openMenu}
    onkeydown={onKeydown}
    onblur={() => (open = false)}
    {placeholder}
    aria-label={ariaLabel}
    aria-expanded={open}
    aria-controls={open ? menuId : undefined}
    aria-autocomplete="list"
    autocomplete="off"
  />
  <span class="caret" aria-hidden="true">▾</span>
  {#if open}
    <ul class="menu" id={menuId} aria-label={ariaLabel + " options"} bind:this={menuEl}>
      {#each rows as row (row.key)}
        {#if "header" in row}
          <li class="grouphead">{row.header}</li>
        {:else}
          <li class:highlight={row.index === highlight}>
            <!-- mousedown is swallowed so the input keeps focus (no blur-
                 before-click race); the click still picks. -->
            <button
              type="button"
              class="opt"
              class:picked={row.opt.value === value && value !== ""}
              onmousedown={(ev) => ev.preventDefault()}
              onclick={() => choose(row.opt)}
              onfocus={() => (highlight = row.index)}
            >
              <span class="name">{row.opt.label}</span>
              {#if row.opt.code}<span class="code">{row.opt.code}</span>{/if}
            </button>
          </li>
        {/if}
      {:else}
        <li class="muted none">no matches</li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .searchselect {
    position: relative;
    display: inline-block;
    min-width: 14rem;
  }
  input {
    width: 100%;
    padding-right: 1.6rem;
  }
  .caret {
    position: absolute;
    right: 0.55rem;
    top: 50%;
    transform: translateY(-50%);
    font-size: 0.7rem;
    color: var(--ink-muted);
    pointer-events: none;
  }
  .menu {
    position: absolute;
    left: 0;
    right: 0;
    top: 100%;
    z-index: 10;
    list-style: none;
    margin: 0.15rem 0 0;
    padding: 0.15rem;
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: 6px;
    box-shadow: 0 4px 14px rgba(20, 22, 25, 0.15);
    max-height: 16rem;
    overflow-y: auto;
  }
  .menu li.highlight {
    background: var(--surface);
    box-shadow: inset 3px 0 0 var(--accent);
  }
  .grouphead {
    font-size: 0.68rem;
    font-weight: 650;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--ink-muted);
    padding: 0.4rem 0.5rem 0.15rem;
  }
  .opt {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.6rem;
    width: 100%;
    text-align: left;
    background: none;
    border: 0;
    padding: 0.25rem 0.5rem;
    color: inherit;
    cursor: pointer;
  }
  .opt .name {
    font-weight: 600;
  }
  .opt.picked .name {
    color: var(--accent);
  }
  .code {
    font-family: var(--mono);
    font-size: 0.72rem;
    color: var(--ink-muted);
  }
  .none {
    padding: 0.3rem 0.5rem;
    font-size: 0.85rem;
  }
</style>
