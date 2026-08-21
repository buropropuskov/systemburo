import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { TOURS, buildTourSteps, allTourSteps } from '../tours';

/**
 * Замок на якоря туров: каждый `element` шага должен реально существовать в
 * исходниках. Исчезнувший селектор тур не роняет - шаг молча вырождается в поповер
 * по центру экрана без подсветки (OnboardingTour деградирует element в null по
 * таймауту), и заметить это можно только глазами. Тест ловит переименование или
 * удаление testid раньше, чем пользователь.
 *
 * Проверка по исходникам, а не по живому DOM: цели живут на семи разных страницах,
 * поднимать их все ради одного утверждения дороже, чем грепнуть.
 */

const SRC_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..');
const SCANNED_EXTENSIONS = new Set(['.vue', '.js', '.ts']);
// Спеки из скана исключены: иначе testid, оставшийся только в тесте, считался бы
// живым якорем - и замок пропустил бы цель, удалённую из интерфейса.
const SKIPPED_DIRS = new Set(['__tests__']);

/**
 * Все значения data-testid, ПРОСТАВЛЕННЫЕ в разметке. Вхождения вида
 * `[data-testid="x"]` (CSS-селектор) отсекаются скобкой перед именем: иначе в
 * набор попадали бы селекторы из самих конфигураций туров, и замок стерёг бы
 * собственный текст - любой переименованный якорь «находился» бы в них же.
 *
 * Динамические (`:data-testid="\`admin-link-${x}\`"`) сюда не попадают - шаги на
 * такие цели не указывают, а появятся - тест честно упадёт и заставит их учесть.
 *
 * @returns {Set<string>}
 */
function collectTestIds() {
  const found = new Set();
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        if (!SKIPPED_DIRS.has(entry.name)) walk(full);
        continue;
      }
      if (!SCANNED_EXTENSIONS.has(path.extname(entry.name))) continue;
      if (entry.name.includes('.spec.')) continue;
      const source = fs.readFileSync(full, 'utf8');
      for (const m of source.matchAll(/(^|[^[])data-testid\s*=\s*["']([^"'`${}]+)["']/gm)) {
        found.add(m[2]);
      }
      // Окно BaseModal телепортируется в body, и снаружи ему нечем проставить
      // data-testid - имя передаётся пропом content-testid, а модалка вешает его
      // на себя. Для замка это такой же живой якорь.
      for (const m of source.matchAll(/content-testid\s*=\s*["']([^"'`${}]+)["']/g)) {
        found.add(m[1]);
      }
      // testid из реестра-конфига, а не из разметки: ApplicationTags берёт имя у
      // описания тега (applicationTags.js) и вешает через :data-testid. Якорь такой
      // же живой - литерал лежит в исходнике рядом с признаком, по которому тег
      // появляется, и исчезнет вместе с ним.
      for (const m of source.matchAll(/testid:\s*\(\s*\w*\s*\)\s*=>\s*['"]([^'"`${}]+)['"]/g)) {
        found.add(m[1]);
      }
      // Условная привязка вида `:data-testid="выбран ? 'ob-blank-selected' : null"`:
      // имя тут - обычный литерал, и якорь на него настоящий. Шаблонные строки
      // по-прежнему не берём - там имя собирается в рантайме.
      for (const m of source.matchAll(/:data-testid\s*=\s*"([^"]*)"/g)) {
        for (const lit of m[1].matchAll(/'([^'`${}]+)'/g)) found.add(lit[1]);
      }
    }
  };
  walk(SRC_DIR);
  return found;
}

/**
 * @param {string} selector
 * @returns {string|null} testid из селектора вида [data-testid="x"]
 */
function testIdFromSelector(selector) {
  return selector.match(/^\[data-testid="([^"]+)"\]$/)?.[1] ?? null;
}

const testIds = collectTestIds();

/**
 * Тот же замок, что применяется к шагам туров - вынесен, чтобы проверить его и на
 * заведомо выдуманном якоре: без этого зелёный тест не отличить от нерабочего.
 *
 * @param {string} selector
 * @returns {boolean}
 */
function anchorExists(selector) {
  const id = testIdFromSelector(selector);
  return id !== null && testIds.has(id);
}

const steps = allTourSteps().filter((s) => s.element);

describe('якоря шагов туров существуют в исходниках', () => {
  it('исходники вообще просканированы (иначе тест был бы зелёным впустую)', () => {
    expect(testIds.size).toBeGreaterThan(100);
    expect(steps.length).toBeGreaterThan(10);
  });

  it('каждый element - селектор по data-testid (иначе замок его не стережёт)', () => {
    steps.forEach((s) => {
      expect(testIdFromSelector(s.element), `шаг ${s.id}: ${s.element}`).not.toBe(null);
    });
  });

  steps.forEach((step) => {
    it(`${step.id} -> ${step.element}`, () => {
      expect(anchorExists(step.element)).toBe(true);
    });
  });

  it('тот же замок отвергает выдуманный якорь и якорь, живущий только в спеке', () => {
    expect(anchorExists('[data-testid="ob-anchor-that-never-existed"]')).toBe(false);
    // testid из этой строки в скан не попадает - спеки исключены из обхода.
    expect(anchorExists('[data-testid="ob-anchor-only-in-spec"]')).toBe(false);
  });

  it('шаги без element (центр-модалы) замок не трогает', () => {
    const centered = TOURS.flatMap((t) => buildTourSteps(t, { factTableRoute: '/table/x' }))
      .filter((s) => !s.element);
    expect(centered.length).toBeGreaterThan(0);
    centered.forEach((s) => expect(s.element ?? null).toBe(null));
  });
});
