// Скриптовая запись демо О!дин: 1920x1080, CDP screencast -> кадры с таймстемпами.
import { chromium } from 'playwright-core';
import fs from 'node:fs';
import path from 'node:path';

const BASE = 'http://localhost:8081';
const OUT = process.argv[2] || path.resolve('frames');
const ONLY = process.argv[3] || ''; // имя сцены для отладки
const EXE = `${process.env.HOME}/Library/Caches/ms-playwright/chromium-1243/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing`;

fs.rmSync(OUT, { recursive: true, force: true });
fs.mkdirSync(OUT, { recursive: true });

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// Оверлей: курсор, клик-волна, плашка главы. Живёт через навигации.
const OVERLAY = () => {
  let fadeFlag = null;
  try { fadeFlag = sessionStorage.getItem('__demoFade'); sessionStorage.removeItem('__demoFade'); } catch (e) {}
  if (fadeFlag) {
    const pre = document.createElement('style');
    pre.id = '__demo_pre';
    pre.textContent = `html{background:${fadeFlag}!important}body{opacity:0!important}`;
    // на старте документа <html> может ещё не существовать
    const add = () => { if (!document.documentElement) return false; document.documentElement.appendChild(pre); return true; };
    if (!add()) { const mo = new MutationObserver(() => { if (add()) mo.disconnect(); }); mo.observe(document, { childList: true }); }
  }
  const install = () => {
    if (document.getElementById('__demo_cursor') || !document.body) return;
    const st = document.createElement('style');
    st.textContent = `
      #__demo_cursor{position:fixed;left:0;top:0;width:30px;height:30px;z-index:2147483647;pointer-events:none;
        transform:translate(-100px,-100px);transition:opacity .3s;filter:drop-shadow(0 3px 6px rgba(0,0,0,.35))}
      #__demo_cursor.hide{opacity:0}
      .__demo_ripple{position:fixed;z-index:2147483646;pointer-events:none;width:46px;height:46px;margin:-23px 0 0 -23px;border-radius:50%;
        background:rgba(37,84,255,.28);border:2px solid rgba(37,84,255,.65);animation:__rip .55s ease-out forwards}
      @keyframes __rip{from{transform:scale(.3);opacity:1}to{transform:scale(1.5);opacity:0}}
      #__demo_chip{position:fixed;left:40px;bottom:40px;z-index:2147483645;pointer-events:none;display:flex;align-items:center;gap:14px;
        padding:14px 24px 14px 16px;border-radius:18px;background:rgba(17,20,33,.88);color:#fff;font:600 22px/1.2 Manrope,Inter,system-ui,sans-serif;
        box-shadow:0 12px 40px rgba(0,0,0,.25);opacity:0;transform:translateY(16px);transition:opacity .45s ease,transform .45s ease;backdrop-filter:blur(8px)}
      #__demo_chip.show{opacity:1;transform:none}
      #__demo_chip b{display:inline-grid;place-items:center;min-width:36px;height:36px;border-radius:11px;background:#c6f36b;color:#111421;font-size:18px}
      #__demo_chip small{display:block;font-weight:500;font-size:15px;opacity:.7;margin-top:3px}
    `;
    document.head.appendChild(st);
    const c = document.createElement('div');
    c.id = '__demo_cursor';
    c.innerHTML = `<svg viewBox="0 0 24 24" width="30" height="30"><path d="M4 2.5l15.5 9.2-6.9 1.6 3.9 7.4-2.9 1.5-3.9-7.5L4 19.6z" fill="#111" stroke="#fff" stroke-width="1.6" stroke-linejoin="round"/></svg>`;
    document.body.appendChild(c);
    const chip = document.createElement('div');
    chip.id = '__demo_chip';
    document.body.appendChild(chip);
    const pos = window.__demoPos || { x: -100, y: -100 };
    c.style.transform = `translate(${pos.x}px,${pos.y}px)`;
    document.addEventListener('mousemove', (e) => {
      window.__demoPos = { x: e.clientX, y: e.clientY };
      c.style.transform = `translate(${e.clientX - 4}px,${e.clientY - 2}px)`;
    }, true);
    document.addEventListener('mousedown', (e) => {
      const r = document.createElement('div');
      r.className = '__demo_ripple';
      r.style.left = e.clientX + 'px';
      r.style.top = e.clientY + 'px';
      document.body.appendChild(r);
      setTimeout(() => r.remove(), 700);
    }, true);
  };
  const fadeIn = () => {
    if (!fadeFlag) return;
    const reveal = () => {
      const pre = document.getElementById('__demo_pre');
      document.body.style.transition = 'opacity .6s ease';
      if (pre) pre.remove();
      setTimeout(() => { document.body.style.transition = ''; }, 800);
    };
    // ждём load и пока исчезнут скелетоны/заглушки (не дольше 3 с)
    const t0 = performance.now();
    const ready = () => {
      const busy = document.querySelector('.skeleton-card, .skeleton-line, .skeleton-block');
      if (busy && performance.now() - t0 < 3000) return setTimeout(ready, 80);
      setTimeout(reveal, 220);
    };
    document.readyState === 'complete' ? ready() : window.addEventListener('load', ready, { once: true });
  };
  window.__demoFadeOut = (color = '#f4f5f7', dur = 450) => new Promise((done) => {
    const f = document.createElement('div');
    f.style.cssText = `position:fixed;inset:0;z-index:2147483644;pointer-events:none;background:${color};opacity:0;transition:opacity ${dur}ms ease`;
    document.body.appendChild(f);
    requestAnimationFrame(() => requestAnimationFrame(() => { f.style.opacity = '1'; }));
    try { sessionStorage.setItem('__demoFade', color); } catch (e) {}
    setTimeout(done, dur + 40);
  });
  // Вуаль для смены экрана внутри SPA: прячет промежуточный кадр перерисовки.
  window.__demoVeil = (on, dur = 260) => new Promise((done) => {
    let v = document.getElementById('__demo_veil');
    if (!v) {
      v = document.createElement('div');
      v.id = '__demo_veil';
      v.style.cssText = 'position:fixed;inset:0;z-index:2147483644;pointer-events:none;background:#f4f5f7;opacity:0';
      document.body.appendChild(v);
    }
    v.style.transition = `opacity ${dur}ms ease`;
    requestAnimationFrame(() => requestAnimationFrame(() => { v.style.opacity = on ? '1' : '0'; }));
    setTimeout(done, dur + 40);
  });
  window.__demoChip = (n, title, sub) => {
    const chip = document.getElementById('__demo_chip');
    if (!chip) return;
    chip.innerHTML = `<b>${n}</b><span>${title}${sub ? `<small>${sub}</small>` : ''}</span>`;
    chip.classList.add('show');
    clearTimeout(window.__chipT);
    window.__chipT = setTimeout(() => chip.classList.remove('show'), 3800);
  };
  window.__demoGlide = (x, y, dur) => new Promise((done) => {
    const c = document.getElementById('__demo_cursor');
    if (!c) return done();
    c.style.transition = `transform ${dur}ms cubic-bezier(.45,.05,.25,1), opacity .3s`;
    c.style.transform = `translate(${x - 4}px,${y - 2}px)`;
    window.__demoPos = { x, y };
    setTimeout(() => { c.style.transition = 'opacity .3s'; done(); }, dur + 20);
  });
  window.__demoCursor = (show) => document.getElementById('__demo_cursor')?.classList.toggle('hide', !show);
  // Плавный скролл с easeInOut: окно или контейнер.
  window.__demoScroll = (target, sel, dur) => new Promise((done) => {
    const box = sel ? document.querySelector(sel) : null;
    const from = box ? box.scrollTop : window.scrollY;
    let to = target;
    if (typeof target === 'string') {
      const el = document.querySelector(target);
      const top = el.getBoundingClientRect().top;
      const base = box ? box.getBoundingClientRect().top : 0;
      to = from + top - base - (box ? 24 : 110);
    }
    const max = box ? box.scrollHeight - box.clientHeight : document.documentElement.scrollHeight - innerHeight;
    to = Math.max(0, Math.min(max, to));
    const t0 = performance.now();
    const ease = (t) => -(Math.cos(Math.PI * t) - 1) / 2;
    const step = (now) => {
      const t = Math.min(1, (now - t0) / dur);
      const y = from + (to - from) * ease(t);
      if (box) box.scrollTo({ top: y, behavior: 'instant' }); else window.scrollTo({ top: y, behavior: 'instant' });
      t < 1 ? requestAnimationFrame(step) : done();
    };
    requestAnimationFrame(step);
  });
  const boot = () => { install(); fadeIn(); };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
  else boot();
};

