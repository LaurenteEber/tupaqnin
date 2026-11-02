// UI tests for the home (P1, E.6), run with `pnpm test`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  accountErrorText,
  activityAgo,
  CANNOT_CREATE,
  cannotCreateKey,
  createErrorKey,
  homeNoticeKeys,
  homeView,
  leftBottom,
  loginNoticeKey,
  projectLineTexts,
  projectNameText,
  projectProgress,
} from "./home.ts";

const catalog = (lang: string): Record<string, string> =>
  JSON.parse(readFileSync(new URL(`./i18n/${lang}.json`, import.meta.url), "utf8"));

const session = (over: Partial<{ account: string; can_create: boolean; cannot_create: string }> = {}) => ({
  account: "",
  can_create: true,
  cannot_create: "",
  ...over,
});

const wantedTexts: Record<string, [RegExp, RegExp]> = {
  account_required: [/cuenta/, /account/],
  account_limit: [/3/, /3/],
  link_full: [/link/, /link/],
  link_budget: [/saldo/, /balance/],
  day_cap: [/hoy/, /today/],
  disk_full: [/espacio/, /space/],
};

test("E.6: there is a text for each reason the row names, and no other", () => {
  assert.deepEqual([...CANNOT_CREATE].sort(), Object.keys(wantedTexts).sort());
  for (const [reason, [es, en]] of Object.entries(wantedTexts)) {
    const key = cannotCreateKey(reason);
    assert.match(catalog("es")[key] ?? "", es, `es: ${key}`);
    assert.match(catalog("en")[key] ?? "", en, `en: ${key}`);
  }
});

test("E.6: the notices give the reason you cannot create", () => {
  for (const reason of CANNOT_CREATE) {
    const keys = homeNoticeKeys(session({ can_create: false, cannot_create: reason, account: "paula" }));
    assert.deepEqual(keys, [cannotCreateKey(reason)]);
  }
  assert.deepEqual(homeNoticeKeys(session({ account: "paula" })), []);
});

test("Eber, 2-nov: without an account the notice says the project is lost with the browser", () => {
  const keys = homeNoticeKeys(session());
  assert.deepEqual(keys, ["home.notice.no_account"]);
  assert.equal(
    catalog("es")["home.notice.no_account"],
    "sin cuenta, tu proyecto se pierde al cerrar el navegador · descárgalo o crea tu usuario para conservarlo",
  );
  assert.match(catalog("en")["home.notice.no_account"], /browser/);
  assert.deepEqual(homeNoticeKeys(session({ can_create: false, cannot_create: "account_required" })), [
    "home.notice.no_account",
    "home.cannot_create.account_required",
  ]);
  for (const lang of ["en", "es"]) {
    for (const text of Object.values(catalog(lang))) {
      assert.doesNotMatch(text, /7 d[ií]as|7 days|vence en/, lang);
    }
  }
});

test("E.6: with no_session the home shows «Entrar» and not the list", () => {
  const view = homeView(null);
  assert.equal(view.needsLink, true);
  assert.equal(view.login, true);
  assert.equal(view.list, false);
  assert.equal(view.start, false);
  assert.equal(view.register, false);
  assert.equal(view.logout, false);
  assert.deepEqual(homeNoticeKeys(null), []);
  assert.equal(catalog("es")["home.login"], "Entrar");
  assert.equal(catalog("es")["home.no_link"], "Necesitas un link de invitación");
  assert.ok(catalog("en")["home.login"]);
  assert.ok(catalog("en")["home.no_link"]);

  // The screen obeys the function: the list and the buttons only behind it.
  const page = readFileSync(new URL("./components/HomePage.tsx", import.meta.url), "utf8");
  assert.match(page, /view\.list && /);
  assert.match(page, /view\.start && /);
  assert.match(page, /view\.needsLink && /);
  assert.match(page, /view\.login && /);
});

