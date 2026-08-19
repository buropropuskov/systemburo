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

  it('переведённые значки не возвращаются растром', () => {
    // Срез считается сделанным, когда PNG удалён и ссылок на него не осталось:
    // без такой пары замена молча откатывается обратной правкой одного файла.
    const gone = [
      'sort', 'arrow', 'export', 'trashcan', 'edit', 'delete',
      'search', 'calendar', 'car', 'clipboard', 'download', 'employees', 'instruction',
      'notifications', 'random', 'recent-changes', 'refresh', 'save', 'stats', 'user', 'xlsx',
      'login', 'password', 'key-blue', 'email-blue', 'phone-blue',
    ];
    const back = gone.filter((n) => fs.existsSync(path.join(SRC, 'assets/icons', `${n}.png`)));
    expect(back, 'растровый файл вернулся в assets').toEqual([]);
    expect(gone.filter((n) => used.has(n)), 'разметка снова тянет растровый значок').toEqual([]);
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

describe('значок сортировки', () => {
  // Потребители показывают обратный порядок поворотом значка на 180 градусов
  // (.sort-icon.desc, .sort-icon.sort-asc, .hf-sort-icon--asc, .trash-table__sort--desc).
  // Связка неявная: симметричный глиф переживает такой поворот без изменений, и
  // указатель направления пропадает молча - разметка и стили при этом верны.
  const segments = [...appIcons.sort.matchAll(
    /<line x1="([\d.]+)" y1="([\d.]+)" x2="([\d.]+)" y2="([\d.]+)"/g,
  )].map((m) => m.slice(1).map(Number));

  // Округление обязательно: 24 - 3.4 даёт 20.599999999999998, и сравнение по
  // сырым числам не совпало бы никогда - проверка стала бы вечнозелёной.
  const round = (v) => Number(v.toFixed(3));
  const key = (s) => [`${s[0]},${s[1]}`, `${s[2]},${s[3]}`].sort().join(' ');
  const rotated = (s) => s.map((v) => round(24 - v));

  it('нарисован отрезками - поворот проверяется по их координатам', () => {
    expect(segments.length, 'глиф перерисован другими фигурами - обнови проверку ниже')
      .toBeGreaterThan(0);
  });

  it('не совпадает сам с собой после поворота на 180 градусов', () => {
    const before = new Set(segments.map(key));
    const after = segments.map(rotated).map(key);
    expect(after.some((s) => !before.has(s)), 'повёрнутый значок неотличим от исходного')
      .toBe(true);
  });
});

describe('значок раскрытия списка', () => {
  // Потребители держат «свёрнуто» базовым состоянием и доворачивают значок на 90
  // градусов (.select-arrow.arrow-open, .button__arrow--open, .rotated). Шеврон,
  // перерисованный вниз, после такого доворота показывал бы открытый список
  // закрытым, а разметка и стили при этом остались бы верными - направление
  // ловится только здесь.
  // Путь вида "m9.3 5.4 6.6 6.6-6.6 6.6": стартовая точка и относительные смещения.
  const nums = appIcons.arrow.match(/d="m([^"]+)"/)[1].split(/[\s]+|(?=-)/).filter(Boolean).map(Number);
  const points = [[nums[0], nums[1]]];
  for (let i = 2; i < nums.length; i += 2) {
    const [px, py] = points[points.length - 1];
    points.push([px + nums[i], py + nums[i + 1]]);
  }

  it('нарисован ломаной из трёх точек - иначе проверка направления ниже слепа', () => {
    expect(points, 'глиф перерисован другими фигурами - обнови разбор').toHaveLength(3);
  });

  it('шеврон смотрит вправо: вершина правее обоих концов', () => {
    const [start, tip, end] = points;
    expect(tip[0]).toBeGreaterThan(start[0]);
    expect(tip[0]).toBeGreaterThan(end[0]);
  });

  it('обводка толще общей - на 8-10 px значок иначе вырождается в волосок', () => {
    const width = Number(appIcons.arrow.match(/stroke-width="([\d.]+)"/)[1]);
    expect(width).toBeGreaterThan(1.7);
  });
});

describe('значок ключа на кнопке входа', () => {
  it('нарисован заливкой - обводка рядом с подписью весом 800 читается волоском', () => {
    // Ключ стоит вплотную к слову «Войти» (20px, font-weight 800). Прежний растр был
    // сплошной фигурой, и линейный глиф на его месте владелец забраковал как «жидкий».
    // Проверка держит именно массу: перерисовка обводкой вернёт ту же претензию молча.
    expect(appIcons.key, 'ключ рисуется заливкой, а не обводкой').toMatch(/fill="currentColor"/);
    expect(appIcons.key, 'обводка у залитого глифа выключена').toMatch(/stroke="none"/);
  });

  it('скважина остаётся дыркой, а не закрашивается вместе с головкой', () => {
    // Без evenodd внутренний контур сольётся с головкой, и ключ станет каплей.
    expect(appIcons.key).toMatch(/fill-rule="evenodd"/);
  });
});

describe('значок выгрузки', () => {
  it('стрелка выходит вверх, а не входит вниз - иначе это download', () => {
    // Знак «внешняя ссылка» (лист со стрелкой в угол) на кнопке «Экспорт» читается
    // как «откроется в новом окне», поэтому здесь лоток и стрелка из него.
    const [, x1, y1, , y2] = appIcons.export
      .match(/<line x1="([\d.]+)" y1="([\d.]+)" x2="([\d.]+)" y2="([\d.]+)"/)
      .map(Number);
    const tip = appIcons.export.match(/<polyline points="[\d.]+ [\d.]+ ([\d.]+) ([\d.]+)/).slice(1).map(Number);
    expect(y1, 'стержень стрелки начинается сверху').toBeLessThan(y2);
    expect(tip[1], 'остриё выше стержня').toBeLessThanOrEqual(y1);
    expect(tip[0], 'остриё по центру стержня').toBeCloseTo(x1, 1);
  });
});

describe('значок настроек', () => {
  // Значок настроек в окнах уведомлений поворачивается на наведении. Шестерёнка о
  // восьми зубцах повторяет себя каждые 45 градусов, поэтому поворот РОВНО на шаг
  // (так было у прежнего значка-ползунков) визуально не отличим от покоя: стили и
  // разметка при этом верны, тест на наличие transform - зелёный.
  const SPIN_STEP = 360 / 8;

  it('оба потребителя берут глиф из реестра, а не рисуют свой svg', () => {
    ['UserNotifications.vue', 'UserNotificationsInline.vue'].forEach((name) => {
      const txt = fs.readFileSync(path.join(SRC, 'components', name), 'utf8');
      const template = txt.split('<script>')[0];
      expect(template, `${name} снова рисует значок настроек вручную`).toMatch(/name="settings"/);
      expect(template.includes('stroke-width="1.8"'), `${name} держит свой svg настроек`).toBe(false);
    });
  });

  it('поворот на наведении меньше углового шага зубцов', () => {
    // Смотрим именно правило наведения на кнопку настроек: в тех же файлах живёт
    // спиннер с rotate(360deg), и он к зубцам отношения не имеет.
    ['UserNotifications.vue', 'UserNotificationsInline.vue'].forEach((name) => {
      const txt = fs.readFileSync(path.join(SRC, 'components', name), 'utf8');
      const rule = txt.match(/\.notifications__settings[\w-]*:hover\s*\{([^{}]*)\}/);
      expect(rule, `${name}: правило наведения на значок настроек пропало`).not.toBeNull();
      const angle = rule[1].match(/rotate\((-?[\d.]+)deg\)/);
      expect(angle, `${name}: поворот значка пропал`).not.toBeNull();
      const value = Math.abs(parseFloat(angle[1]));
      expect(value % SPIN_STEP, `${name}: поворот на ${value} кратен шагу зубцов и не виден`).not.toBe(0);
    });
  });

  it('шестерёнка держит восемь зубцов', () => {
    // Число зубцов - половина контракта с поворотом: сменив его, надо пересчитать угол.
    const outerArcs = [...appIcons.settings.matchAll(/A9\.3 9\.3 /g)];
    expect(outerArcs).toHaveLength(8);
  });
});