const browser = await chromium.launch({
  executablePath: EXE,
  headless: true,
  args: ['--hide-scrollbars', '--force-color-profile=srgb', '--font-render-hinting=none'],
});
const context = await browser.newContext({
  viewport: { width: 1920, height: 1080 },
  deviceScaleFactor: 1,
  colorScheme: 'light',
  locale: 'ru-RU',
  timezoneId: 'Europe/Moscow',
});
await context.addInitScript(OVERLAY);
const page = await context.newPage();
const errors = [];
page.on('pageerror', (e) => errors.push('pageerror: ' + e.message));
page.on('console', (m) => { if (m.type() === 'error') errors.push('console: ' + m.text()); });

// --- screencast ---
const cdp = await context.newCDPSession(page);
const frames = [];
let n = 0;
let paused = false; // во время смены экрана кадры не пишем: держится последний кадр старого экрана
cdp.on('Page.screencastFrame', async (f) => {
  if (paused) { cdp.send('Page.screencastFrameAck', { sessionId: f.sessionId }).catch(() => {}); return; }
  const file = path.join(OUT, String(n++).padStart(6, '0') + '.jpg');
  fs.writeFileSync(file, Buffer.from(f.data, 'base64'));
  frames.push({ file, ts: f.metadata.timestamp });
  cdp.send('Page.screencastFrameAck', { sessionId: f.sessionId }).catch(() => {});
});
const startCast = () => cdp.send('Page.startScreencast', { format: 'jpeg', quality: 92, maxWidth: 1920, maxHeight: 1080, everyNthFrame: 1 });

