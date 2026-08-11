import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync } from 'node:fs';
import { join, resolve, relative } from 'node:path';

/**
 * Тумблер, собранный из скрытого чекбокса и дорожки, обязан оставаться в порядке
 * обхода (#1879).
 *
 * У «Обязательного согласования» инпут прятался через `display: none`, то есть
 * выпадал из tab-order целиком: переключить тумблер с клавиатуры было нельзя, и
 * кольца фокуса у него не было. Разметка при этом исправна, v-model работает,
 * компонент рендерится - ни jsdom-спека, ни линтер такого не видят, дефект живёт
 * исключительно в CSS. Соседние тумблеры проекта (`ww-switch`, `round-switch`,
 * общие SwitchToggle/ToggleSwitch) прячут инпут смещением и прозрачностью и
 * этой дырой не страдают.
 *
 * Скан идёт по всем SFC: тумблеры на проекте размножены копипастой, и новый
 * попадает под замок сам. Опознаём их по правилу `... :checked + .дорожка` -
 * тому же признаку, что и в toggleStatesDistinct.
 */

const srcDir = resolve(__dirname, '../..');

/** Способы убрать элемент из дерева доступности вместе с порядком обхода. */
const REMOVED_FROM_TAB_ORDER = /(?:^|[;{\s])(?:display\s*:\s*none|visibility\s*:\s*hidden)\s*(?:;|$)/;

const stripComments = (css) => css.replace(/\/\*[\s\S]*?\*\//g, '');

/** Все правила из <style>-блоков файла: селектор + объявления. */
function cssRules(source) {
  const rules = [];
  for (const style of source.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)) {
    for (const rule of stripComments(style[1]).matchAll(/([^{}@;]+)\{([^{}]*)\}/g)) {
      rules.push({ selector: rule[1].trim().replace(/\s+/g, ' '), body: rule[2] });
    }
  }
  return rules;
}

/** Правило прячет инпут, а не что-то ещё. */
const hidesInput = (selector) => /(?:^|[\s>+~])(?:input|[\w.[\]"'=-]*input)[\w[\]"'=-]*$/.test(selector)
  || /input(?:\[[^\]]*\])?$/.test(selector);

const vueFiles = () => readdirSync(srcDir, { recursive: true, withFileTypes: true })
  .filter((e) => e.isFile() && e.name.endsWith('.vue'))
  .map((e) => join(e.parentPath ?? e.path, e.name));

/** Файлы, где живёт CSS-тумблер: есть правило вида `...:checked + .дорожка`. */
function toggleFiles() {
  const found = [];
  for (const file of vueFiles()) {
    const rules = cssRules(readFileSync(file, 'utf8'));
    if (rules.some((r) => /:checked\s*\+/.test(r.selector))) found.push({ file, rules });
  }
  return found;
}

describe('CSS-тумблеры остаются достижимыми с клавиатуры', () => {
  const files = toggleFiles();

  it('находит тумблеры проекта', () => {
    expect(files.length, 'признак `:checked +` перестал опознавать тумблеры').toBeGreaterThanOrEqual(6);
  });

  it('ни один не прячет свой инпут display:none / visibility:hidden', () => {
    const broken = [];
    for (const { file, rules } of files) {
      for (const rule of rules) {
        if (!hidesInput(rule.selector)) continue;
        if (!REMOVED_FROM_TAB_ORDER.test(rule.body)) continue;
        broken.push(`${relative(srcDir, file)}: ${rule.selector} - инпут выпал из порядка обхода`);
      }
    }
    expect(broken, 'тумблер нельзя будет переключить с клавиатуры').toEqual([]);
  });

  it('у «Обязательного согласования» кольцо фокуса рисует дорожка', () => {
    const rules = cssRules(readFileSync(join(srcDir, 'components/ResponsibleUsersSection.vue'), 'utf8'));
    const ring = rules.find((r) => /:focus-visible\s*\+\s*\.slider$/.test(r.selector));
    expect(ring, 'скрытый инпут своего фокуса не показывает - кольцо обязано быть на дорожке').toBeTruthy();
    expect(ring.body).toMatch(/outline\s*:/);
  });
});
