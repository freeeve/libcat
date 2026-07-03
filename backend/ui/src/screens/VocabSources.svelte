<script lang="ts">
  // Vocabularies (tasks/067): the click-to-download authority-source list.
  // Built-in and registered sources show their capabilities (live typeahead,
  // downloadable snapshot), license, and install state; Download fetches the
  // source's SKOS dump into the vocab index (a worker job -- the row polls
  // until it lands), Refresh re-downloads in place, Remove drops the terms.
  import { onMount } from "svelte";
  import { ApiError, downloadVocabSource, fetchVocabSources, removeVocabSnapshot } from "../lib/api";
  import { bindKeys, popScope, pushScope } from "../lib/keyboard";
  import type { VocabSourceView } from "../lib/types";

  const SCOPE = "vocabsources";
  const POLL_MS = 4000;

  let sources = $state<VocabSourceView[]>([]);
  let selected = $state(0);
  let busy = $state("");
  let error = $state("");
  let status = $state("");
  let timer: ReturnType<typeof setInterval> | undefined;

  const hasActive = $derived(sources.some((s) => s.job?.status === "QUEUED" || s.job?.status === "RUNNING"));

  onMount(() => {
    pushScope(SCOPE);
    const unbind = bindKeys(SCOPE, {
      j: { description: "next source", legend: "move", keyLabel: "j/k", handler: () => move(1) },
      k: { description: "previous source", hidden: true, handler: () => move(-1) },
      ArrowDown: { description: "next source", hidden: true, handler: () => move(1) },
      ArrowUp: { description: "previous source", hidden: true, handler: () => move(-1) },
      d: { description: "download / refresh the selected source", legend: "download", handler: () => void downloadSelected() },
      x: { description: "remove the selected snapshot", legend: "remove", handler: () => void removeSelected() },
      r: { description: "refresh the list", legend: "refresh", handler: () => void refresh() },
    });
    void refresh();
    timer = setInterval(() => {
      if (hasActive) void refresh();
    }, POLL_MS);
    return () => {
      unbind();
      popScope(SCOPE);
      clearInterval(timer);
    };
  });

  function move(delta: number): void {
    if (sources.length === 0) return;
    selected = Math.min(sources.length - 1, Math.max(0, selected + delta));
    document.querySelectorAll("tbody tr")[selected]?.scrollIntoView?.({ block: "nearest" });
  }

  async function refresh(): Promise<void> {
    try {
      sources = (await fetchVocabSources()).sources ?? [];
      selected = Math.min(selected, Math.max(0, sources.length - 1));
    } catch (e) {
      error = e instanceof ApiError ? e.message : "loading sources failed";
    }
  }

  async function download(s: VocabSourceView): Promise<void> {
    busy = s.name;
    error = "";
    status = "";
    try {
      await downloadVocabSource(s.name);
      status = `${s.name} queued -- the worker downloads and installs it shortly`;
      await refresh();
    } catch (e) {
      error = e instanceof ApiError ? e.message : "queuing the download failed";
    } finally {
      busy = "";
    }
  }

  async function remove(s: VocabSourceView): Promise<void> {
    if (!s.installed) return;
    busy = s.name;
    error = "";
    status = "";
    try {
      await removeVocabSnapshot(s.name);
      status = `${s.name} removed -- its terms left the index`;
      await refresh();
    } catch (e) {
      error = e instanceof ApiError ? e.message : "removing the snapshot failed";
    } finally {
      busy = "";
    }
  }

  function downloadSelected(): void {
    const s = sources[selected];
    if (s?.snapshotUrl) void download(s);
  }

  function removeSelected(): void {
    const s = sources[selected];
    if (s?.installed) void remove(s);
  }

  function working(s: VocabSourceView): boolean {
    return s.job?.status === "QUEUED" || s.job?.status === "RUNNING";
  }
</script>

