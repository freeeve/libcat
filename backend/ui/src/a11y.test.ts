// Axe audit over the Login screen, the WorkEditor's document renderer
// (WorkDocView), the moderation Queue, and the VocabPicker modal, mounted
// with fixture data in jsdom. color-contrast needs a real rendering engine
// (canvas), so that single rule is skipped here; the palette in app.css is
// chosen for WCAG AA contrast.
import { afterEach, describe, expect, it, vi } from "vitest";
import { mount, unmount, flushSync } from "svelte";
import axe from "axe-core";
import Login from "./screens/Login.svelte";
import Queue from "./screens/Queue.svelte";
import VocabPicker from "./components/VocabPicker.svelte";
import WorkDocView from "./components/WorkDocView.svelte";
import { invalidateAccess, loginLocal } from "./lib/auth";
import { setConfig } from "./lib/config";
import { sessionStore } from "./lib/stores";
import type { QueuePage, Suggestion, WorkDoc } from "./lib/types";

const fixtureDoc: WorkDoc = {
  workId: "w-001",
  profileId: "work-monograph",
  work: {
    id: "w-001",
    fields: {
      title: [{ v: "The Sea Around Us", prov: "feed:overdrive", node: "_:t1" }],
      subjectLabels: [
        { v: "Ocean", prov: "enrichment:locsh", node: "_:s1", iri: false },
        { v: "Marine biology", prov: "editorial:", node: "_:s2" },
        { v: "Oceanography -- history", prov: "feed:marc", node: "_:s3", overridden: true },
      ],
      language: [{ v: "en", lang: "en", prov: "feed:overdrive", node: "_:l1" }],
    },
  },
  instances: [
    {
      id: "i-001",
      fields: {
        isbn: [{ v: "9780195069976", prov: "feed:overdrive", node: "_:i1" }],
      },
    },
  ],
  passthrough: [
    '<http://example.org/w-001> <http://example.org/p> "unclaimed" <feed:overdrive> .',
  ],
};

const fixtureSuggestions: Suggestion[] = [
  {
    workId: "w1",
    term: { scheme: "lcsh", id: "http://id.loc.gov/sh1", label: "Sea monsters" },
    type: "ADD",
    status: "PENDING",
    supporterCount: 4,
    provenance: "PIPELINE",
    confidence: 0.91,
    workTitle: "The Sea Around Us",
    createdAt: "2026-06-01T00:00:00Z",
    lastActivityAt: "2026-06-02T00:00:00Z",
  },
  {
    workId: "w2",
    term: { scheme: "lcsh", id: "http://id.loc.gov/sh2", label: "Whales" },
    type: "REMOVE",
    status: "PENDING",
    supporterCount: 2,
    reasonCounts: { "off-topic": 2, offensive: 1 },
    provenance: "PATRON",
    workTitle: "Moby-Dick",
    createdAt: "2026-06-03T00:00:00Z",
    lastActivityAt: "2026-06-03T00:00:00Z",
  },
  {
    workId: "w3",
    term: { scheme: "folk", id: "cozy-fantasy", label: "cozy-fantasy" },
    type: "ADD",
    status: "PENDING",
    supporterCount: 7,
    provenance: "PATRON",
    workTitle: "Legends & Lattes",
    createdAt: "2026-06-04T00:00:00Z",
    lastActivityAt: "2026-06-04T00:00:00Z",
  },
];

async function audit(node: Element): Promise<axe.AxeResults> {
  return axe.run(node, {
    rules: { "color-contrast": { enabled: false } },
  });
}

function jwtLike(): string {
  const body = btoa(JSON.stringify({ email: "staff@example.org", roles: ["librarian"] }))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
  return `hdr.${body}.sig`;
}

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status });
}

/** Seeds a librarian session with fetch fully mocked; queue reads return the
 *  fixture page. Returns the cleanup for afterEach. */
async function mockStaffBackend(page: QueuePage): Promise<() => void> {
  setConfig({ apiBase: "", localAuth: true, provider: "test", schemes: ["lcsh", "fast", "folk"] });
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  fetchMock.mockResolvedValueOnce(json({ accessToken: jwtLike(), refreshToken: "r1", expiresIn: 900 }));
  await loginLocal("staff@example.org", "pw");
  fetchMock.mockImplementation(() => Promise.resolve(json(page)));
  sessionStore.set({ email: "staff@example.org", roles: ["librarian"] });
  return () => {
    vi.unstubAllGlobals();
    sessionStore.set(null);
    setConfig(null);
    invalidateAccess();
    localStorage.clear();
  };
}

let cleanup: (() => void) | null = null;

afterEach(() => {
  cleanup?.();
  cleanup = null;
  document.body.innerHTML = "";
});

describe("a11y", () => {
  it("Login has no axe violations", async () => {
    const host = document.createElement("div");
    document.body.appendChild(host);
    const app = mount(Login, {
      target: host,
      props: {
        config: {
          apiBase: "",
          localAuth: true,
          oidc: { issuer: "https://issuer.example", clientId: "spa" },
          provider: "test",
        },
      },
    });
    cleanup = () => unmount(app);
    flushSync();
    const results = await audit(host);
    expect(results.violations).toEqual([]);
  });

  it("WorkEditor document view has no axe violations", async () => {
    // WorkDocView renders an <article>; give it the page's main landmark the
    // WorkEditor screen provides in the app.
    const host = document.createElement("main");
    document.body.appendChild(host);
    const app = mount(WorkDocView, { target: host, props: { doc: fixtureDoc, etag: "etag-1" } });
    cleanup = () => unmount(app);
    flushSync();
    const results = await audit(host);
    expect(results.violations).toEqual([]);
  });

  it("Queue with rows and staged decisions has no axe violations", async () => {
    const restore = await mockStaffBackend({ items: fixtureSuggestions, cursor: "c1" });
    const host = document.createElement("div");
    document.body.appendChild(host);
    const app = mount(Queue, { target: host });
    cleanup = () => {
      unmount(app);
      restore();
    };
    flushSync();
    await new Promise((r) => setTimeout(r, 0)); // let the mocked queue load land
    flushSync();
    expect(host.textContent).toContain("The Sea Around Us");
    // Stage one decision so the publish bar is part of the audited tree.
    const approveBtn = [...host.querySelectorAll("button")].find((b) => b.textContent?.trim() === "Approve");
    approveBtn?.click();
    flushSync();
    expect(host.textContent).toContain("staged");
    const results = await audit(host);
    expect(results.violations).toEqual([]);
  });

  it("VocabPicker modal has no axe violations", async () => {
    setConfig({ apiBase: "", localAuth: true, provider: "test", schemes: ["lcsh", "fast", "folk"] });
    const fetchMock = vi.fn(() => Promise.resolve(json({ terms: [] })));
    vi.stubGlobal("fetch", fetchMock);
    const host = document.createElement("div");
    document.body.appendChild(host);
    const app = mount(VocabPicker, {
      target: host,
      props: { onselect: () => undefined, onclose: () => undefined },
    });
    cleanup = () => {
      unmount(app);
      vi.unstubAllGlobals();
      setConfig(null);
    };
    flushSync();
    expect(host.querySelector('[role="dialog"]')).not.toBeNull();
    const results = await audit(host);
    expect(results.violations).toEqual([]);
  });
});
