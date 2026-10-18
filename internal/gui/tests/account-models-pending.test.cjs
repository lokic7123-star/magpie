// An account/key model choice has its own Save. The provider Save must not
// close with success while leaving a checked account choice unsaved.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { test } = require('node:test');
const { chromium, webkit } = require('playwright');
const assets = path.resolve(__dirname, '../assets');
const SG = 'deepseek-v4.1-flash-sg', FREE = 'deepseek-v4.1-flash';
const warning = "Save or cancel the account's model selection first.";

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ['chromium', 'webkit'])) {
  for (const lang of ['en', 'zh', 'zh-TW', 'ja', 'de']) {
    for (const kind of ['subscription', 'key']) {
      test(`${engine} ${lang} ${kind}: provider Save preserves pending account models`, async t => {
        const browser = await (engine === 'webkit' ? webkit.launch() : chromium.launch({ channel: process.env.PLAYWRIGHT_CHANNEL || 'chromium' }));
        t.after(() => browser.close());
        const page = await browser.newPage({ viewport: { width: lang === 'en' ? 1000 : 560, height: 800 }, reducedMotion: 'reduce' });
        page.setDefaultTimeout(6000);
        const posts = [], errors = [];
        page.on('pageerror', e => errors.push(e.message));
        const refs = kind === 'subscription' ? ['one@example.test', 'two@example.test'] : ['key-one', 'key-two'];
        const p = {
          id: kind === 'subscription' ? 'workbuddy-ai' : 'relay', name: kind === 'subscription' ? 'WorkBuddy AI' : 'Relay',
          icon: 'generic', ready: true, catalog: '', responses: '', anthropic: '', headers: {}, agents: [], fallback: [],
          chat: kind === 'subscription' ? '' : 'https://relay.example.invalid/v1',
          models: [{ id: FREE, name: 'Flash', on: true }, { id: SG, name: 'Flash SG', on: true }],
          accountModels: Object.fromEntries(refs.map(ref => [ref, [FREE]])),
          keyList: kind === 'subscription' ? [] : refs.map((id, i) => ({ id, name: id, masked: 'synthetic', on: true, active: i === 0 })),
          ...(kind === 'subscription' ? { account: { agent: 'workbuddy-ai', agentName: 'WorkBuddy AI', user: refs[0], plan: 'Free', logins: refs.map((user, i) => ({ user, plan: 'Free', active: i === 0, on: true })) } } : { key: { set: true, masked: 'synthetic' } }),
        };
        const providers = { providers: [p], presets: [], excluded: [], gateway: { running: true, window: true } };
        await page.route('**/*', async route => {
          const url = new URL(route.request().url());
          if (url.hostname !== 'magpie.test') throw new Error('Unexpected external request');
          const json = data => route.fulfill({ json: data });
          if (url.pathname === '/boot.js') return route.fulfill({ contentType: 'text/javascript', body: `window.bootPrefs={lang:${JSON.stringify(lang)},theme:"light",web:true};` });
          if (url.pathname === '/wails/runtime.js') return route.fulfill({ contentType: 'text/javascript', body: 'export const Window={};' });
          if (url.pathname === '/api/state') return json({ agents: [], profiles: [], settings: { lang, theme: 'light' } });
          if (url.pathname === '/api/providers') return json(providers);
          if (url.pathname === '/api/groups') return json({ groups: [] });
          if (url.pathname === '/api/plugins') return json({ plugins: [] });
          if (url.pathname.startsWith('/api/') && route.request().method() === 'POST') {
            const body = route.request().postDataJSON(); posts.push({ path: url.pathname, body });
            if (url.pathname === '/api/provider/accountmodels') p.accountModels[body.account] = body.allow;
            if (url.pathname === '/api/provider/save' && body.models) for (const m of p.models) m.on = body.models.includes(m.id);
            return json(providers);
          }
          if (url.pathname.startsWith('/api/')) return json({});
          const file = path.join(assets, url.pathname === '/' ? 'index.html' : url.pathname);
          return route.fulfill({ body: await fs.readFile(file), contentType: { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.png': 'image/png' }[path.extname(file)] });
        });
        await page.goto('http://magpie.test/?view=providers');
        const words = await page.evaluate(w => ({ save: t('Save'), cancel: t('Cancel'), discard: t('Discard'), warning: t(w) }), warning);
        const open = () => page.locator('.row.provider', { hasText: p.name }).first().click();
        const row = ref => page.locator(`.editor .acc[data-account-id="${ref}"]`);
        const panel = () => row(refs[0]).locator('.acct-models');
        const chip = () => panel().locator('.mchip', { hasText: 'Flash SG' });
        const outer = name => page.locator('.editor > .bar').getByRole('button', { name, exact: true });
        await open();
        await row(refs[0]).locator('.amodels').click();
        // An unchanged open picker does not prevent the provider Save.
        await outer(words.save).click();
        await page.locator('.editor').waitFor({ state: 'detached' });
        posts.length = 0;
        await open();
        if (!await panel().count()) await row(refs[0]).locator('.amodels').click();
        await chip().click();
        assert.match(await chip().getAttribute('class'), /\bon\b/, 'SG visibly checked');
        await outer(words.save).click();
        await page.getByText(words.warning, { exact: true }).first().waitFor();
        assert.deepEqual(posts, [], 'No provider or account write before resolving pending choices');
        assert.match(await chip().getAttribute('class'), /\bon\b/, 'Blocked save keeps SG checked');
        assert.deepEqual(p.accountModels[refs[0]], [FREE], 'No implicit account permission expansion');
        // Switching the open account picker also keeps those unsaved choices.
        await row(refs[1]).locator('.amodels').click();
        assert.equal(await panel().count(), 1);
        assert.match(await chip().getAttribute('class'), /\bon\b/);
        // Canceling the discard question preserves the pending selection.
        await outer(words.cancel).click();
        await page.getByRole('alertdialog').getByRole('button', { name: words.cancel, exact: true }).click();
        assert.match(await chip().getAttribute('class'), /\bon\b/);
        await panel().getByRole('button', { name: words.save, exact: true }).click();
        await panel().waitFor({ state: 'detached' });
        await outer(words.save).click();
        await page.locator('.editor').waitFor({ state: 'detached' });
        assert.deepEqual(posts.map(x => x.path), ['/api/provider/accountmodels', '/api/provider/save']);
        assert.deepEqual(p.accountModels[refs[0]], [FREE, SG]);
        assert.deepEqual(p.accountModels[refs[1]], [FREE], 'Other account remains unchanged');
        // Discard really clears the picker draft on reopening.
        await open();
        await row(refs[0]).locator('.amodels').click();
        await chip().click();
        await outer(words.cancel).click();
        await page.getByRole('alertdialog').getByRole('button', { name: words.discard, exact: true }).click();
        await page.locator('.editor').waitFor({ state: 'detached' });
        await open();
        assert.equal(await panel().count(), 0, 'Discarded picker draft cleared');
        await row(refs[0]).locator('.amodels').click();
        assert.match(await chip().getAttribute('class'), /\bon\b/, 'Saved choice survives reopen');
        assert.deepEqual(errors, []);
      });
    }
  }
}
