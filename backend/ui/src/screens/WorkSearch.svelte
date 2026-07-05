<script lang="ts">
  // Debounced search over /v1/works with keyboard-navigable results:
  // RowList carries j/k/arrows and Enter-to-open; "/" refocuses the box.
  // Query, rows, and selection live in screenState so returning from an
  // editor lands on the same row; a stale list refetches in the background
  // and re-finds the selected work by id.
  import { onMount } from "svelte";
  import { fetchWorks, ApiError } from "../lib/api";
  import { bindKeys, pushScope, popScope } from "../lib/keyboard";
  import { navigate } from "../lib/router";
  import { screenState } from "../lib/screenState.svelte";
  import { sequencer } from "../lib/sequence";
  import RowList from "../components/RowList.svelte";
  import type { WorkSummary } from "../lib/types";

  const SCOPE = "works";
  const DEBOUNCE_MS = 250;
  const FRESH_MS = 60_000;
  const seq = sequencer();

  const st = screenState("works", () => ({
    q: "",
    works: [] as WorkSummary[],
    total: 0,
    matched: 0,
    selected: 0,
    loadedAt: 0,
  }));

  let error = $state("");
  let loading = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;

  onMount(() => {
    pushScope(SCOPE);
    const unbind = bindKeys(SCOPE, {
      "/": { description: "focus the search box", legend: "search", handler: focusSearch },
      m: { description: "load more results", legend: "more", handler: () => void loadMore() },
    });
    if (Date.now() - st.loadedAt > FRESH_MS) void search(st.q, true);
    return () => {
      unbind();
      popScope(SCOPE);
      clearTimeout(timer);
    };
  });

  function onInput(): void {
    clearTimeout(timer);
    timer = setTimeout(() => void search(st.q, false), DEBOUNCE_MS);
  }

  /** Runs the search; a refresh keeps the selection pinned to the same
      work id, a new query starts back at the top. */
  async function search(query: string, refresh: boolean): Promise<void> {
    const t = seq.take();
    loading = true;
    error = "";
    const keepId = refresh ? st.works[st.selected]?.WorkID : undefined;
    try {
      const page = await fetchWorks(query);
      if (t.stale) return;
      st.works = page.works ?? [];
      st.total = page.total;
      st.matched = page.matched ?? st.works.length;
      st.loadedAt = Date.now();
      const found = keepId ? st.works.findIndex((w) => w.WorkID === keepId) : -1;
      st.selected = found >= 0 ? found : Math.min(st.selected, Math.max(0, st.works.length - 1));
      if (!refresh) st.selected = 0;
    } catch (e) {
      if (t.stale) return;
      st.works = [];
      error = e instanceof ApiError && e.status === 401 ? "session expired -- sign in again" : "search failed";
    } finally {
      if (!t.stale) loading = false;
    }
  }

  /** Appends the next window of matches; selection stays put. */
  async function loadMore(): Promise<void> {
    if (loading || st.works.length >= st.matched) return;
    const t = seq.take();
    loading = true;
    error = "";
    try {
      const page = await fetchWorks(st.q, 50, st.works.length);
      if (t.stale) return;
      const seen = new Set(st.works.map((w) => w.WorkID));
      st.works = [...st.works, ...(page.works ?? []).filter((w) => !seen.has(w.WorkID))];
      st.total = page.total;
      st.matched = page.matched ?? st.matched;
      st.loadedAt = Date.now();
    } catch {
      if (t.stale) return;
      error = "loading more failed";
    } finally {
      if (!t.stale) loading = false;
    }
  }

  function open(w: WorkSummary): void {
    navigate(`/works/${encodeURIComponent(w.WorkID)}`);
  }

  function focusSearch(): void {
    document.getElementById("work-q")?.focus();
  }
</script>

<main class="wide">
  <h1>Work search</h1>
  <p class="lede">
    <label for="work-q" class="muted">Title, contributor, tag, ISBN, or id</label>
  </p>
  <input id="work-q" type="search" bind:value={st.q} oninput={onInput} placeholder="Search works…" autocomplete="off" />
  <p class="muted status" aria-live="polite">
    {#if loading && st.works.length === 0}Searching…{:else if error}<span class="error">{error}</span>{:else}{st.works.length} of {st.matched} matched · {st.total} in catalog{/if}
    {#if !error && st.works.length > 0}
      · <a href={st.q.trim() ? "#/exports?kind=search&q=" + encodeURIComponent(st.q.trim()) : "#/exports?kind=all"}>Export these results…</a>
    {/if}
  </p>

  <RowList items={st.works} bind:selected={st.selected} getKey={(w) => w.WorkID} ariaLabel="Search results" scope={SCOPE} itemName="result" onactivate={open}>
    {#snippet row(w: WorkSummary)}
      <a class="row-link" href={"#/works/" + encodeURIComponent(w.WorkID)} title={w.Tags?.length ? w.Tags.join(", ") : undefined}>
        <span class="title">{w.Title || "(untitled)"}</span>
        <span class="muted who">{w.Contributors?.join("; ") ?? ""}</span>
        <span class="flags">
          {#if w.Tombstoned}<span class="flag" data-kind="tombstoned" title="retired; public search redirects or serves gone">tombstoned</span>{/if}
          {#if w.Suppressed}<span class="flag" data-kind="suppressed" title="hidden from public projection and search">suppressed</span>{/if}
          {#if w.Withdrawn}<span class="flag" data-kind="withdrawn" title={"gone from its feed since " + w.Withdrawn + " (tasks/078)"}>withdrawn</span>{/if}
          {#if !w.Items && !w.HasAvailability && !w.Tombstoned}<span class="flag" data-kind="unheld" title="no items and no live-availability identifier">no holdings</span>{/if}
        </span>
        <span class="id">{w.WorkID}</span>
      </a>
    {/snippet}
  </RowList>

  {#if st.works.length < st.matched}
    <p><button class="button button--quiet" onclick={() => void loadMore()} disabled={loading}>Load more ({st.matched - st.works.length} left)</button></p>
  {/if}
</main>

<style>
  #work-q {
    width: 100%;
    max-width: 28rem;
    font-size: 1rem;
  }
  .lede {
    margin: 0.2rem 0;
  }
  .status {
    margin: 0.35rem 0;
    font-size: var(--fs-meta);
  }
  .row-link {
    display: grid;
    grid-template-columns: minmax(12rem, auto) 1fr auto auto;
    gap: 0 0.9rem;
    align-items: baseline;
    padding: 0.22rem 0.55rem;
    text-decoration: none;
    color: inherit;
  }
  .flags {
    display: inline-flex;
    gap: 0.3rem;
  }
  .flag {
    font-size: 0.68rem;
    font-weight: 650;
    border: 1px solid var(--rule);
    border-radius: 999px;
    padding: 0.02em 0.55em;
    color: var(--ink-muted);
    white-space: nowrap;
  }
  .flag[data-kind="suppressed"],
  .flag[data-kind="tombstoned"] {
    border-color: var(--danger);
    color: var(--danger);
  }
  .flag[data-kind="withdrawn"] {
    border-color: #c77d0a;
    color: #c77d0a;
  }
  .title {
    font-weight: 600;
    color: var(--accent);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .who {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .id {
    font-family: var(--mono);
    font-size: var(--fs-meta);
    color: var(--ink-muted);
  }
</style>
