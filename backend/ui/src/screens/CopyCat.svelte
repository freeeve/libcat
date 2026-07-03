<script lang="ts">
  // Copy cataloging (tasks/050): search external Z39.50/SRU targets, stage
  // hits (or a .mrc upload) into a reviewable batch, see each record's match
  // banner ("would merge with Work w…"), pick the overlay policy and
  // per-record decisions, and commit through the shared ingest pipeline.
  // Targets are admin-configured.
  import { onMount } from "svelte";
  import {
    ApiError,
    commitCopycatBatch,
    copycatSearch,
    deleteCopycatBatch,
    deleteCopycatTarget,
    fetchCopycatBatch,
    fetchCopycatBatches,
    fetchCopycatTargets,
    putCopycatTarget,
    reviewCopycatBatch,
    stageCopycatBatch,
  } from "../lib/api";
  import { sessionStore } from "../lib/stores";
  import type { CopycatBatch, CopycatPolicy, CopycatSearchResult, CopycatStagedRecord, CopycatTarget } from "../lib/types";

  const POLICIES: { value: CopycatPolicy; label: string }[] = [
    { value: "replace-feed", label: "Replace feed data (editorial always kept)" },
    { value: "fill-holes-only", label: "Fill holes only (never overwrite an existing edition)" },
    { value: "never", label: "Never overlay (import only unmatched records)" },
  ];

  let targets = $state<CopycatTarget[]>([]);
  let newTarget = $state<CopycatTarget>({ name: "", url: "", protocol: "sru" });
  let query = $state("");
  let results = $state<CopycatSearchResult[]>([]);
  let failures = $state<Record<string, string>>({});
  let picked = $state<Record<number, boolean>>({});
  let batches = $state<CopycatBatch[]>([]);
  let openBatch = $state<CopycatBatch | null>(null);
  let openRecords = $state<CopycatStagedRecord[]>([]);
  let busy = $state(false);
  let status = $state("");
  let error = $state("");

  const isAdmin = $derived(($sessionStore?.roles ?? []).includes("admin"));
  const pickedCount = $derived(Object.values(picked).filter(Boolean).length);

  onMount(() => {
    void loadTargets();
    void loadBatches();
  });

  async function loadTargets(): Promise<void> {
    try {
      targets = (await fetchCopycatTargets()).targets ?? [];
    } catch {
      targets = [];
    }
  }

  async function loadBatches(): Promise<void> {
    try {
      batches = (await fetchCopycatBatches()).batches ?? [];
    } catch {
      batches = [];
    }
  }

  async function addTarget(): Promise<void> {
    error = "";
    try {
      await putCopycatTarget($state.snapshot(newTarget));
      newTarget = { name: "", url: "", protocol: "sru" };
      await loadTargets();
    } catch (e) {
      error = e instanceof ApiError ? e.message : "saving the target failed";
    }
  }

  async function removeTarget(name: string): Promise<void> {
    try {
      await deleteCopycatTarget(name);
      await loadTargets();
    } catch {
      error = "deleting the target failed";
    }
  }

  async function search(): Promise<void> {
    busy = true;
    error = "";
    status = "";
    results = [];
    picked = {};
    try {
      const res = await copycatSearch(query);
      results = res.results ?? [];
      failures = res.failures ?? {};
    } catch (e) {
      error = e instanceof ApiError ? e.message : "search failed";
    } finally {
      busy = false;
    }
  }

  async function stagePicked(): Promise<void> {
    const records = results.filter((_, i) => picked[i]).map((r) => $state.snapshot(r.record));
    if (records.length === 0) return;
    busy = true;
    error = "";
    try {
      const res = await stageCopycatBatch({ label: `search: ${query}`, source: "search", records });
      status = `staged ${res.records.length} record${res.records.length === 1 ? "" : "s"}`;
      picked = {};
      await loadBatches();
      await open(res.batch.id);
    } catch (e) {
      error = e instanceof ApiError ? e.message : "staging failed";
    } finally {
      busy = false;
    }
  }

  async function upload(ev: Event): Promise<void> {
    const input = ev.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    busy = true;
    error = "";
    try {
      const buf = new Uint8Array(await file.arrayBuffer());
      let bin = "";
      for (const b of buf) bin += String.fromCharCode(b);
      const res = await stageCopycatBatch({ label: file.name, mrc: btoa(bin) });
      status = `staged ${res.records.length} record${res.records.length === 1 ? "" : "s"} from ${file.name}`;
      await loadBatches();
      await open(res.batch.id);
    } catch (e) {
      error = e instanceof ApiError ? e.message : "upload failed";
    } finally {
      busy = false;
      input.value = "";
    }
  }

  async function open(id: string): Promise<void> {
    error = "";
    try {
      const res = await fetchCopycatBatch(id);
      openBatch = res.batch;
      openRecords = res.records ?? [];
    } catch {
      error = "loading the batch failed";
    }
  }

  async function setPolicy(policy: CopycatPolicy): Promise<void> {
    if (!openBatch) return;
    try {
      openBatch = await reviewCopycatBatch(openBatch.id, { policy });
    } catch (e) {
      error = e instanceof ApiError ? e.message : "updating the policy failed";
    }
  }

  async function setDecision(index: number, decision: "import" | "skip"): Promise<void> {
    if (!openBatch) return;
    try {
      await reviewCopycatBatch(openBatch.id, { decisions: { [String(index)]: decision } });
      openRecords = openRecords.map((r) => (r.index === index ? { ...r, decision } : r));
    } catch {
      error = "updating the decision failed";
    }
  }

  async function commit(): Promise<void> {
    if (!openBatch) return;
    busy = true;
    error = "";
    try {
      const done = await commitCopycatBatch(openBatch.id);
      openBatch = done;
      status = `committed ${done.committed} record${done.committed === 1 ? "" : "s"}, ${done.skipped} skipped (${done.policy})`;
      await loadBatches();
    } catch (e) {
      error = e instanceof ApiError ? e.message : "commit failed";
    } finally {
      busy = false;
    }
  }

  async function removeBatch(id: string): Promise<void> {
    try {
      await deleteCopycatBatch(id);
      if (openBatch?.id === id) {
        openBatch = null;
        openRecords = [];
      }
      await loadBatches();
    } catch {
      error = "deleting the batch failed";
    }
  }

  function matchLabel(r: CopycatStagedRecord): string {
    if (r.match.matchedInstance) return "already in the catalog";
    if (r.match.matchedWork) return "would merge with an existing work";
    return "new";
  }
