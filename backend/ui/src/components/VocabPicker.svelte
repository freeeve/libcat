<script lang="ts">
  // Modal vocabulary picker: scheme tabs over config.schemes (folk excluded),
  // search-as-you-type against /v1/terms, arrow-key result navigation, and a
  // details pane (definition, alt labels, semantic neighborhood) for the
  // highlighted term. Focus is trapped while open; Escape closes and focus
  // returns to the opener. Enter or click emits the term through onselect.
  import { onMount } from "svelte";
  import { searchTerms } from "../lib/api";
  import { getConfig } from "../lib/config";
  import { popScope, pushScope } from "../lib/keyboard";
  import { allAltLabels, bestDefinition, bestLabel } from "../lib/vocab";
  import NeighborhoodPanel from "./NeighborhoodPanel.svelte";
  import type { Term } from "../lib/types";

  let {
    title = "Pick a term",
    onselect,
    onclose,
  }: {
    title?: string;
    onselect: (term: Term) => void;
    onclose: () => void;
  } = $props();

  const SCOPE = "picker";
  const DEBOUNCE_MS = 200;
  const schemes = (getConfig().schemes ?? []).filter((s) => s !== "folk");

  let scheme = $state(schemes[0] ?? "");
  let q = $state("");
  let results = $state<Term[]>([]);
  let highlight = $state(0);
  let searching = $state(false);
  let error = $state("");
  let panel = $state<HTMLElement | null>(null);
  let inputEl = $state<HTMLInputElement | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;

  const current = $derived(results[highlight]);

  onMount(() => {
    const opener = document.activeElement as HTMLElement | null;
    pushScope(SCOPE); // silences the screen's bindings while the modal is up
    inputEl?.focus();
    return () => {
      popScope(SCOPE);
      clearTimeout(timer);
      opener?.focus?.();
    };
  });

  function setScheme(s: string): void {
    scheme = s;
    void search(q);
    inputEl?.focus();
  }

  function onInput(): void {
    clearTimeout(timer);
    timer = setTimeout(() => void search(q), DEBOUNCE_MS);
  }

  async function search(query: string): Promise<void> {
    if (!scheme || query.trim() === "") {
      results = [];
      highlight = 0;
      return;
    }
    searching = true;
    error = "";
    try {
      const res = await searchTerms(scheme, query);
      results = res.terms ?? [];
      highlight = 0;
    } catch {
      results = [];
      error = "term search failed";
    } finally {
      searching = false;
    }
  }

  function onInputKeydown(ev: KeyboardEvent): void {
    if (ev.key === "ArrowDown") {
      ev.preventDefault();
      move(1);
    } else if (ev.key === "ArrowUp") {
      ev.preventDefault();
      move(-1);
    } else if (ev.key === "Enter") {
      ev.preventDefault();
      if (current) onselect(current);
    }
  }

  function move(delta: number): void {
    if (results.length === 0) return;
    highlight = Math.min(results.length - 1, Math.max(0, highlight + delta));
    document.getElementById(`vp-opt-${highlight}`)?.scrollIntoView({ block: "nearest" });
  }

  /** Focus trap: Tab cycles inside the dialog, Escape closes it. */
  function onPanelKeydown(ev: KeyboardEvent): void {
    if (ev.key === "Escape") {
      ev.stopPropagation();
      onclose();
      return;
    }
    if (ev.key !== "Tab" || !panel) return;
    const focusables = panel.querySelectorAll<HTMLElement>('button, input, [tabindex]:not([tabindex="-1"])');
    if (focusables.length === 0) return;
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    if (ev.shiftKey && document.activeElement === first) {
      ev.preventDefault();
      last.focus();
    } else if (!ev.shiftKey && document.activeElement === last) {
      ev.preventDefault();
      first.focus();
    }
  }
</script>

