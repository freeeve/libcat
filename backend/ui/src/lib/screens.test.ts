import { describe, expect, it } from "vitest";
import { NON_SCREEN_ROUTES, ROUTES } from "./router";
import { chordMap, isCurrent, navMenus, paletteLabel, SCREENS, settingsScreens, sidebarScreens } from "./screens";

// the palette, the "g <letter>" chords, and the sidebar were three
// hand-maintained lists and no two agreed. The palette answered "No matching
// commands." for Vocabularies, Withdrawals and Profiles -- which does not say
// "that screen is elsewhere", it says the thing does not exist. These tests are
// the pin: a new screen that routes but is not listed fails the build.
describe("SCREENS covers the router", () => {
  it("has an entry for every navigable route", () => {
    const navigable = ROUTES.filter((r) => !NON_SCREEN_ROUTES.has(r.name)).map((r) => r.name);
    const listed = new Set(SCREENS.map((s) => s.route));
    const missing = navigable.filter((name) => !listed.has(name));
    expect(missing, `routes with no screen entry: ${missing.join(", ")}`).toEqual([]);
  });

  it("lists no screen the router cannot resolve", () => {
    const routed = new Map(ROUTES.map((r) => [r.name, r.pattern]));
    for (const s of SCREENS) {
      expect(routed.get(s.route), `screen ${s.route} is not a route`).toBe(s.path);
    }
  });

  // A detail route lights its parent's sidebar link; it is not itself a screen.
  it("only claims alsoCurrent routes that exist and are not screens", () => {
    const routed = new Set(ROUTES.map((r) => r.name));
    const listed = new Set(SCREENS.map((s) => s.route));
    for (const s of SCREENS) {
      for (const name of s.alsoCurrent ?? []) {
        expect(routed.has(name), `${s.route}.alsoCurrent names ${name}, which does not route`).toBe(true);
        expect(listed.has(name), `${s.route}.alsoCurrent names ${name}, which is its own screen`).toBe(false);
      }
    }
  });
});