<main class="wide">
  <h1>Vocabularies</h1>
  <p class="muted intro">
    Public authority sources ready to use. <strong>Live</strong> sources answer the term picker's typeahead through
    their public APIs; <strong>downloadable</strong> sources install a snapshot into the local index for instant,
    offline search. Downloading again refreshes an installed snapshot in place.
  </p>

  <p aria-live="polite">
    {#if status}<span class="ok">{status}</span>{/if}
    {#if error}<span class="error">{error}</span>{/if}
  </p>

  {#if sources.length === 0}
    <p class="muted">No authority sources are registered.</p>
  {:else}
    <table>
      <thead>
        <tr>
          <th scope="col">Source</th>
          <th scope="col">Scheme</th>
          <th scope="col">Capabilities</th>
          <th scope="col">License</th>
          <th scope="col">Installed</th>
          <th scope="col">Status</th>
          <th scope="col">Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each sources as s, i (s.name)}
          <tr class:selected={i === selected} onfocusin={() => (selected = i)}>
            <td>
              <span class="name">{s.name}</span>
              {#if s.homepage}<a class="home" href={s.homepage} target="_blank" rel="noreferrer">about</a>{/if}
            </td>
            <td class="mono">{s.scheme}</td>
            <td>
              {#if s.suggestUrl}<span class="badge">live</span>{/if}
              {#if s.snapshotUrl}<span class="badge">downloadable</span>{/if}
            </td>
            <td class="muted">{s.license ?? ""}</td>
            <td>
              {#if s.installed}
                {s.installed.terms.toLocaleString()} terms
                <span class="muted">({new Date(s.installed.installedAt).toLocaleDateString()})</span>
              {:else if s.snapshotUrl}
                <span class="muted">not installed</span>
              {:else}
                <span class="muted">live only</span>
              {/if}
            </td>
            <td>
              {#if s.job}
                <span class="badge" data-status={s.job.status}>{s.job.status}</span>
                {#if s.job.error}<span class="error">{s.job.error}</span>{/if}
              {/if}
            </td>
            <td class="actions">
              {#if s.snapshotUrl}
                <button class="button" onclick={() => void download(s)} disabled={busy === s.name || working(s)}>
                  {working(s) ? "Working…" : s.installed ? "Refresh" : "Download"}
                </button>
              {/if}
              {#if s.installed}
                <button class="button button--quiet" onclick={() => void remove(s)} disabled={busy === s.name || working(s)}>
                  Remove
                </button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
    <p class="note">
      Additional sources (GND, Getty, MeSH, Homosaurus, …) register as drop-in configs through
      <code>POST /v1/vocabsources</code> -- see <code>docs/authority-sources.md</code>. Download and remove need the
      admin role.
    </p>
  {/if}
</main>

<style>
  .intro {
    max-width: 46rem;
  }
  table {
    border-collapse: collapse;
    width: 100%;
    font-size: 0.9rem;
  }
  th,
  td {
    text-align: left;
    padding: 0.35rem 0.6rem;
    border-bottom: 1px solid var(--rule);
  }
  tbody tr.selected {
    background: var(--surface);
  }
  tbody tr.selected td:first-child {
    box-shadow: inset 3px 0 0 var(--accent);
  }
  .name {
    font-weight: 600;
  }
  .home {
    margin-left: 0.4rem;
    font-size: 0.8rem;
  }
  .mono {
    font-family: var(--mono);
    font-size: 0.82rem;
  }
  .badge {
    font-size: 0.72rem;
    font-weight: 700;
    border-radius: 999px;
    padding: 0.1em 0.7em;
    border: 1px solid var(--rule);
    margin-right: 0.25rem;
  }
  .badge[data-status="DONE"] {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-ink);
  }
  .badge[data-status="FAILED"] {
    background: var(--danger);
    border-color: var(--danger);
    color: #fff;
  }
  .actions {
    white-space: nowrap;
  }
  .actions .button {
    margin-right: 0.35rem;
  }
  .note {
    font-size: 0.87rem;
    color: var(--ink-muted);
    max-width: 46rem;
    border-left: 3px solid var(--rule);
    padding-left: 0.7rem;
  }
  .ok {
    color: var(--accent);
  }
</style>
