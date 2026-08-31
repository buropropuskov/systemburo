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
import { computeClip, cropToClip, normalize, waitForStableRects } from './lib/capture.mjs';
import { openBrowser, newContext, signIn, calmPage, SCALE, VIEWPORT } from './lib/session.mjs';

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
    // Набор с клавиатуры - для областей редактирования, куда нельзя подставить
    // значение полем ввода: у редактора письма это не input, а разметка.
    else if (step.type) await page.keyboard.type(step.type);
    else if (step.hover) await page.locator(step.hover).nth(step.nth ?? 0).hover();
    else if (step.wait) await page.locator(step.wait).first().waitFor({ state: 'visible' });
    else if (step.waitHidden) await page.locator(step.waitHidden).first().waitFor({ state: 'hidden' });
    else if (step.scrollTo) {
      await page.locator(step.scrollTo).first().scrollIntoViewIfNeeded();
    } else throw new Error(`неизвестное действие подготовки: ${JSON.stringify(step)}`);
  }
}

/**
 * Стирает черновик заявки до загрузки приложения.
 *
 * Форма подачи хранит набранное в хранилище браузера, а окружение у роли одно на
 * всю пачку: без очистки кадр экрана подачи показывал бы вложения, добавленные
 * предыдущим кадром, и пересъёмка одного кадра давала бы не то, что пересъёмка
 * всей пачки. Ключи те же, что чистит сама форма после отправки заявки.
 */
async function clearDraft(page) {
  await page.addInitScript(() => {
    for (const key of Object.keys(localStorage)) {
      if (/draft|attachment|application/i.test(key)) localStorage.removeItem(key);
    }
  });
}

async function shoot(page, shot, outDir) {
  if (shot.waitFor) {
    await page.locator(shot.waitFor).first().waitFor({ state: 'visible', timeout: 20000 });
  }
  /*
   * Уборка - до подготовки, а не после: она закрывает всплывшее клавишей Esc,
   * и после подготовки закрывала бы ровно то окно, ради которого кадр и
   * снимается.
   */
  if (shot.calm !== false) await calmPage(page);
  await prepare(page, shot.prepare);

  const targets = shot.highlight ?? [];
  await waitForStableRects(
    page,
    [shot.clip?.selector, ...targets.map((target) => target.selector)].filter(Boolean),
  );
  const { boxes, warnings: outlineWarnings } = await drawOutlines(page, targets);

  const { clip, warnings: clipWarnings } = await computeClip(page, shot.clip, targets);

  const badgeWarnings = await drawBadges(page, boxes, clip);

  /*
   * Снимается всё окно, область вырезается потом: съёмка области силами
   * браузера приносит в кадр отражённые куски того, что лежит за наложенным
   * окном (см. cropToClip).
   */
  const file = path.join(outDir, `${shot.id}.png`);
  await page.screenshot({ path: file, animations: 'disabled' });
  await clearOutlines(page);
  await cropToClip(file, clip, SCALE);
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

  // Кадров в документе десятки, а переснимать после правки манифеста обычно
  // нужно несколько: имена перечисляются через запятую.
  const wanted = only ? new Set(only.split(',').map((name) => name.trim()).filter(Boolean)) : null;
  const shots = manifest.shots.filter((shot) => !wanted || wanted.has(shot.id));
  if (shots.length === 0) throw new Error(`в манифесте ${doc} нет кадров${only ? ` с именем ${only}` : ''}`);

  const outDir = path.join(DOCS_ROOT, 'src', 'screenshots', doc);
  await mkdir(outDir, { recursive: true });

  const browser = await openBrowser();
  const clockAt = clockMoment();
  const sessions = new Map();
  const warnings = [];

  try {
    for (const shot of shots) {
      // Страница входа снимается до входа, поэтому у такого кадра своё
      // окружение без учётной записи. Отдельный ключ, а не роль: роли в
      // accounts.json - это работники, а здесь работника ещё нет.
      const role = shot.anon ? '(без входа)' : (shot.role ?? manifest.role);
      if (!sessions.has(role)) {
        const context = await newContext(browser);
        if (!shot.anon) {
          const account = accounts.roles[role];
          if (!account) throw new Error(`в accounts.json нет роли ${role}`);
          // Вход - через форму на отдельной вкладке; дальше она закрывается,
          // сеанс остаётся в окружении и продлевается сам.
          const login = await signIn(
            context,
            { username: account.username, password: accounts.password },
            { baseUrl, apiBase, clockAt, keepConsent: account.keepConsent === true },
          );
          await login.close();
        }
        sessions.set(role, context);
      }

      /*
       * Каждому кадру - своя вкладка. Общая вкладка тянет за собой состояние
       * предыдущего кадра: раскрытая панель, наведённый указатель, положение
       * прокрутки. Один кадр из-за этого уже перестал сниматься в пачке, хотя
       * в одиночку снимался. Открыть вкладку дешевле, чем ловить такое.
       *
       * Сбой одного кадра не роняет пачку: разбирать замечания по одному,
       * перезапуская прогон после каждого, - самый долгий путь к чистому
       * прогону.
       */
      const page = await sessions.get(role).newPage();
      let shotWarnings;
      try {
        if (clockAt) await page.clock.setFixedTime(clockAt);
        if (shot.clearDraft) await clearDraft(page);
        /*
         * Высокое окно - для форм, которые в обычное не помещаются целиком.
         * Снимок берётся с видимой области, поэтому у длинной формы низ просто
         * не попадал в кадр, а выноскам не хватало места. Ширина не меняется:
         * от неё зависит раскладка, и узкое окно перевело бы её в мобильную.
         */
        // Ширина окна тоже задаётся кадром: ряд фильтров, не поместившийся в
        // стандартные 1300, переносит последнюю кнопку на вторую строку, и
        // кадр показывает раскладку, которой на мониторе поста не бывает.
        if (shot.viewport?.height || shot.viewport?.width) {
          await page.setViewportSize({
            width: shot.viewport.width ?? VIEWPORT.width,
            height: shot.viewport.height ?? VIEWPORT.height,
          });
        }
        await page.goto(`${baseUrl}${shot.goto ?? '/'}`);
        /*
         * Уменьшение страницы для кадров, где окно приложения выше экрана и
         * прокручивается внутри себя: снимок такого окна показывает половину
         * содержимого, а читателю обещана карточка целиком.
         */
        /*
         * Правка стилей для кадра: окно приложения с внутренней прокруткой
         * показывает половину содержимого, и снимок обещает читателю не то, что
         * в нём есть. Раскрываем такое окно на всю высоту содержимого.
         */
        if (shot.style) await page.addStyleTag({ content: shot.style });
        if (shot.zoom) {
          await page.evaluate((value) => {
            document.documentElement.style.zoom = String(value);
          }, shot.zoom);
        }
        shotWarnings = await shoot(page, shot, outDir);
      } catch (error) {
        shotWarnings = [`${shot.id}: не снялся - ${error.message.split('\n')[0]}`];
      } finally {
        await page.close();
      }
      warnings.push(...shotWarnings);
      console.log(`${shotWarnings.length ? '!' : '+'} ${shot.id}`);
    }
  } finally {
    for (const context of sessions.values()) await context.close();
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