test("E.6: a session shows what it may do", () => {
  const anon = homeView(session());
  assert.deepEqual(
    [anon.needsLink, anon.start, anon.list, anon.register, anon.login, anon.logout],
    [false, true, true, true, true, false],
  );
  const mine = homeView(session({ account: "paula" }));
  assert.deepEqual(
    [mine.needsLink, mine.start, mine.list, mine.register, mine.login, mine.logout],
    [false, true, true, false, false, true],
  );
});

test("E.5: each code a form can get has its text, and the wait says how long", () => {
  const codes = [
    "username_taken",
    "link_accounts_full",
    "invalid_credentials",
    "already_registered",
    "invalid_username",
    "invalid_password",
    "too_many_attempts",
    "no_session",
    "internal",
  ];
  for (const code of codes) {
    const { key } = accountErrorText(code, 12);
    assert.equal(key, `home.account.error.${code}`);
    for (const lang of ["en", "es"]) assert.ok(catalog(lang)[key], `${lang}: ${key}`);
  }
  const wait = accountErrorText("too_many_attempts", 12);
  assert.deepEqual(wait.params, { seconds: "12" });
  for (const lang of ["en", "es"]) assert.match(catalog(lang)[wait.key], /\{seconds\}/, lang);
  // Whatever else comes back is never shown raw.
  assert.equal(accountErrorText("http_413").key, "home.account.error.internal");
});

test("E.6: a refused create names its reason once; the rest is generic", () => {
  assert.equal(createErrorKey("link_budget", null), "home.cannot_create.link_budget");
  // The notice already says it.
  assert.equal(createErrorKey("link_budget", session({ can_create: false, cannot_create: "link_budget" })), null);
  assert.equal(createErrorKey("no_session", null), "home.no_link");
  assert.equal(createErrorKey("template_unknown", null), "home.error.internal");
  assert.equal(createErrorKey("", null), null);
  for (const lang of ["en", "es"]) assert.ok(catalog(lang)["home.error.internal"], lang);
});

test("P1: «hace 2 h» comes from last_activity, in both catalogs", () => {
  const now = Date.parse("2026-11-02T12:00:00.000Z");
  const cases: [string, string, string | undefined][] = [
    ["2026-11-02T11:59:40.000Z", "home.ago.now", undefined],
    ["2026-11-02T11:15:00.000Z", "home.ago.minutes", "45"],
    ["2026-11-02T10:00:00.000Z", "home.ago.hours", "2"],
    ["2026-10-30T12:00:00.000Z", "home.ago.days", "3"],
    // A clock a little behind the server's is not «in the future».
    ["2026-11-02T12:00:30.000Z", "home.ago.now", undefined],
  ];
  for (const [at, key, n] of cases) {
    const got = activityAgo(at, now);
    assert.equal(got.key, key, at);
    assert.equal(got.params?.n, n, at);
    for (const lang of ["en", "es"]) assert.ok(catalog(lang)[key], `${lang}: ${key}`);
  }
});

test("P1: «1 de 3 secciones firmadas», as Go counted them", () => {
  assert.deepEqual(projectProgress({ signed: 1, sections: 3 }), {
    key: "home.project.signed",
    params: { signed: "1", sections: "3" },
  });
  assert.deepEqual(projectProgress({ signed: 0, sections: 0 }), { key: "home.project.no_sections" });
  assert.equal(
    catalog("es")["home.project.signed"].replace("{signed}", "1").replace("{sections}", "3"),
    "1 de 3 secciones firmadas",
  );
  for (const lang of ["en", "es"]) {
    assert.ok(catalog(lang)["home.project.no_sections"], lang);
    assert.ok(catalog(lang)["home.project.no_account"], lang);
  }
});