<div class="scrim">
  <div class="panel" role="dialog" aria-modal="true" aria-label={title} tabindex="-1" bind:this={panel} onkeydown={onPanelKeydown}>
    <header class="head">
      <h2>{title}</h2>
      <button class="button button--quiet" onclick={onclose}>Close</button>
    </header>

    {#if schemes.length === 0}
      <p class="muted">No controlled vocabularies are loaded on this deployment.</p>
    {:else}
      <div class="tabs" role="group" aria-label="Vocabulary scheme">
        {#each schemes as s (s)}
          <button class="tab" class:active={s === scheme} aria-pressed={s === scheme} onclick={() => setScheme(s)}>
            {s}
          </button>
        {/each}
      </div>

      <label class="muted" for="vp-q">Search {scheme}</label>
      <input
        id="vp-q"
        type="search"
        bind:this={inputEl}
        bind:value={q}
        oninput={onInput}
        onkeydown={onInputKeydown}
        autocomplete="off"
        placeholder="Type to search…"
      />
      <p class="muted status" aria-live="polite">
        {#if searching}
          Searching…
        {:else if error}
          <span class="error">{error}</span>
        {:else if q.trim() && results.length === 0}
          No matches.
        {:else if results.length > 0}
          {results.length} match{results.length === 1 ? "" : "es"} -- arrows to highlight, Enter to pick
        {:else}
          Type to search.
        {/if}
      </p>

      <div class="cols">
        <ul class="options" aria-label="Matching terms">
          {#each results as t, i (t.id)}
            <li id={"vp-opt-" + i} class:highlight={i === highlight}>
              <button class="opt" onclick={() => onselect(t)} onfocus={() => (highlight = i)}>
                <span class="opt-label">{bestLabel(t)}</span>
                <span class="opt-id">{t.id}</span>
              </button>
            </li>
          {/each}
        </ul>

        {#if current}
          <aside class="details" aria-label="Term details">
            <h3>{bestLabel(current)}</h3>
            <p class="opt-id">{current.id}</p>
            {#if bestDefinition(current)}
              <p class="def">{bestDefinition(current)}</p>
            {/if}
            {#if allAltLabels(current).length > 0}
              <p class="alt"><span class="muted">Also known as:</span> {allAltLabels(current).join("; ")}</p>
            {/if}
            {#key current.scheme + " " + current.id}
              <NeighborhoodPanel term={current} {onselect} />
            {/key}
          </aside>
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: rgba(20, 22, 25, 0.55);
    display: grid;
    place-items: center;
    z-index: 40;
  }
  .panel {
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: 8px;
    padding: 1rem 1.25rem 1.25rem;
    width: min(52rem, 94vw);
    max-height: 85vh;
    overflow-y: auto;
  }
  .head {
    display: flex;
    align-items: baseline;
    gap: 1rem;
  }
  .head h2 {
    margin: 0.25rem 0;
    flex: 1;
  }
  .tabs {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
    margin: 0.5rem 0 0.75rem;
  }
  .tab {
    background: var(--surface);
    border: 1px solid var(--rule);
    border-radius: 999px;
    padding: 0.2em 0.9em;
    color: var(--ink-muted);
    font-size: 0.85rem;
    font-weight: 600;
  }
  .tab.active {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-ink);
  }
  label {
    display: block;
    font-size: 0.85rem;
    margin-bottom: 0.2rem;
  }
  #vp-q {
    width: 100%;
    font-size: 1rem;
  }
  .status {
    margin: 0.35rem 0;
    font-size: 0.85rem;
  }
  .cols {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
    align-items: start;
  }
  @media (max-width: 40rem) {
    .cols {
      grid-template-columns: 1fr;
    }
  }
  .options {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 22rem;
    overflow-y: auto;
  }
  .options li {
    border: 1px solid transparent;
    border-bottom-color: var(--rule);
  }
  .options li.highlight {
    border-color: var(--accent);
    border-radius: 4px;
    background: var(--surface);
  }
  .opt {
    display: block;
    width: 100%;
    text-align: left;
    background: none;
    border: 0;
    padding: 0.4rem 0.5rem;
    color: inherit;
  }
  .opt-label {
    display: block;
    font-weight: 600;
    color: var(--accent);
  }
  .opt-id {
    display: block;
    font-family: var(--mono);
    font-size: 0.72rem;
    color: var(--ink-muted);
    word-break: break-all;
    margin: 0.1rem 0;
  }
  .details {
    border: 1px solid var(--rule);
    border-radius: 6px;
    padding: 0.6rem 0.9rem;
    max-height: 22rem;
    overflow-y: auto;
  }
  .details h3 {
    margin: 0.2rem 0;
  }
  .def {
    font-size: 0.9rem;
  }
  .alt {
    font-size: 0.85rem;
  }
</style>
