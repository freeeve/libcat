<script lang="ts">
  // Sticky action bar over the staged decision list: counts, clear, and the
  // batch apply -- with the publish-in-the-same-call variant for librarians.
  let {
    approveCount,
    rejectCount,
    canPublishNow,
    busy,
    onapply,
    onclear,
  }: {
    approveCount: number;
    rejectCount: number;
    canPublishNow: boolean;
    busy: boolean;
    onapply: (publish: boolean) => void;
    onclear: () => void;
  } = $props();

  const total = $derived(approveCount + rejectCount);
</script>

{#if total > 0}
  <div class="bar" role="region" aria-label="Staged decisions">
    <span class="counts">
      <strong>{total}</strong> staged · {approveCount} approve · {rejectCount} reject
    </span>
    <span class="spacer"></span>
    <button class="button button--quiet" onclick={onclear} disabled={busy}>Clear</button>
    <button class="button" onclick={() => onapply(false)} disabled={busy}>Apply</button>
    {#if canPublishNow}
      <button class="button" onclick={() => onapply(true)} disabled={busy}>Apply &amp; publish</button>
    {/if}
  </div>
{/if}

<style>
  .bar {
    position: sticky;
    bottom: var(--legend-h);
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.6rem 0.9rem;
    margin-top: 1rem;
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: 8px 8px 0 0;
    box-shadow: 0 -4px 14px rgba(20, 22, 25, 0.08);
  }
  .spacer {
    flex: 1;
  }
</style>