</script>

<main>
  <h1>Copy cataloging</h1>

  <details class="targets">
    <summary>Search targets ({targets.length})</summary>
    <ul class="tlist">
      {#each targets as t (t.name)}
        <li>
          <span class="mono">{t.name}</span> · {t.protocol} · <span class="muted">{t.url}</span>
          {#if isAdmin}
            <button class="button button--quiet mini" onclick={() => void removeTarget(t.name)}>Remove</button>
          {/if}
        </li>
      {:else}
        <li class="muted">No targets configured{isAdmin ? "" : " -- ask an admin"}.</li>
      {/each}
    </ul>
    {#if isAdmin}
      <div class="row">
        <input aria-label="Target name" bind:value={newTarget.name} placeholder="name (e.g. loc)" />
        <input class="grow" aria-label="Target URL" bind:value={newTarget.url} placeholder="SRU base URL or z3950 host:port/DB" />
        <select aria-label="Protocol" bind:value={newTarget.protocol}>
          <option value="sru">SRU</option>
          <option value="z3950">Z39.50</option>
        </select>
        <button class="button" onclick={() => void addTarget()}>Add target</button>
      </div>
    {/if}
  </details>

  <section aria-label="External search">
    <h2>Search external targets</h2>
    <div class="row">
      <input
        class="grow"
        aria-label="Search query"
        bind:value={query}
        placeholder="title, author, ISBN…"
        onkeydown={(ev) => ev.key === "Enter" && void search()}
      />
      <button class="button" onclick={() => void search()} disabled={busy || !query.trim()}>Search</button>
      <label class="button button--quiet upload-btn">
        Stage a .mrc file… <input type="file" accept=".mrc,.marc" onchange={(ev) => void upload(ev)} hidden />
      </label>
    </div>
    <p aria-live="polite">
      {#if busy}<span class="muted">Working…</span>{/if}
      {#if status}<span class="ok">{status}</span>{/if}
      {#if error}<span class="error">{error}</span>{/if}
      {#each Object.entries(failures) as [name, msg] (name)}
        <span class="error">{name}: {msg}</span>
      {/each}
    </p>

    {#if results.length > 0}
      <table>
        <thead>
          <tr><th scope="col"><span class="sr-only">Select</span></th><th scope="col">Target</th><th scope="col">Title</th><th scope="col">Author</th><th scope="col">Date</th><th scope="col">ISBN</th></tr>
        </thead>
        <tbody>
          {#each results as r, i (i)}
            <tr>
              <td><input type="checkbox" aria-label={"Select " + (r.title || "result")} bind:checked={picked[i]} /></td>
              <td class="mono">{r.target}</td>
              <td>{r.title}</td>
              <td>{r.author}</td>
              <td>{r.date}</td>
              <td class="mono">{r.isbn}</td>
            </tr>
          {/each}
        </tbody>
      </table>
      <p>
        <button class="button" onclick={() => void stagePicked()} disabled={busy || pickedCount === 0}>
          Stage {pickedCount || ""} selected for review
        </button>
      </p>
    {/if}
  </section>

  <section aria-label="Staged batches">
    <h2>Staged batches</h2>
    <ul class="blist">
      {#each batches as b (b.id)}
        <li class:open={openBatch?.id === b.id}>
          <button class="blabel" onclick={() => void open(b.id)}>
            {b.label} <span class="muted">· {b.records} records · {b.source}</span>
            <span class="badge" data-status={b.status}>{b.status}</span>
          </button>
          <button class="button button--quiet mini" onclick={() => void removeBatch(b.id)}>Delete</button>
        </li>
      {:else}
        <li class="muted">Nothing staged yet.</li>
      {/each}
    </ul>

    {#if openBatch}
      <div class="review" aria-label={"Batch " + openBatch.label}>
        <div class="row">
          <label for="cc-policy" class="muted">Overlay policy</label>
          <select
            id="cc-policy"
            value={openBatch.policy}
            disabled={openBatch.status !== "STAGED"}
            onchange={(ev) => void setPolicy((ev.currentTarget as HTMLSelectElement).value as CopycatPolicy)}
          >
            {#each POLICIES as p (p.value)}
              <option value={p.value}>{p.label}</option>
            {/each}
          </select>
        </div>
        <table>
          <thead>
            <tr><th scope="col">#</th><th scope="col">Title</th><th scope="col">Match</th><th scope="col">Decision</th></tr>
          </thead>
          <tbody>
            {#each openRecords as r (r.index)}
              <tr>
                <td class="mono">{r.index + 1}</td>
                <td>{r.title || "(untitled)"}</td>
                <td>
                  <span class="match" data-kind={r.match.matchedInstance ? "instance" : r.match.matchedWork ? "work" : "new"}>
                    {matchLabel(r)}
                  </span>
                  {#if r.match.workId}
                    <a href={"#/works/" + encodeURIComponent(r.match.workId)}>open {r.match.workId}</a>
                  {/if}
                </td>
                <td>
                  {#if openBatch.status === "STAGED"}
                    <label><input type="radio" name={"d" + r.index} checked={r.decision === "import"}
                      onchange={() => void setDecision(r.index, "import")} /> import</label>
                    <label><input type="radio" name={"d" + r.index} checked={r.decision === "skip"}
                      onchange={() => void setDecision(r.index, "skip")} /> skip</label>
                  {:else}
                    <span class="muted">{r.decision}</span>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
        <p class="actions">
          <button class="button" onclick={() => void commit()} disabled={busy || openBatch.status !== "STAGED"}>
            {openBatch.status === "STAGED" ? "Commit batch" : `Committed ${openBatch.committed} / skipped ${openBatch.skipped}`}
          </button>
        </p>
      </div>
    {/if}
  </section>
</main>

<style>
  h2 {
    font-size: 1rem;
    margin: 1.2rem 0 0.5rem;
  }
  .targets {
    margin: 0.6rem 0;
  }
  .targets summary {
    cursor: pointer;
    color: var(--ink-muted);
  }
  .tlist {
    list-style: none;
    padding: 0;
    margin: 0.4rem 0;
  }
  .tlist li {
    padding: 0.2rem 0;
  }
  .row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    flex-wrap: wrap;
    margin: 0.4rem 0;
  }
  .grow {
    flex: 1;
    min-width: 14rem;
    max-width: 30rem;
  }
  .mono {
    font-family: var(--mono);
    font-size: 0.85em;
  }
  .mini {
    font-size: 0.72rem;
    padding: 0.05em 0.6em;
  }
  .upload-btn {
    cursor: pointer;
  }
  table {
    border-collapse: collapse;
    width: 100%;
    font-size: 0.9rem;
  }
  th,
  td {
    text-align: left;
    padding: 0.3rem 0.55rem;
    border-bottom: 1px solid var(--rule);
  }
  .blist {
    list-style: none;
    padding: 0;
    margin: 0.3rem 0;
  }
  .blist li {
    display: flex;
    gap: 0.6rem;
    align-items: center;
    border-bottom: 1px solid var(--rule);
    padding: 0.25rem 0;
  }
  .blist li.open .blabel {
    color: var(--accent);
  }
  .blabel {
    background: none;
    border: 0;
    padding: 0.2rem 0;
    color: inherit;
    text-align: left;
    flex: 1;
    font-weight: 600;
  }
  .badge {
    font-size: 0.7rem;
    font-weight: 700;
    border-radius: 999px;
    padding: 0.08em 0.6em;
    border: 1px solid var(--rule);
    margin-left: 0.5em;
  }
  .badge[data-status="COMMITTED"] {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-ink);
  }
  .review {
    border: 1px solid var(--rule);
    border-radius: 8px;
    padding: 0.7rem 1rem 1rem;
    margin-top: 0.8rem;
  }
  .match {
    font-size: 0.82rem;
    border-radius: 999px;
    padding: 0.08em 0.7em;
    border: 1px solid var(--rule);
    margin-right: 0.5em;
  }
  .match[data-kind="new"] {
    background: var(--surface);
  }
  .match[data-kind="work"] {
    border-color: #c77d0a;
  }
  .match[data-kind="instance"] {
    border-color: crimson;
  }
  .actions {
    margin-top: 0.8rem;
  }
  .ok {
    color: var(--accent);
  }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
</style>
