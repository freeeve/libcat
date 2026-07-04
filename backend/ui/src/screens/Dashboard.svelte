<script lang="ts">
  // Landing screen: who is signed in, the catalog at a glance (work count,
  // installed vocabularies, waiting review work), and where to go. Single
  // letters jump straight to a screen (the same letters the global "g"
  // sequences use). Each stat tile is a link; a tile whose endpoint fails
  // or sits above the viewer's role simply stays away.
  import { onMount } from "svelte";
  import { canModerate, canPublish, type Session } from "../lib/auth";
  import {
    fetchCopycatBatches,
    fetchDuplicates,
    fetchQueue,
    fetchVocabSources,
    fetchWithdrawn,
    fetchWorks,
  } from "../lib/api";
  import { bindKeys, popScope, pushScope, type BindingSpec } from "../lib/keyboard";
  import { navigate } from "../lib/router";

  let { session }: { session: Session } = $props();

  const SCOPE = "dashboard";
  const JUMPS: [string, string, string][] = [
    ["w", "/works", "go to works"],
    ["a", "/authorities", "go to authorities"],
    ["q", "/queue", "go to the queue"],
    ["b", "/batch", "go to batch operations"],
    ["m", "/macros", "go to macros"],
    ["e", "/exports", "go to exports"],
    ["i", "/copycat", "go to import"],
    ["u", "/duplicates", "go to duplicates"],
    ["p", "/promotions", "go to promotions"],
    ["t", "/withdrawals", "go to withdrawals"],
  ];

  /** Tile order on the glance row. */
  const STAT_ORDER = ["#/works", "#/vocabularies", "#/copycat", "#/duplicates", "#/withdrawals"];

  interface Stat {
    label: string;
    href: string;
    value: number;
    /** Quiet second line ("3 installed", "merge candidates"). */
    sub?: string;
    /** Attention tiles get the accent rail when nonzero (waiting work). */
    attention?: boolean;
  }

  let stats = $state<Stat[]>([]);
  let pending = $state<number | null>(null);
  let queueError = $state("");

  onMount(() => {
    pushScope(SCOPE);
    const specs: Record<string, BindingSpec> = {};
    for (const [key, path, description] of JUMPS) {
      specs[key] = {
        description,
        legend: "jump to screen",
        keyLabel: "w/a/q/…",
        hidden: key !== "w",
        handler: () => navigate(path),
      };
    }
    const unbind = bindKeys(SCOPE, specs);
    void loadPending();
    void loadStats();
    return () => {
      unbind();
      popScope(SCOPE);
    };
  });

  async function loadPending(): Promise<void> {
    if (!canModerate(session)) return;
    try {
      const page = await fetchQueue({ status: "PENDING" });
      pending = page.items.length;
    } catch {
      queueError = "queue unavailable";
    }
  }

  function upsert(s: Stat): void {
    stats = [...stats.filter((x) => x.href !== s.href), s].sort(
      (a, b) => STAT_ORDER.indexOf(a.href) - STAT_ORDER.indexOf(b.href),
    );
  }

  /** Fires the count queries concurrently; each tile appears as its number
   *  lands, in a stable order. Failures just leave the tile out. */
  function loadStats(): void {
    if (!canPublish(session)) return;
    fetchWorks("", 1).then(
      (page) => upsert({ label: "Works", href: "#/works", value: page.total }),
      () => {},
    );
    fetchVocabSources().then((res) => {
      const installed = (res.sources ?? []).filter((s) => s.installed);
      const terms = installed.reduce((n, s) => n + (s.installed?.terms ?? 0), 0);
      upsert({
        label: "Vocabulary terms",
        href: "#/vocabularies",
        value: terms,
        sub: `${installed.length} installed vocabular${installed.length === 1 ? "y" : "ies"}`,
      });
    }, () => {});
    fetchCopycatBatches().then((res) => {
      const staged = (res.batches ?? []).filter((b) => b.status === "STAGED").length;
      upsert({ label: "Staged batches", href: "#/copycat", value: staged, sub: "awaiting review", attention: true });
    }, () => {});
    fetchDuplicates().then(
      (res) => upsert({ label: "Duplicate groups", href: "#/duplicates", value: (res.groups ?? []).length, sub: "merge candidates", attention: true }),
      () => {},
    );
    fetchWithdrawn().then(
      (res) => upsert({ label: "Withdrawals", href: "#/withdrawals", value: (res.works ?? []).length, sub: "gone from their feed", attention: true }),
      () => {},
    );
  }
</script>

<main class="wide">
  <h1>Dashboard</h1>
  <p>
    Signed in as <strong>{session.email}</strong>
    {#if session.roles.length > 0}
      <span class="muted">({session.roles.join(", ")})</span>
    {/if}
  </p>

  {#if stats.length > 0}
    <ul class="stats" aria-label="Catalog at a glance">
      {#each stats as s (s.href)}
        <li>
          <a href={s.href} class:attention={s.attention && s.value > 0}>
            <span class="stat-label">{s.label}</span>
            <span class="stat-value">{s.value.toLocaleString()}</span>
            {#if s.sub}<span class="stat-sub muted">{s.sub}</span>{/if}
          </a>
        </li>
      {/each}
    </ul>
  {/if}

  <nav aria-label="Sections">
    <ul class="cards">
      <li>
        <a href="#/works">
          <h2>Work search</h2>
          <p class="muted">Find and open catalog records.</p>
        </a>
      </li>
      {#if canModerate(session)}
        <li>
          <a href="#/queue">
            <h2>Review queue</h2>
            <p class="muted">
              {#if pending !== null}
                {pending} pending suggestion{pending === 1 ? "" : "s"}
              {:else if queueError}
                {queueError}
              {:else}
                Loading…
              {/if}
            </p>
          </a>
        </li>
        <li>
          <a href="#/promotions">
            <h2>Tag promotions</h2>
            <p class="muted">Fold community tags into controlled vocabulary.</p>
          </a>
        </li>
      {/if}
    </ul>
  </nav>
</main>

<style>
  /* The glance row: hero numbers in text ink (no decorative color), label
     above, quiet context below. Attention tiles carry the accent rail only
     when there is actually waiting work. */
  .stats {
    list-style: none;
    padding: 0;
    margin: 1rem 0 1.4rem;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr));
    gap: 0.8rem;
    max-width: 72rem;
  }
  .stats a {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    border: 1px solid var(--rule);
    border-radius: 8px;
    padding: 0.65rem 0.95rem 0.7rem;
    text-decoration: none;
    color: inherit;
  }
  .stats a:hover {
    border-color: var(--accent);
  }
  .stats a.attention {
    box-shadow: inset 3px 0 0 var(--accent);
  }
  .stat-label {
    font-size: 0.72rem;
    font-weight: 650;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--ink-muted);
  }
  .stat-value {
    font-size: 1.9rem;
    font-weight: 650;
    line-height: 1.15;
    font-variant-numeric: tabular-nums;
  }
  .stat-sub {
    font-size: 0.78rem;
  }
  .cards {
    list-style: none;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr));
    gap: 1rem;
    max-width: 72rem;
  }
  .cards a {
    display: block;
    border: 1px solid var(--rule);
    border-radius: 8px;
    padding: 0.75rem 1.1rem;
    text-decoration: none;
    color: inherit;
  }
  .cards a:hover {
    border-color: var(--accent);
  }
  .cards h2 {
    margin: 0.2rem 0;
    color: var(--accent);
  }
  .cards p {
    margin: 0.2rem 0 0.4rem;
  }
</style>
