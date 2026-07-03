<script lang="ts">
  // External subject lookup (tasks/073): a button fans the work's ISBNs out
  // to the copycat targets and lists the 6XX headings they carry -- the
  // "suck these in" flow. Reconciled headings add as controlled subjects,
  // the rest as tags; every Add is an ordinary staged editorial op, so the
  // provenance badge, preview, and audit trail apply unchanged.
  import { ApiError, lookupSubjects } from "../lib/api";
  import type { SubjectCandidate } from "../lib/types";

  let {
    workId,
    onadd,
  }: {
    workId: string;
    onadd: (c: SubjectCandidate) => void;
  } = $props();

  let candidates = $state<SubjectCandidate[]>([]);
  let failures = $state<Record<string, string>>({});
  let added = $state<Record<string, boolean>>({});
  let ran = $state(false);
  let busy = $state(false);
  let error = $state("");

  async function run(): Promise<void> {
    busy = true;
    error = "";
    try {
      const res = await lookupSubjects(workId);
      candidates = res.candidates ?? [];
      failures = res.failures ?? {};
      added = {};
      ran = true;
    } catch (e) {
      error = e instanceof ApiError ? e.message : "lookup failed";
    } finally {
      busy = false;
    }
  }

  function key(c: SubjectCandidate): string {
    return c.tag + "|" + c.heading;
  }

  function add(c: SubjectCandidate): void {
    onadd(c);
    added[key(c)] = true;
  }
</script>

<div class="lookup">
  <p class="acts">
    <button class="button button--quiet act" onclick={() => void run()} disabled={busy}>
      {busy ? "Searching targets…" : ran ? "Look up subjects again" : "Look up subjects at targets…"}
    </button>
    {#if error}<span class="error" role="alert">{error}</span>{/if}
    {#each Object.entries(failures) as [name, msg] (name)}
      <span class="error">{name}: {msg}</span>
    {/each}
  </p>
  {#if ran && candidates.length === 0 && !error}
    <p class="muted small">The targets' records carry no headings this work lacks.</p>
  {:else if candidates.length > 0}
    <ul class="cands">
      {#each candidates as c (key(c))}
        <li>
          <span class="heading">{c.heading}</span>
          <span class="meta">
            {c.tag}{c.source ? " · " + c.source : ""} · {c.count}×
            {c.targets.join(", ")}
          </span>
          {#if c.term}
            <span class="badge controlled" title={c.term.id}>{c.term.scheme}</span>
          {:else}
            <span class="badge" title="no whole-heading match in a loaded vocabulary">adds as tag</span>
          {/if}
          {#if added[key(c)]}
            <span class="ok">staged</span>
          {:else}
            <button class="button act" onclick={() => add(c)}>Add</button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .lookup {
    margin: 0.3rem 0 0.6rem;
  }
  .acts {
    display: flex;
    gap: 0.6rem;
    align-items: center;
    flex-wrap: wrap;
    margin: 0.2rem 0;
  }
  .act {
    font-size: 0.78rem;
    padding: 0.1em 0.7em;
  }
  .small {
    font-size: 0.82rem;
  }
  .cands {
    list-style: none;
    margin: 0.25rem 0;
    padding: 0;
    max-width: 44rem;
  }
  .cands li {
    display: flex;
    align-items: baseline;
    gap: 0.55rem;
    padding: 0.15rem 0;
    border-bottom: 1px dashed var(--rule);
    flex-wrap: wrap;
  }
  .heading {
    font-weight: 600;
  }
  .meta {
    font-size: 0.75rem;
    color: var(--ink-muted);
    flex: 1;
    min-width: 10rem;
  }
  .badge {
    font-size: 0.66rem;
    font-weight: 650;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--ink-muted);
    border: 1px solid var(--rule);
    border-radius: 999px;
    padding: 0.05em 0.55em;
  }
  .badge.controlled {
    border-color: var(--accent);
    color: var(--accent);
  }
  .ok {
    color: var(--accent);
    font-size: 0.78rem;
    font-weight: 600;
  }
</style>
