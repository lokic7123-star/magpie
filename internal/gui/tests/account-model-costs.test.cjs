// Run with Node's test runner and Playwright on the module path; see README.md.
// Each account of a subscription, and each key, can be kept for some of the
// provider's models only (#474: 增加账号级别的模型选项). Its row has a badge:
// one kept for some says how many, always shown; one serving every model
// says All models, on hover. A click opens the provider's models under the
// row as chips; Save posts provider/accountmodels with the account (or the
// key's id) and the models picked, All models posts none. A click moves
// nothing and the page stays where it is. A subscription with one account
// has no badge. In English and Chinese.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const fresh = () => ({
  codex: {
    id: "codex", name: "Codex", icon: "codex-color", chat: "", responses: "", anthropic: "", catalog: "",
    models: [{ id: "gpt-6", name: "", on: true }, { id: "gpt-6-mini", name: "", on: true }, { id: "gpt-5", name: "", on: false }],
    agents: [], fallback: [], headers: {}, keyList: [], proxy: "",
    account: {
      agent: "codex", agentName: "Codex", user: "me@example.com", plan: "PLUS",
      logins: [
        { user: "me@example.com", plan: "PLUS", active: true, on: true },
        { user: "Spare@Example.com", plan: "PLUS", on: true },
      ],
    },
    accountModels: { "spare@example.com": ["gpt-6-mini"] },
  },
  claude: {
    id: "claude", name: "Claude", icon: "claude-color", chat: "", responses: "", anthropic: "", catalog: "",
    models: [{ id: "claude-opus-5", name: "", on: true }], agents: [], fallback: [], headers: {}, keyList: [], proxy: "",
    account: { agent: "claude", agentName: "Claude Code", user: "solo@example.com", plan: "MAX", logins: [{ user: "solo@example.com", plan: "MAX", active: true, on: true }] },
  },
  relay: {
    id: "relay", name: "Relay", icon: "generic", chat: "https://relay.example.com/v1", responses: "", anthropic: "", catalog: "",
    models: [{ id: "big", name: "", on: true }, { id: "cheap", name: "Cheap one", on: true }], agents: [], fallback: [], headers: {},
    key: { set: true, masked: "sk-…one" }, balanceToken: { takes: false, set: false }, proxy: "",
    keyList: [{ id: "aaaaaaaaaa", name: "Team", masked: "sk-…one", on: true, active: true }, { id: "bbbbbbbbbb", name: "", masked: "sk-…two", on: true }],
  },
});

function serve(lang, posts) {
  const ps = fresh(); ps.relay.models[0].rate = 0.03; ps.relay.models[1].free = true;
  const providers = { providers: [ps.codex, ps.claude, ps.relay], presets: [], excluded: [], gateway: { running: true, window: true } };
  const state = { agents: [], profiles: [], settings: { lang, theme: "light" } };
  return async (route) => {
    const url = new URL(route.request().url());
    const json = (data) => route.fulfill({ json: data });
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:true};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json(state);
    if (url.pathname === "/api/providers") return json(providers);
    if (url.pathname === "/api/groups") return json({ groups: [] });
    if (url.pathname === "/api/plugins") return json({ plugins: [] });
    if (url.pathname === "/api/provider/accountmodels" && route.request().method() === "POST") {
      const body = route.request().postDataJSON();
      posts.push(body);
      const p = providers.providers.find((x) => x.id === body.id);
      const am = { ...(p.accountModels || {}) };
      if (body.allow.length) am[body.account.toLowerCase()] = body.allow;
      else delete am[body.account.toLowerCase()];
      p.accountModels = am;
      return json(providers);
    }
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    await route.fulfill({ body: await fs.readFile(file), contentType });
  };
}


for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
 for (const lang of ["en", "zh", "zh-TW", "ja", "de"]) {
  test(`${engine} ${lang}: account chips preserve free and credit rates`, async (t) => {
   const browser=await (engine==="webkit"?webkit.launch():chromium.launch({channel:"chromium"}));
   t.after(()=>browser.close());
   const page=await browser.newPage({viewport:{width:900,height:760},reducedMotion:"reduce"});
   page.setDefaultTimeout(5000);
   const errors=[];page.on("pageerror",e=>errors.push(e.message));
   await page.route("**/*",serve(lang,[]));
   await page.goto("http://magpie.test/?view=providers");
   await page.locator(".row.provider",{hasText:"Relay"}).first().click();
   await page.locator('.acc[data-account-id="aaaaaaaaaa"]').hover();
   await page.locator('.acc[data-account-id="aaaaaaaaaa"] .amodels').click();
   const panel=page.locator('.acc[data-account-id="aaaaaaaaaa"] .acct-models');
   await panel.locator('.mchip',{hasText:"Cheap one"}).locator('.badge.free').waitFor();
   assert.match(await panel.locator('.mchip',{hasText:"big"}).textContent(),/0[.,]03/);
   assert.equal(await panel.locator('.badge.free').count(),1);
   assert.deepEqual(errors,[]);
  });
 }
}