// --- курсор ---
let cur = { x: 1500, y: 900 };
async function moveTo(x, y, dur = 650) {
  // длительность от расстояния: короткие движения не «ползут», длинные не «прыгают»
  const dist = Math.hypot(x - cur.x, y - cur.y);
  const d = Math.round(Math.min(1300, Math.max(450, dur * 0.6 + dist * 0.45)));
  await page.evaluate(([a, b, t]) => window.__demoGlide(a, b, t), [x, y, d]);
  await page.mouse.move(x, y);
  cur = { x, y };
}
async function box(sel) {
  const loc = page.locator(sel).first();
  await loc.waitFor({ state: 'visible', timeout: 15000 });
  const b = await loc.boundingBox();
  return { loc, x: b.x + b.width / 2, y: b.y + b.height / 2, b };
}
async function hover(sel, dur) {
  const { x, y } = await box(sel);
  await moveTo(x, y, dur);
}
async function click(sel, { dur = 650, after = 700 } = {}) {
  const { x, y } = await box(sel);
  await moveTo(x, y, dur);
  await sleep(180);
  await page.mouse.down();
  await sleep(90);
  await page.mouse.up();
  await sleep(after);
}
async function type(sel, text, delay = 75) {
  await click(sel, { after: 250 });
  for (const ch of text) {
    await page.keyboard.type(ch);
    await sleep(delay + Math.random() * 40);
  }
}
const scroll = (target, sel = null, dur = 1400) => page.evaluate(([t, s, d]) => window.__demoScroll(t, s, d), [target, sel, Math.round(dur * 1.2)]);
const chip = (n, title, sub = '') => page.evaluate(([a, b, c]) => window.__demoChip(a, b, c), [n, title, sub]);
// Кроссфейд: снимок старого экрана -> смена экрана без записи -> снимок поверх нового растворяется.
async function crossSwitch(action, dur = 250) {
  paused = true;
  await page.evaluate(() => { const c = document.getElementById('__demo_cursor'); if (c) c.style.visibility = 'hidden'; }).catch(() => {});
  const shot = (await page.screenshot({ type: 'jpeg', quality: 92 })).toString('base64');
  await page.evaluate(() => { const c = document.getElementById('__demo_cursor'); if (c) c.style.visibility = ''; }).catch(() => {});
  await action();
  await page.waitForFunction(() => !document.querySelector('.skeleton-card, .skeleton-line, .skeleton-block'), null, { timeout: 3000 }).catch(() => {});
  await sleep(250);
  await page.evaluate(async ([b64, x, y]) => {
    const c = document.getElementById('__demo_cursor');
    if (c) { c.style.transition = 'none'; c.style.transform = `translate(${x - 4}px,${y - 2}px)`; }
    window.__demoPos = { x, y };
    const src = 'data:image/jpeg;base64,' + b64;
    const img = new Image(); img.src = src; await img.decode();
    const d = document.createElement('div');
    d.id = '__demo_xf';
    d.style.cssText = `position:fixed;inset:0;z-index:2147483646;pointer-events:none;background:url(${src}) 0 0/100% 100% no-repeat;opacity:1`;
    document.body.appendChild(d);
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
  }, [shot, cur.x, cur.y]);
  paused = false;
  await sleep(80);
  await page.evaluate((t) => {
    const d = document.getElementById('__demo_xf');
    d.style.transition = `opacity ${t}ms ease`;
    d.style.opacity = '0';
    setTimeout(() => d.remove(), t + 100);
  }, dur);
  await sleep(dur + 150);
}
const nav = (url) => crossSwitch(() => page.goto(url, { waitUntil: 'networkidle' }));
// Клик, который меняет экран, с кроссфейдом.
async function clickSwitch(sel, waitSel) {
  const { x, y } = await box(sel);
  await moveTo(x, y);
  await sleep(220);
  await crossSwitch(async () => {
    await page.mouse.down(); await sleep(60); await page.mouse.up();
    if (waitSel) await page.locator(waitSel).first().waitFor({ state: 'visible', timeout: 15000 }).catch(() => {});
    await page.evaluate(() => window.scrollTo({ top: 0, behavior: 'instant' }));
  });
}
const cursor = (show) => page.evaluate((s) => window.__demoCursor(s), show);

