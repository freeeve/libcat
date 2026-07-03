<script lang="ts">
  // The drawer rail: a fixed footer naming the keys live on this screen,
  // read from the same registry as the "?" overlay so it can never drift.
  // Re-renders on every scope or binding change via keymapVersion; the brief
  // CSS fade-in makes scope switches legible (the global reduced-motion rule
  // in app.css collapses it).
  import { formatKey, keymapVersion, legendBindings, topScope, type Binding } from "../lib/keyboard";

  let entries = $state<Binding[]>([]);
  let scopeKey = $state("");

  $effect(() => {
    void $keymapVersion;
    entries = legendBindings();
    scopeKey = topScope();
  });
</script>

<footer class="legend" aria-label="Keyboard shortcuts">
  {#key scopeKey}
    <span class="entries">
      {#each entries as b (b.key)}
        <span class="entry"><kbd>{formatKey(b)}</kbd> {b.legend ?? b.description}</span>
      {/each}
      <span class="entry"><kbd>?</kbd> help</span>
    </span>
  {/key}
</footer>

<style>
  .legend {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    height: var(--legend-h);
    background: var(--bg);
    border-top: 1px solid var(--rule);
    font-size: 0.72rem;
    color: var(--ink-muted);
    z-index: 40;
    display: flex;
    align-items: center;
    padding: 0 1.5rem;
  }
  .entries {
    display: flex;
    align-items: center;
    gap: 0.9rem;
    white-space: nowrap;
    overflow: hidden;
    animation: legend-fade 120ms ease-out;
  }
  @keyframes legend-fade {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }
  .entry {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }
  kbd {
    font-family: var(--mono);
    font-size: 0.92em;
    line-height: 1.4;
    background: var(--surface);
    border: 1px solid var(--rule);
    border-bottom-width: 2px;
    border-radius: 4px;
    padding: 0 0.4em;
  }
</style>
