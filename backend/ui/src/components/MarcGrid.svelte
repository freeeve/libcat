<script lang="ts">
  // The MARC editing grid (tasks/049): one row per field -- tag, indicators,
  // and the "$a … $b …" subfield line (control fields edit their raw value)
  // -- keyboard-first: Enter in a row's line inserts a fresh row below,
  // Alt+D duplicates the row, tag order is the cataloger's own. Fixed fields
  // (leader, 006/007/008) expand into positional builders. Lossy tags carry
  // a non-blocking warning: their edits persist verbatim, not modeled.
  import FixedFieldGrid from "./FixedFieldGrid.svelte";
  import { blankField, isControlTag, isFixedTag, lineToSubfields, subfieldsToLine } from "../lib/marc";
  import type { MarcRecordDoc, MarcField } from "../lib/types";

  let {
    record,
    knownLoss = {},
    onchange,
  }: {
    record: MarcRecordDoc;
    knownLoss?: Record<string, string>;
    onchange: (record: MarcRecordDoc) => void;
  } = $props();

  const FIDELITY_URL = "https://github.com/freeeve/libcatalog/blob/main/docs/marc-fidelity.md";

  let expanded = $state<Record<number, boolean>>({});

  function update(fields: MarcField[]): void {
    onchange({ ...record, fields });
  }

  function setField(i: number, patch: Partial<MarcField>): void {
    update(record.fields.map((f, j) => (j === i ? { ...f, ...patch } : f)));
  }

  function setTag(i: number, tag: string): void {
    const f = record.fields[i];
    const next: MarcField = { ...f, tag, lossy: knownLoss[tag] ?? "" };
    if (isControlTag(tag) && !isControlTag(f.tag)) {
      next.value = subfieldsToLine(f.subfields);
      next.subfields = undefined;
      next.ind1 = next.ind2 = undefined;
    } else if (!isControlTag(tag) && isControlTag(f.tag)) {
      next.subfields = lineToSubfields(f.value ?? "");
      next.value = undefined;
      next.ind1 = next.ind2 = " ";
    }
    update(record.fields.map((cur, j) => (j === i ? next : cur)));
  }

  function insertBelow(i: number, field?: MarcField): void {
    const next = [...record.fields];
    next.splice(i + 1, 0, field ?? blankField());
    update(next);
    queueMicrotask(() => focusRow(i + 1));
  }

  function removeRow(i: number): void {
    update(record.fields.filter((_, j) => j !== i));
  }

  function onLineKeydown(ev: KeyboardEvent, i: number): void {
    if (ev.key === "Enter") {
      ev.preventDefault();
      insertBelow(i);
    } else if (ev.altKey && ev.key.toLowerCase() === "d") {
      ev.preventDefault();
      insertBelow(i, structuredClone($state.snapshot(record.fields[i])));
    }
  }

  function focusRow(i: number): void {
    document.getElementById(`marc-tag-${record.node}-${i}`)?.focus();
  }
</script>