// --- заставка и финал ---
const INTRO = fs.readFileSync(new URL('./intro.html', import.meta.url), 'utf8');
const OUTRO = fs.readFileSync(new URL('./outro.html', import.meta.url), 'utf8');

const scenes = {
  async intro() {
    // до старта записи на экране чистый лаймовый фон (см. ниже), заставка проявляется уже в записи
    await sleep(9000);
  },

  async home() {
    if (page.url().startsWith(BASE)) await nav(BASE + '/');
    else { await page.goto(BASE + '/', { waitUntil: 'networkidle' }); await sleep(900); }
    await cursor(true);
    await chip(1, 'Главная', 'отели, настроения, ИИ-подбор');
    await moveTo(620, 520, 900);
    await sleep(1200);
    await scroll(700, null, 2200);
    await sleep(1300);
    await scroll('#dealsSection', null, 2000);
    await sleep(1300);
    await scroll('#destinationsSection', null, 1800);
    await sleep(1300);
    await scroll('#aiSection', null, 1800);
    await sleep(1500);
    await scroll(0, null, 2200);
    await moveTo(960, 640, 700);
    await sleep(600);
  },

  async search() {
    if (!(await page.locator('#searchDestination').count())) { await nav(BASE + '/'); await cursor(true); }
    await chip(2, 'Поиск по потребностям', 'фильтры, расстояния, причины отказа');
    await type('#searchDestination', 'Сочи');
    await sleep(500);
    await clickSwitch('#heroSearch button[type=submit]', '#searchGrid >> text=Подробнее');
    await sleep(900);
    await hover('#searchGrid >> text=Подробнее >> nth=0', 900);
    await sleep(900);
    await scroll(420, null, 1600);
    await sleep(1200);
    // Фильтры: набор «С детьми»
    await scroll('[data-toggle-filter-group=needs]', null, 1500);
    await sleep(700);
    await click('[data-need-preset=family]', { after: 1800 });
    await hover('[data-need=kids]', 700);
    await sleep(900);
    await scroll('#searchGrid', null, 1500);
    await sleep(1400);
    await scroll('#searchView details summary', null, 2200);
    await sleep(800);
    await click('#searchView details summary', { after: 2600 });
    await scroll('#searchGrid', null, 1600);
    await sleep(800);
  },

  async hotel() {
    await clickSwitch('#searchGrid >> text=Подробнее >> nth=0', '.match-passport');
    await chip(3, 'Карточка отеля', 'паспорт совпадения и «Что если?»');
    await sleep(900);
    await moveTo(1000, 400, 900);
    await sleep(1400);
    await scroll('.match-passport', null, 1800);
    await sleep(1500);
    await hover('.match-passport .match-title', 600);
    await sleep(1600);
    await scroll('.wi-switch[data-key=budget]', null, 1600);
    await sleep(1000);
    await click('.wi-switch[data-key=nights]', { after: 2400 });
    await click('.wi-switch[data-key=flexible]', { after: 2400 });
    await click('.wi-switch[data-key=flexible]', { after: 900 });
    // Честная цена
    await page.evaluate(() => { [...document.querySelectorAll('#hotelContent h2')].find((h) => /Из чего состоит цена/.test(h.textContent)).id = '__price'; });
    await scroll('#__price', null, 1800);
    await sleep(2600);
    await scroll('#hotelContent h3.audit-title', null, 1400);
    await sleep(2400);
    // Что рядом + карта
    await scroll('[data-nearby-map-open]', null, 1800);
    await sleep(1500);
    await click('[data-nearby-map-open]', { after: 3200 });
    await page.keyboard.press('Escape');
    await sleep(900);
  },

  async booking() {
    await chip(4, 'Бронирование', 'вход → детали → гость → демо-оплата');
    await scroll(0, null, 1800);
    await sleep(500);
    await hover('#detailRoom', 600);
    await page.selectOption('#detailRoom', '35');
    await sleep(1600);
    await click('#detailBook', { after: 900 });
    const opened = () => page.evaluate(() => !!document.querySelector('#authModal.is-open, #bookingModal.is-open'));
    if (!(await opened())) { errors.push('detailBook: fallback click'); await page.click('#detailBook'); }
    await sleep(700);
    // Вход демо-аккаунтом
    if (await page.evaluate(() => !!document.querySelector('#authModal.is-open'))) {
      await sleep(900);
      await click('#demoLoginButton', { after: 2200 });
    }
    await page.locator('#bookingNext1').waitFor({ state: 'visible', timeout: 8000 });
    await sleep(1500);
    await hover('#bookingTotalStep1', 700);
    await sleep(1800);
    await click('#bookingNext1', { after: 1300 });
    await type('#bookingPhone', '+7 999 120-44-08', 55);
    await sleep(700);
    await click('#bookingNext2', { after: 1300 });
    await type('#cardNumber', '4242 4242 4242 4242', 45);
    await type('#cardExpiry', '12 / 28', 70);
    await type('#cardCvc', '123', 90);
    await sleep(600);
    await hover('#bookingTotalStep3', 600);
    await sleep(1500);
    await click('#bookingSubmit', { after: 2600 });
    await page.locator('#bookingSuccess').waitFor({ state: 'visible', timeout: 10000 });
    await hover('#bookingSuccessCode', 700);
    await sleep(2200);
    await clickSwitch('#bookingGoTrips', '#tripsContent');
    await sleep(1500);
    await moveTo(1100, 500, 800);
    await sleep(2500);
    await scroll(260, null, 1600);
    await sleep(1800);
    await scroll(0, null, 1200);
  },

  async trip() {
    if (!(await page.locator('#tripPlanOpen').isVisible())) await nav(BASE + '/');
    await chip(5, 'ИИ-режиссёр поездки', 'отель, дни, погода и бюджет в одном плане');
    await click('#tripPlanOpen', { after: 1400 });
    await hover('[data-demo-scenario=romance]', 600);
    await sleep(400);
    await click('[data-demo-scenario=family]', { after: 3600 });
    await page.locator('#txTab0').waitFor({ timeout: 10000 });
    await hover('#tripTitle', 700);
    await sleep(1600);
    await hover('.tx-hotel', 700);
    await sleep(1400);
    await click('#txWhyBtn', { after: 1200 });
    await scroll('#txWhy', '#tripBody', 1200);
    await sleep(3200);
    await click('#txWhyBtn', { after: 600 });
    await scroll('.tx-days', '#tripBody', 1400);
    await sleep(1200);
    await click('#txTab2', { after: 1600 });
    await page.evaluate(() => {
      const b = document.querySelector('#tripBody');
      const days = document.querySelector('.tx-days');
      return window.__demoScroll(b.scrollTop + days.getBoundingClientRect().top - b.getBoundingClientRect().top + 260, '#tripBody', 1600);
    });
    await sleep(2400);
    await scroll('.tx-days', '#tripBody', 1200);
    await click('#txTab1', { after: 1400 });
    await click('#txRebuild', { after: 2800 });
    await scroll('.tx-budget', '#tripBody', 1800);
    await sleep(3600);
    await scroll('.tx-tips', '#tripBody', 1400);
    await sleep(2400);
    await scroll(0, '#tripBody', 1600);
    await sleep(600);
  },

  async share() {
    await chip(6, 'Поделиться маршрутом', 'ссылка, QR и публичная страница без входа');
    await click('#txShare', { after: 2000 });
    const href = await page.evaluate(() => [...document.querySelectorAll('a')].find((a) => /share\.html\?s=/.test(a.href) && a.offsetParent)?.href);
    await hover('a:has-text("Открыть страницу")', 700);
    await sleep(2600);
    await page.mouse.down(); await sleep(90); await page.mouse.up();
    await sleep(300);
    await nav(href);
    await sleep(900);
    await moveTo(1000, 560, 800);
    await sleep(1400);
    await scroll(700, null, 2400);
    await sleep(1800);
    await scroll(1500, null, 2400);
    await sleep(1800);
    await scroll(2600, null, 2600);
    await sleep(1600);
    await scroll(0, null, 2200);
    await sleep(800);
  },

  async assistant() {
    if (!(await page.locator('#assistantButton').isVisible()) || !(await page.locator('#homeView').isVisible())) await nav(BASE + '/');
    await chip(7, 'ИИ-консьерж', 'работает локально, отвечает действиями');
    await click('#assistantButton', { after: 1400 });
    await type('#assistantInput', 'Тихий отель у моря в Сочи для семьи с ребёнком', 55);
    await sleep(400);
    await click('#assistantSend', { after: 3800 });
    await sleep(2600);
    await type('#assistantInput', 'Сколько стоит Cedar & Sea на 4 ночи?', 55);
    await click('#assistantSend', { after: 4200 });
    await sleep(1800);
    const act = page.locator('#assistantMessages [data-assistant-action]').last();
    if (await act.count()) {
      const b = await act.boundingBox();
      await moveTo(b.x + b.width / 2, b.y + b.height / 2, 700);
      await sleep(1200);
    }
    await sleep(1200);
  },

  async outro() {
    await cursor(false);
    await sleep(300);
    await crossSwitch(() => page.setContent(OUTRO, { waitUntil: "networkidle" }), 500);
    await sleep(5500);
  },
};

