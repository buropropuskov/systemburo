/**
 * Замок на порядок загрузки темы (#1415).
 *
 * Тему профиля обязан тянуть main.js ДО `app.mount`: bootstrap-скрипт index.html
 * знает только localStorage, а на новом устройстве его нет - и приложение
 * успевало отрисоваться в светлой теме, а через секунду-две перекрашивалось.
 * Проверяем текстом по исходникам: юнит-тестом порядок в main.js (там top-level
 * await и сайд-эффекты) не воспроизвести.
 */
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const SRC = path.resolve(__dirname, '..');
const mainJs = fs.readFileSync(path.join(SRC, 'main.js'), 'utf8');
const appVue = fs.readFileSync(path.join(SRC, 'App.vue'), 'utf8');

describe('загрузка темы при старте', () => {
  it('main.js запрашивает тему профиля до mount', () => {
    const sync = mainJs.indexOf('syncFromServer');
    const mount = mainJs.indexOf('app.mount(');
    expect(sync, 'main.js не запрашивает тему профиля').toBeGreaterThan(-1);
    expect(mount).toBeGreaterThan(-1);
    expect(sync, 'тема должна запрашиваться ДО mount, иначе интерфейс мелькнёт чужой темой')
      .toBeLessThan(mount);
  });

  it('запрос темы дожидается вместе с остальной загрузкой', () => {
    // Пачка с maintenance: параллельно, поэтому ничего не удлиняет, но mount
    // ждёт оба ответа.
    const batch = mainJs.match(/const boot = \[[\s\S]{0,400}?await Promise\.all\(boot\)/);
    expect(batch, 'тема должна уходить в awaited-пачку загрузки').not.toBeNull();
    expect(batch[0]).toContain('syncFromServer');
  });

  it('App.created тему больше не запрашивает - иначе будет второй запрос', () => {
    const created = appVue.slice(appVue.indexOf('created()'), appVue.indexOf('beforeUnmount()'));
    expect(created.length).toBeGreaterThan(0);
    expect(created).not.toContain('syncFromServer');
  });
});