<div class="grid" role="group" aria-label="MARC fields">
  <div class="row head">
    <span class="tag">Tag</span><span class="ind">I1</span><span class="ind">I2</span><span>Value</span>
  </div>

  <div class="row">
    <span class="tag mono">LDR</span>
    <span class="ind"></span><span class="ind"></span>
    <div class="val">
      <input
        class="line mono"
        aria-label="Leader"
        value={record.leader}
        onchange={(ev) => onchange({ ...record, leader: (ev.currentTarget as HTMLInputElement).value })}
      />
      <button class="button button--quiet mini" onclick={() => (expanded = { ...expanded, [-1]: !expanded[-1] })}>
        {expanded[-1] ? "Hide" : "Positions"}
      </button>
      {#if expanded[-1]}
        <FixedFieldGrid tag="LDR" value={record.leader} onchange={(v) => onchange({ ...record, leader: v })} />
      {/if}
    </div>
  </div>

  {#each record.fields as f, i (i)}
    <div class="row" class:lossy={!!(f.lossy || knownLoss[f.tag])}>
      <input
        id={`marc-tag-${record.node}-${i}`}
        class="tag mono"
        aria-label="Tag"
        maxlength="3"
        value={f.tag}
        onchange={(ev) => setTag(i, (ev.currentTarget as HTMLInputElement).value)}
      />
      {#if isControlTag(f.tag)}
        <span class="ind"></span><span class="ind"></span>
      {:else}
        <input class="ind mono" aria-label="Indicator 1" maxlength="1" value={f.ind1 ?? " "}
          onchange={(ev) => setField(i, { ind1: (ev.currentTarget as HTMLInputElement).value || " " })} />
        <input class="ind mono" aria-label="Indicator 2" maxlength="1" value={f.ind2 ?? " "}
          onchange={(ev) => setField(i, { ind2: (ev.currentTarget as HTMLInputElement).value || " " })} />
      {/if}
      <div class="val">
        {#if isControlTag(f.tag)}
          <input class="line mono" aria-label={"Field " + f.tag + " value"} value={f.value ?? ""}
            onkeydown={(ev) => onLineKeydown(ev, i)}
            onchange={(ev) => setField(i, { value: (ev.currentTarget as HTMLInputElement).value })} />
          {#if isFixedTag(f.tag)}
            <button class="button button--quiet mini" onclick={() => (expanded = { ...expanded, [i]: !expanded[i] })}>
              {expanded[i] ? "Hide" : "Positions"}
            </button>
            {#if expanded[i]}
              <FixedFieldGrid tag={f.tag} value={f.value ?? ""} onchange={(v) => setField(i, { value: v })} />
            {/if}
          {/if}
        {:else}
          <input class="line mono" aria-label={"Field " + f.tag + " subfields"} value={subfieldsToLine(f.subfields)}
            onkeydown={(ev) => onLineKeydown(ev, i)}
            onchange={(ev) => setField(i, { subfields: lineToSubfields((ev.currentTarget as HTMLInputElement).value) })} />
        {/if}
        {#if f.lossy || knownLoss[f.tag]}
          <p class="warn">
            Crosswalk-lossy tag: {f.lossy || knownLoss[f.tag]} -- edits persist verbatim
            (<a href={FIDELITY_URL} target="_blank" rel="noreferrer">fidelity table</a>).
          </p>
        {/if}
      </div>
      <span class="acts">
        <button class="button button--quiet mini" title="Duplicate field (Alt+D)"
          onclick={() => insertBelow(i, structuredClone($state.snapshot(f)))}>Dup</button>
        <button class="button button--quiet mini" onclick={() => removeRow(i)}>Del</button>
      </span>
    </div>
  {/each}

  <p>
    <button class="button button--quiet" onclick={() => insertBelow(record.fields.length - 1)}>Add field</button>
    <span class="muted hint">Enter in a value inserts a row below · Alt+D duplicates</span>
  </p>
</div>

<style>
  .grid {
    margin: 0.5rem 0;
  }
  .row {
    display: grid;
    grid-template-columns: 3.6rem 1.6rem 1.6rem 1fr auto;
    gap: 0.4rem;
    align-items: start;
    padding: 0.15rem 0;
    border-bottom: 1px solid var(--rule);
  }
  .row.head {
    font-size: 0.72rem;
    color: var(--ink-muted);
    border-bottom-color: var(--ink-muted);
  }
  .row.lossy {
    background: color-mix(in srgb, var(--surface) 80%, #c77d0a 8%);
  }
  .mono {
    font-family: var(--mono);
  }
  .tag {
    width: 3.4rem;
  }
  .ind {
    width: 1.5rem;
    text-align: center;
  }
  .val {
    display: block;
  }
  .line {
    width: 100%;
    font-size: 0.85rem;
  }
  .mini {
    font-size: 0.72rem;
    padding: 0.05em 0.6em;
  }
  .warn {
    font-size: 0.78rem;
    margin: 0.15rem 0 0.1rem;
    color: inherit;
  }
  .acts {
    display: inline-flex;
    gap: 0.25rem;
  }
  .hint {
    font-size: 0.78rem;
  }
</style>