const order = ONLY ? ONLY.split(',') : Object.keys(scenes);
let groupStart = false;
if (order[0] === 'intro') {
  // тот же origin, чтобы грузились шрифты; запись стартует, когда заставка уже на экране
  await page.goto(BASE + '/assets/app.css');
  // шрифты в кэш, затем заставка; запись стартует сразу, анимации появления идут уже в кадре
  await page.setContent(INTRO, { waitUntil: 'networkidle' });
  await page.evaluate(() => document.fonts.ready);
  await page.setContent(INTRO, { waitUntil: 'domcontentloaded' });
  await cursor(false).catch(() => {});
}
if (order[0] !== 'intro') {
  // Кусок: главная грузится до старта записи, запись начинается с серого кадра и мягкого проявления.
  await page.goto(BASE + '/', { waitUntil: 'networkidle' });
  await page.waitForFunction(() => !document.querySelector('.skeleton-card, .skeleton-line, .skeleton-block'), null, { timeout: 5000 }).catch(() => {});
  cur = { x: 960, y: 640 };
  await page.mouse.move(cur.x, cur.y);
  await sleep(300);
  groupStart = true;
}
await startCast();
if (groupStart) await sleep(300);
for (const name of order) {
  const t = Date.now();
  try {
    await scenes[name]();
  } catch (e) {
    errors.push(`scene ${name}: ${e.message.split('\n')[0]}`);
    console.error('SCENE FAIL', name, e.message.split('\n')[0]);
    await page.screenshot({ path: path.join(OUT, `fail-${name}.png`) }).catch(() => {});
  }
  console.log(`scene ${name}: ${((Date.now() - t) / 1000).toFixed(1)}s`);
}
await sleep(400);
await cdp.send('Page.stopScreencast');
await sleep(300);
fs.writeFileSync(path.join(OUT, 'frames.json'), JSON.stringify(frames));
fs.writeFileSync(path.join(OUT, 'errors.txt'), errors.join('\n'));
console.log('frames', frames.length, 'errors', errors.length);
errors.forEach((e) => console.log(' ', e));
await browser.close();
