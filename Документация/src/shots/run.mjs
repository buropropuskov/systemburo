/**
 * Съёмка снимков экрана для руководств.
 *
 * Читает манифест документа, проходит по кадрам, обводит нужные элементы и
 * складывает готовые PNG в src/screenshots/<документ>/.
 *
 * Запуск (из каталога frontend, чтобы нашёлся playwright):
 *   node ../Документация/src/shots/run.mjs --doc=user
 *   node ../Документация/src/shots/run.mjs --doc=user --only=news-guide
 *
 * Возвращает ненулевой код, если хоть один кадр дал предупреждение: срезанная
 * выноска и слишком высокий кадр - это брак, который иначе всплывёт уже в
 * собранном документе.
 */

import { mkdir, readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

import { drawOutlines, drawBadges, clearOutlines } from './lib/highlight.mjs';
import { computeClip, normalize } from './lib/capture.mjs';
import { openBrowser, signIn, calmPage } from './lib/session.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const DOCS_ROOT = path.resolve(HERE, '..', '..');

function arg(name, fallback) {
  const found = process.argv.find((value) => value.startsWith(`--${name}=`));
  return found ? found.slice(name.length + 3) : fallback;
}

/**
 * Момент, на котором фиксируются часы. Берётся начало текущего часа: снимки,
 * отснятые подряд, показывают одно и то же время, а маркер доступа при этом
 * остаётся действующим - его срок отсчитывается от настоящего момента, который
 * всегда позже.
 */
function clockMoment() {
  const now = new Date();
  now.setMinutes(0, 0, 0);
  return now;
}

/** Выполняет подготовительные действия перед кадром. */
async function prepare(page, steps) {
  for (const step of steps ?? []) {
    if (step.click) await page.locator(step.click).nth(step.nth ?? 0).click();
    else if (step.fill) await page.locator(step.fill[0]).fill(step.fill[1]);
    else if (step.select) await page.locator(step.select[0]).selectOption(step.select[1]);
    else if (step.press) await page.keyboard.press(step.press);
    else if (step.hover) await page.locator(step.hover).nth(step.nth ?? 0).hover();
    else if (step.wait) await page.locator(step.wait).first().waitFor({ state: 'visible' });
    else if (step.waitHidden) await page.locator(step.waitHidden).first().waitFor({ state: 'hidden' });
    else if (step.scrollTo) {
      await page.locator(step.scrollTo).first().scrollIntoViewIfNeeded();
    } else throw new Error(`неизвестное действие подготовки: ${JSON.stringify(step)}`);
  }
}

async function shoot(page, shot, outDir) {
  if (shot.goto) {
    await page.goto(new URL(shot.goto, page.url()).href);
  }
  if (shot.waitFor) {
    await page.locator(shot.waitFor).first().waitFor({ state: 'visible', timeout: 20000 });
  }
  await prepare(page, shot.prepare);
  if (shot.calm !== false) await calmPage(page);

  const targets = shot.highlight ?? [];
  const { boxes, warnings: outlineWarnings } = await drawOutlines(page, targets);

  const { clip, warnings: clipWarnings } = await computeClip(page, shot.clip, targets);

  const badgeWarnings = await drawBadges(page, boxes, clip);

  const file = path.join(outDir, `${shot.id}.png`);
  await page.screenshot({ path: file, clip, animations: 'disabled' });
  await clearOutlines(page);
  await normalize(file);

  return [...outlineWarnings, ...clipWarnings, ...badgeWarnings].map(
    (text) => `${shot.id}: ${text}`,
  );
}

async function main() {
  const doc = arg('doc');
  if (!doc) throw new Error('не задан документ: --doc=<ключ>');
  const only = arg('only');
  const baseUrl = arg('base', 'http://localhost:5199');
  const apiBase = arg('api', 'http://localhost:8095/api');

  const accounts = JSON.parse(await readFile(path.join(HERE, 'accounts.json'), 'utf8'));
  const manifest = JSON.parse(await readFile(path.join(HERE, `${doc}.json`), 'utf8'));

  const shots = manifest.shots.filter((shot) => !only || shot.id === only);
  if (shots.length === 0) throw new Error(`в манифесте ${doc} нет кадров${only ? ` с именем ${only}` : ''}`);

  const outDir = path.join(DOCS_ROOT, 'src', 'screenshots', doc);
  await mkdir(outDir, { recursive: true });

  const { browser, context } = await openBrowser();
  const clockAt = clockMoment();
  const pages = new Map();
  const warnings = [];

  try {
    for (const shot of shots) {
      const role = shot.role ?? manifest.role;
      if (!pages.has(role)) {
        const account = accounts.roles[role];
        if (!account) throw new Error(`в accounts.json нет роли ${role}`);
        pages.set(
          role,
          await signIn(
            context,
            { username: account.username, password: accounts.password },
            { baseUrl, apiBase, clockAt },
          ),
        );
      }
      const shotWarnings = await shoot(pages.get(role), shot, outDir);
      warnings.push(...shotWarnings);
      console.log(`${shotWarnings.length ? '!' : '+'} ${shot.id}`);
    }
  } finally {
    await context.close();
    await browser.close();
  }

  console.log(`\nОтснято кадров: ${shots.length}, каталог: ${path.relative(DOCS_ROOT, outDir)}`);
  if (warnings.length) {
    console.error(`\nПредупреждения (${warnings.length}):`);
    for (const warning of warnings) console.error(`  ${warning}`);
    process.exit(1);
  }
}

main().catch((error) => {
  console.error(`Съёмка не удалась: ${error.message}`);
  process.exit(1);
});