test("Every home key is in both catalogs, and the two catalogs have the same", () => {
  const es = Object.keys(catalog("es")).filter((k) => k.startsWith("home."));
  const en = Object.keys(catalog("en")).filter((k) => k.startsWith("home."));
  assert.deepEqual(es.sort(), en.sort());
  const page = readFileSync(new URL("./components/HomePage.tsx", import.meta.url), "utf8");
  for (const [, key] of page.matchAll(/t\("(home\.[a-z_.]+)"/g)) {
    assert.ok(en.includes(key), key);
  }
});

test("B-91: the login form warns only while you hold a project without an account", () => {
  assert.equal(loginNoticeKey([]), null);
  assert.equal(loginNoticeKey([{ no_account: false }]), null);
  assert.equal(loginNoticeKey([{ no_account: false }, { no_account: true }]), "home.login_moves_project");
  assert.match(catalog("es")["home.login_moves_project"] ?? "", /3/);
  assert.match(catalog("en")["home.login_moves_project"] ?? "", /3/);
});

// U.2: the left panel's list and its bottom.
const NOW = Date.parse("2026-11-02T12:00:00Z");

test("U.2: a project's line says when, how many sections and how many signed", () => {
  const p = { name: "Tesis", last_activity: "2026-11-02T10:00:00Z", sections: 3, signed: 1, no_account: false };
  assert.deepEqual(projectLineTexts(p, NOW), [
    { key: "home.ago.hours", params: { n: "2" } },
    { key: "left.project.sections", params: { n: "3" } },
    { key: "left.project.signed_one" },
  ]);
});

test("U.2: one section reads in the singular, and none says so", () => {
  const base = { name: "", last_activity: "2026-11-02T11:59:30Z", signed: 0, no_account: false };
  assert.deepEqual(projectLineTexts({ ...base, sections: 1 }, NOW).slice(1), [
    { key: "left.project.section_one" },
    { key: "left.project.signed", params: { n: "0" } },
  ]);
  assert.equal(catalog("es")["left.project.signed_one"], "1 firmada");
  assert.deepEqual(projectLineTexts({ ...base, sections: 0 }, NOW), [{ key: "home.ago.now" }, { key: "home.project.no_sections" }]);
});

test("U.2: a project without an account says it on its line", () => {
  const p = { name: "", last_activity: "2026-11-02T11:00:00Z", sections: 0, signed: 0, no_account: true };
  assert.deepEqual(projectLineTexts(p, NOW).at(-1), { key: "home.project.no_account" });
});

test("U.2 (rule 7): a project without a name shows the catalog's name, never its id", () => {
  assert.deepEqual(projectNameText({ name: "" }), { key: "project.untitled" });
  assert.equal(projectNameText({ name: "Tesis" }), "Tesis");
});

test("U.2 (B3-18): without an account, the bottom warns, and offers to register and enter", () => {
  const session = { account: "", can_create: true, cannot_create: "" };
  const open = leftBottom(session, true);
  assert.equal(open.notice, true);
  assert.equal(open.noticeDownload, true);
  assert.equal(open.register, true);
  assert.equal(open.login, true);
  assert.equal(open.logout, false);
});

test("U.2 (B3-18): with no project open, «descárgalo» does not appear", () => {
  const closed = leftBottom({ account: "", can_create: true, cannot_create: "" }, false);
  assert.equal(closed.notice, true);
  assert.equal(closed.noticeDownload, false);
});

test("U.2: with an account, the bottom shows the user and «Salir», and no warning", () => {
  const view = leftBottom({ account: "eber", can_create: true, cannot_create: "" }, true);
  assert.equal(view.notice, false);
  assert.equal(view.register, false);
  assert.equal(view.login, false);
  assert.equal(view.logout, true);
});

test("U.2 (B3-18): with no session, only «Entrar», and no way to start a project", () => {
  const view = leftBottom(null, false);
  assert.equal(view.notice, false);
  assert.equal(view.register, false);
  assert.equal(view.login, true);
  assert.equal(view.start, false);
  assert.equal(leftBottom({ account: "", can_create: false, cannot_create: "day_cap" }, false).start, true);
});

test("U.2: the left panel's texts are in both catalogs", () => {
  for (const lang of ["en", "es"]) {
    const texts = catalog(lang);
    for (const key of ["left.new", "left.project.sections", "left.project.section_one", "left.project.signed", "left.project.signed_one", "left.notice.lead", "left.notice.download", "left.notice.or", "left.notice.register", "language.label"]) {
      assert.ok(texts[key], `${lang}: ${key}`);
    }
  }
});