describe("the derived surfaces", () => {
  it("gives the palette every screen, including the admin-only one", () => {
    // Hiding a screen's existence from the palette is the bug; the route
    // already refuses a non-admin.
    const profiles = SCREENS.find((s) => s.route === "profiles");
    expect(profiles?.adminOnly).toBe(true);
    for (const name of ["vocabsources", "withdrawals", "profiles"]) {
      expect(SCREENS.some((s) => s.route === name)).toBe(true);
    }
  });

  it("searches the palette by the label a person would type", () => {
    const labels = SCREENS.map((s) => paletteLabel(s).toLowerCase());
    for (const typed of ["vocabularies", "withdrawals", "profiles", "duplicates", "import"]) {
      expect(labels.some((l) => l.includes(typed)), `nothing matches "${typed}"`).toBe(true);
    }
    // "Import" is the sidebar's word; the palette also answers "copy cataloging".
    expect(paletteLabel(SCREENS.find((s) => s.route === "copycat")!)).toBe("Copy cataloging (import)");
  });

  // The palette's order is muscle memory: open it, press Enter, land on Works.
  // Deriving NAV from a table made this an accident waiting to happen, so it is
  // pinned.
  it("leads with Works and trails with Dashboard", () => {
    expect(SCREENS[0].route).toBe("works");
    expect(SCREENS[1].route).toBe("authorities");
    expect(SCREENS[SCREENS.length - 1].route).toBe("dashboard");
  });

  it("assigns each chord to exactly one screen", () => {
    const chords = SCREENS.filter((s) => s.chord).map((s) => s.chord);
    expect(new Set(chords).size).toBe(chords.length);
    const map = chordMap();
    expect(map["g v"]?.[0]).toBe("/vocabularies");
    expect(map["g t"]?.[0]).toBe("/withdrawals");
    expect(map["g p"]?.[0]).toBe("/promotions");
    // Profiles had no chord; "g p" was taken, so it takes "f".
    expect(map["g f"]?.[0]).toBe("/profiles");
  });

  it("keeps the primary nav to the daily operational verbs", () => {
    const asStaff = sidebarScreens(false).map((s) => s.route);
    const asAdmin = sidebarScreens(true).map((s) => s.route);
    // Grouped screens live in the Settings menu, never the bar -- for
    // anyone (the settings-menu declutter, task 382). Enrichment moved on
    // from Settings to the Maintenance nav menu (task 448).
    for (const moved of ["profiles", "suggestions", "vocabsources", "macros", "enrichment"]) {
      expect(asStaff, `${moved} back on the staff nav`).not.toContain(moved);
      expect(asAdmin, `${moved} back on the admin nav`).not.toContain(moved);
    }
    // Menued screens live under their nav menu, not flat on the bar
    // (task 435).
    for (const menued of ["queue", "promotions", "duplicates", "withdrawals", "audit", "diversity", "exports"]) {
      expect(asStaff, `${menued} back flat on the nav`).not.toContain(menued);
    }
    // The dashboard is the brand link, not a nav item.
    expect(asAdmin).not.toContain("dashboard");
    // The flat bar is exactly the four daily verbs: with the three menus it
    // renders 7 top-level entries, down from 11 (task 435).
    expect(asStaff).toEqual(["works", "authorities", "batch", "copycat"]);
  });

  it("groups the occasional destinations into three nav menus (task 435)", () => {
    const menus = navMenus(false);
    expect(menus.map((m) => m.id)).toEqual(["review", "maintenance", "reports"]);
    const byId = Object.fromEntries(menus.map((m) => [m.id, m.screens.map((s) => s.route)]));
    // Review is one job: triaging incoming community input.
    expect(byId.review).toEqual(["queue", "promotions"]);
    // Maintenance: the collection operations. Enrichment is admin-gated,
    // so staff see two entries here (task 448).
    expect(byId.maintenance).toEqual(["duplicates", "withdrawals"]);
    // Reports: getting information out.
    expect(byId.reports).toEqual(["exports", "audit", "diversity"]);
    // A menu is a home, not a hiding place: every menued screen keeps its
    // palette entry and chord reachability by staying in SCREENS.
    for (const m of menus) {
      for (const s of m.screens) {
        expect(SCREENS).toContain(s);
        expect(s.chord, `${s.route} lost its chord`).not.toBeNull();
      }
    }
    // No screen is both menued and Settings-grouped.
    for (const s of SCREENS.filter((x) => x.menu)) {
      expect(s.group, `${s.route} is both menued and grouped`).toBeUndefined();
    }
    // Admins additionally see Enrichment under Maintenance -- an
    // operations screen, not Settings material (task 448).
    const adminMaint = navMenus(true).find((m) => m.id === "maintenance")!;
    expect(adminMaint.screens.map((s) => s.route)).toEqual(["duplicates", "withdrawals", "enrichment"]);
  });

  it("sections the settings menu and gates administration by role", () => {
    const staff = settingsScreens(false);
    const admin = settingsScreens(true);
    // Per-user preferences show for everyone.
    expect(staff.prefs.map((s) => s.route)).toContain("macros");
    // Instance configuration: admins see all of it; staff see only the
    // non-adminOnly entries (Vocabularies is librarian-usable).
    expect(admin.admin.map((s) => s.route)).toEqual(
      expect.arrayContaining(["vocabsources", "profiles", "suggestions"]),
    );
    // Enrichment left Settings for the Maintenance menu (task 448).
    expect(admin.admin.map((s) => s.route)).not.toContain("enrichment");
    expect(staff.admin.map((s) => s.route)).toEqual(["vocabsources"]);
    // The menu is a home, not a hiding place: every grouped screen keeps
    // its palette entry and chord reachability by staying in SCREENS.
    for (const s of [...admin.prefs, ...admin.admin]) {
      expect(SCREENS).toContain(s);
    }
  });

  it("marks a detail route as current on its parent's link", () => {
    const works = SCREENS.find((s) => s.route === "works")!;
    expect(isCurrent(works, "works")).toBe(true);
    expect(isCurrent(works, "work")).toBe(true);
    expect(isCurrent(works, "queue")).toBe(false);
    const authorities = SCREENS.find((s) => s.route === "authorities")!;
    expect(isCurrent(authorities, "authority")).toBe(true);
  });
});
