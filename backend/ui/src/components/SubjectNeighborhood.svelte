<script lang="ts">
  // The crosswalking panel under an expanded subject chip (tasks/071): the
  // term's also-known-as labels and definition, then its SKOS neighborhood
  // -- broader, narrower, related, and siblings (the broader terms' other
  // children) -- each neighbor with Replace and Add actions that stage
  // ordinary ops through the parent form.
  import { onMount } from "svelte";
  import { resolveTerm } from "../lib/api";
  import { allAltLabels, bestDefinition, bestLabel } from "../lib/vocab";
  import type { Term } from "../lib/types";

  let {
    term,
    onreplace,
    onadd,
  }: {
    term: Term;
    onreplace: (t: Term) => void;
    onadd: (t: Term) => void;
  } = $props();

  interface Group {
    rel: string;
    terms: Term[];
  }

  let groups = $state<Group[]>([]);
  let loading = $state(true);

  onMount(() => void load());

  async function resolveAll(ids: string[]): Promise<Term[]> {
    const settled = await Promise.allSettled(ids.map((id) => resolveTerm(term.scheme, id)));
    return settled.filter((r) => r.status === "fulfilled").map((r) => r.value);
  }

  async function load(): Promise<void> {
    const [broader, narrower, related] = await Promise.all([
      resolveAll(term.broader ?? []),
      resolveAll(term.narrower ?? []),
      resolveAll(term.related ?? []),
    ]);
    // Siblings: the broader terms' other narrower children.
    const seen = new Set([term.id, ...(term.narrower ?? []), ...(term.related ?? [])]);
    const siblingIds: string[] = [];
    for (const parent of broader) {
      for (const id of parent.narrower ?? []) {
        if (!seen.has(id)) {
          seen.add(id);
          siblingIds.push(id);
        }
      }
    }
    const siblings = await resolveAll(siblingIds);
    groups = [
      { rel: "Broader", terms: broader },
      { rel: "Narrower", terms: narrower },
      { rel: "Related", terms: related },
      { rel: "Siblings", terms: siblings },
    ].filter((g) => g.terms.length > 0);
    loading = false;
  }
</script>

<div class="hood" aria-label={"Neighborhood of " + bestLabel(term)}>
  {#if allAltLabels(term).length > 0}
    <p class="aka"><span class="muted">Also known as:</span> {allAltLabels(term).join("; ")}</p>
  {/if}
  {#if bestDefinition(term)}
    <p class="def muted">{bestDefinition(term)}</p>
  {/if}

  {#if loading}
    <p class="muted small" role="status">Loading neighborhood…</p>
  {:else if groups.length === 0}
    <p class="muted small">No broader, narrower, related, or sibling terms.</p>
  {:else}
    {#each groups as g (g.rel)}
      <div class="rel">
        <h4>{g.rel}</h4>
        <ul>
          {#each g.terms as t (t.id)}
            <li>
              <span class="nlabel" title={bestDefinition(t) || t.id}>{bestLabel(t)}</span>
              <button
                class="button act"
                onclick={() => onreplace(t)}
                title={"Replace this subject with " + bestLabel(t)}
              >
                Replace
              </button>
              <button class="button button--quiet act" onclick={() => onadd(t)} title={"Also add " + bestLabel(t)}>
                Add
              </button>
            </li>
          {/each}
        </ul>
      </div>
    {/each}
  {/if}
</div>

<style>
  .hood {
    border: 1px solid var(--rule);
    border-left: 3px solid var(--accent);
    border-radius: 6px;
    background: var(--surface);
    padding: 0.45rem 0.8rem 0.6rem;
    margin: 0.2rem 0 0.4rem;
    max-width: 34rem;
  }
  .aka,
  .def {
    font-size: 0.85rem;
    margin: 0.2rem 0;
  }
  .small {
    font-size: 0.8rem;
  }
  .rel h4 {
    margin: 0.45rem 0 0.1rem;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--ink-muted);
  }
  .rel ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .rel li {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.12rem 0;
  }
  .nlabel {
    flex: 1;
    min-width: 8rem;
  }
  .act {
    font-size: 0.72rem;
    padding: 0.05em 0.65em;
  }
</style>
