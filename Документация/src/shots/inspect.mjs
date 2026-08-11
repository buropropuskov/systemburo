/**
 * Разведка экрана: что на нём есть и как оно подписано.
 *
 * Подписи полей и кнопок в руководстве обязаны совпадать с экраном дословно, а
 * писать их по памяти или по комментарию в коде нельзя - на этом проекте
 * комментарий уже расходился с поведением. Этот скрипт снимает подписи с живого
 * стенда: заголовки, кнопки, поля с их подписями и метками, а также опорные
 * признаки элементов, по которым потом пишется манифест кадров.
 *
 * Запуск:
 *   node Документация/src/shots/inspect.mjs --role=user --route=/news
 *   node Документация/src/shots/inspect.mjs --role=user --route=/ --anon
 *   node Документация/src/shots/inspect.mjs --role=user --route=/news --click='[data-testid="ob-work-modes"]'
 */

import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

import { openBrowser, newContext, signIn } from './lib/session.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));

function arg(name, fallback) {
  const found = process.argv.find((value) => value.startsWith(`--${name}=`));
  if (found) return found.slice(name.length + 3);
  return process.argv.includes(`--${name}`) ? true : fallback;
}

const DUMP = () => {
  const seen = new Set();
  const visible = (element) => {
    const rect = element.getBoundingClientRect();
    if (rect.width < 4 || rect.height < 4) return false;
    const style = getComputedStyle(element);
    return style.visibility !== 'hidden' && style.opacity !== '0';
  };
  const clean = (text) => (text || '').replace(/\s+/g, ' ').trim().slice(0, 120);

  const rows = [];
  const push = (kind, element, label, extra = '') => {
    const testid = element.getAttribute('data-testid') || '';
    const key = `${kind}|${label}|${testid}`;
    if (seen.has(key)) return;
    seen.add(key);
    const rect = element.getBoundingClientRect();
    rows.push(
      [
        kind,
        label,
        testid ? `[data-testid="${testid}"]` : '',
        extra,
        `${Math.round(rect.width)}x${Math.round(rect.height)}`,
      ].join('\t'),
    );
  };

  document.querySelectorAll('h1, h2, h3, h4').forEach((element) => {
    if (visible(element)) push(`ЗАГОЛОВОК ${element.tagName}`, element, clean(element.textContent));
  });

  document.querySelectorAll('button, a[href], [role="button"]').forEach((element) => {
    if (!visible(element)) return;
    const label = clean(element.textContent) || clean(element.getAttribute('aria-label')) ||
      clean(element.getAttribute('title'));
    if (label) push('КНОПКА', element, label, element.disabled ? 'неактивна' : '');
  });

  // Опорные признаки: по ним пишется манифест кадров. Радиус нужен, чтобы
  // заранее видеть, какое скругление получит обводка.
  document.querySelectorAll('[data-testid]').forEach((element) => {
    if (!visible(element)) return;
    const style = getComputedStyle(element);
    push('ОПОРА', element, element.getAttribute('data-testid'), `радиус=${style.borderTopLeftRadius}`);
  });

  document.querySelectorAll('input, textarea, select').forEach((element) => {
    if (!visible(element)) return;
    const byFor = element.id ? document.querySelector(`label[for="${CSS.escape(element.id)}"]`) : null;
    const wrapping = element.closest('label');
    const label =
      clean(byFor?.textContent) ||
      clean(wrapping?.textContent) ||
      clean(element.getAttribute('aria-label')) ||
      '(без подписи)';
    const attrs = [
      element.tagName === 'INPUT' ? `тип=${element.type}` : element.tagName.toLowerCase(),
      element.placeholder ? `подсказка="${clean(element.placeholder)}"` : '',
      element.required ? 'обязательное' : '',
      element.maxLength > 0 ? `макс=${element.maxLength}` : '',
      element.disabled ? 'неактивно' : '',
    ].filter(Boolean);
    push('ПОЛЕ', element, label, attrs.join(', '));
  });

  return rows;
};

async function main() {
  const role = arg('role', 'user');
  const route = arg('route', '/news');
  const anon = arg('anon', false);
  const click = arg('click', null);
  const baseUrl = arg('base', 'http://localhost:5199');
  const apiBase = arg('api', 'http://localhost:8095/api');

  const accounts = JSON.parse(await readFile(path.join(HERE, 'accounts.json'), 'utf8'));
  const browser = await openBrowser();
  const context = await newContext(browser);

  try {
    let page;
    if (anon) {
      page = await context.newPage();
      await page.goto(`${baseUrl}${route}`);
    } else {
      const account = accounts.roles[role];
      if (!account) throw new Error(`в accounts.json нет роли ${role}`);
      page = await signIn(
        context,
        { username: account.username, password: accounts.password },
        { baseUrl, apiBase, clockAt: null, keepConsent: account.keepConsent === true },
      );
      if (route !== '/') await page.goto(`${baseUrl}${route}`);
    }

    await page.waitForLoadState('domcontentloaded');
    if (click) {
      await page.locator(click).first().click();
    }
    // Списки подтягиваются запросами: без паузы разведка видит пустой экран.
    await page.waitForTimeout(2500);

    console.log(`URL: ${page.url()}`);
    for (const row of await page.evaluate(DUMP)) console.log(row);
  } finally {
    await context.close();
    await browser.close();
  }
}

main().catch((error) => {
  console.error(`Разведка не удалась: ${error.message}`);
  process.exit(1);
});
