<script lang="ts">
  // The richer part of one audit entry: the harvest provenance of an
  // approved subject (which peer sourced it, with a link to the peer's
  // record) and the field-level diff a record edit carried (N-Quads lines
  // added/removed, collapsed behind a count).
  import type { AuditEntry } from "../lib/types";

  let { entry }: { entry: AuditEntry } = $props();

  const changes = $derived(entry.changes);
  const added = $derived(changes?.added ?? []);
  const removed = $derived(changes?.removed ?? []);
</script>

{#if entry.attributions?.length}
  <span class="prov muted">
    source:
    {#each entry.attributions as a, i (a.source + i)}{i > 0 ? ", " : ""}{#if a.ref}<a
          href={a.ref}
          target="_blank"
          rel="noopener"
          title={a.basis && a.key ? `${a.basis} ${a.key}` : undefined}>{a.source}</a
        >{:else}<span title={a.basis && a.key ? `${a.basis} ${a.key}` : undefined}>{a.source}</span>{/if}{/each}
  </span>
{/if}

{#if changes && (added.length || removed.length)}
  <details class="changes">
    <summary>changed &plus;{added.length} / &minus;{removed.length}{changes.more ? ` (+${changes.more} more)` : ""}</summary>
    <ul class="difflines">
      {#each added as l, i (i)}<li class="add">+ {l}</li>{/each}
      {#each removed as l, i (i)}<li class="rem">&minus; {l}</li>{/each}
    </ul>
  </details>
{/if}

<style>
  .prov {
    font-size: 0.85rem;
  }
  .changes {
    flex-basis: 100%;
    font-size: 0.85rem;
  }
  .changes summary {
    cursor: pointer;
    color: var(--ink-muted);
    font-family: var(--mono);
  }
  .difflines {
    list-style: none;
    margin: 0.3rem 0 0;
    padding: 0.3rem 0.5rem;
    background: var(--bg-subtle, var(--bg));
    border: 1px solid var(--rule);
    border-radius: 4px;
    overflow-x: auto;
  }
  .difflines li {
    font-family: var(--mono);
    font-size: 0.75rem;
    white-space: pre;
  }
  .add {
    color: var(--ok, #157347);
  }
  .rem {
    color: var(--danger, #b02a37);
  }
</style>
