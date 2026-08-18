/**
 * Замки реестра значков интерфейса.
 *
 * Реестр вытесняет растровые иконки, и вытеснение идёт постепенно: часть экранов
 * ещё на PNG. Поэтому здесь два рода проверок - контракт самого компонента и
 * покрытие: каждая живая растровая иконка обязана иметь глиф в реестре, иначе
 * перевод следующего экрана упирается в недостающую картинку.
 */
import fs from 'node:fs';
import path from 'node:path';
import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import AppIcon from '../AppIcon.vue';
import { appIcons, appIconNames } from '../appIcons.js';

const SRC = path.resolve(__dirname, '../../..');

/** Растровые имена, у которых цвет уехал в CSS: в реестре они без суффикса. */
const RASTER_ALIASES = {
  'key-blue': 'key',
  'email-blue': 'email',
  'phone-blue': 'phone',
};

function sourceFiles(dir, acc = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) sourceFiles(full, acc);
    else if (/\.(vue|js)$/.test(e.name)) acc.push(full);
  }
  return acc;
}

describe('AppIcon', () => {
  it('рендерит svg обводкой currentColor и размером из пропа', () => {
    const svg = mount(AppIcon, { props: { name: 'search', size: 32 } }).find('svg');
    expect(svg.exists()).toBe(true);
    expect(svg.attributes('stroke')).toBe('currentColor');
    expect(svg.attributes('fill')).toBe('none');
    expect(svg.attributes('width')).toBe('32');
    expect(svg.attributes('height')).toBe('32');
    expect(svg.attributes('viewBox')).toBe('0 0 24 24');
  });

  it('размер по умолчанию - 20', () => {
    expect(mount(AppIcon, { props: { name: 'edit' } }).find('svg').attributes('width')).toBe('20');
  });

  it('aria-hidden - значок декоративный, смысл несёт подпись рядом', () => {
    expect(mount(AppIcon, { props: { name: 'trashcan' } }).find('svg').attributes('aria-hidden'))
      .toBe('true');
  });

  it('неизвестное имя не роняет рендер (пустой svg)', () => {
    const svg = mount(AppIcon, { props: { name: 'does-not-exist' } }).find('svg');
    expect(svg.exists()).toBe(true);
    expect(svg.html()).not.toContain('<path');
  });

  it('все глифы реестра рендерят непустую разметку', () => {
    for (const name of appIconNames) {
      const inner = mount(AppIcon, { props: { name } }).find('svg').element.innerHTML;
      expect(inner.length, `глиф "${name}" пустой`).toBeGreaterThan(0);
    }
  });

  it('глифы не задают собственный цвет - только currentColor', () => {
    // Литеральный цвет внутри разметки убил бы главное свойство реестра: значок
    // перестал бы наследовать цвет текста, и темизация вернулась бы к фильтрам.
    const hardcoded = appIconNames.filter((n) => /(fill|stroke)="(?!currentColor|none)/.test(appIcons[n]));
    expect(hardcoded, 'цвет глифа задаёт CSS через color, а не разметка').toEqual([]);
  });
});

describe('покрытие растровых иконок', () => {
  const used = new Set();
  for (const file of sourceFiles(SRC)) {
    for (const m of fs.readFileSync(file, 'utf8').matchAll(/@\/assets\/icons\/([\w-]+)\.png/g)) {
      used.add(m[1]);
    }
  }

  it('у каждой живой растровой иконки есть глиф в реестре', () => {
    const missing = [...used]
      .map((n) => RASTER_ALIASES[n] ?? n)
      .filter((n) => !appIcons[n]);
    expect([...new Set(missing)], 'нарисуйте глиф в appIcons.js перед переводом экрана').toEqual([]);
  });

  it('экран входа переведён на реестр целиком', () => {
    // Экран входа - доказательство схемы: если PNG вернётся сюда, значит замена
    // где-то откатилась (иконка полей, ключ на кнопке или контакты поддержки).
    ['LoginComponent.vue', 'PasswordRecoveryModal.vue'].forEach((name) => {
      const txt = fs.readFileSync(path.join(SRC, 'components', name), 'utf8');
      expect(txt, `${name} снова тянет растровую иконку`).not.toMatch(/@\/assets\/icons\//);
    });
  });
});
