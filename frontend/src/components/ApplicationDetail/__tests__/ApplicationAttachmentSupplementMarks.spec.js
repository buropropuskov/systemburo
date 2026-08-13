import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import ApplicationAttachmentDetail from '../ApplicationAttachmentDetail.vue';

const ROW = '[data-testid="attachment-element-row"]';
const BADGE = '[data-testid="attachment-supplement-badge"]';

const SFC = readFileSync(resolve(__dirname, '../ApplicationAttachmentDetail.vue'), 'utf8');

/** Тело правила для селектора, без учёта переносов. */
function rule(src, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const found = src.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`));
  return found ? found[1].replace(/\s+/g, ' ').trim() : null;
}

// Признаки приезжают плоскими полями строки состава (SupplementMark встроен в DTO машины,
// сотрудника и ТМЦ) - фикстуру строим по json-тегам, а не по своим догадкам.
function mark({ id = 3, number = 2, status = 'pending', pending = true } = {}) {
  return {
    supplement_id: id,
    supplement_number: number,
    supplement_status: status,
    is_pending: pending,
  };
}

function car(over = {}) {
  return { id: 1, car_number: 'А123ВС', car_brand: 'Toyota', unload_places: [], ...over };
}

function employee(over = {}) {
  return {
    id: 1,
    last_name: 'Иванов',
    first_name: 'Иван',
    middle_name: 'Иванович',
    position: 'Водитель',
    target_tables: [],
    ...over,
  };
}

function item(over = {}) {
  return { id: 1, name: 'Ноутбук', count: 2, ...over };
}

function mountAttachment(type, rows) {
  const key = { cars: 'cars', people: 'employees', items: 'items' }[type];
  return mount(ApplicationAttachmentDetail, {
    props: {
      attachment: { id: 1, attachment_type: type, attachment_display_name: 'Вложение' },
      [key]: rows,
    },
  });
}

describe('ApplicationAttachmentDetail — метки дополнения (#1685)', () => {
  it('строка исходной подачи (supplement_id: null) не получает ни бейджа, ни класса', () => {
    const wrapper = mountAttachment('cars', [car({
      supplement_id: null,
      supplement_number: null,
      supplement_status: null,
      is_pending: false,
    })]);

    expect(wrapper.find(BADGE).exists()).toBe(false);
    expect(wrapper.find(ROW).classes().join(' ')).not.toContain('supplement');
  });

  it('поля дополнения нет в ответе вовсе: строка остаётся обычной', () => {
    const wrapper = mountAttachment('cars', [car()]);

    expect(wrapper.find(BADGE).exists()).toBe(false);
    expect(wrapper.find(ROW).classes().join(' ')).not.toContain('supplement');
  });

  it('раунд ждёт согласующих: «На согласовании» и предупреждающий вид строки', () => {
    const wrapper = mountAttachment('cars', [car(mark({ status: 'pending' }))]);

    expect(wrapper.find(BADGE).text()).toBe('На согласовании');
    expect(wrapper.find(BADGE).classes()).toContain('badge--warning');
    expect(wrapper.find(BADGE).attributes('data-hint')).toContain('Дополнение №2');
    expect(wrapper.find(ROW).classes()).toContain('el-row--supplement-pending');
  });

  it('раунд согласован: «Ждёт принятия» и информационный вид строки', () => {
    const wrapper = mountAttachment('cars', [car(mark({ status: 'approved' }))]);

    expect(wrapper.find(BADGE).text()).toBe('Ждёт принятия');
    expect(wrapper.find(BADGE).classes()).toContain('badge--info');
    expect(wrapper.find(ROW).classes()).toContain('el-row--supplement-approved');
  });

  // На мобилке карточка уводит метку на свою строку: класс ключевой ячейки - цель
  // правила, обёртка .supplement-line - то, что забирает строку целиком. Без обёртки
  // место метки снова определяла бы длина значения, и она жала бы гос. номер.
  it('метка стоит в ключевой ячейке, в обёртке под отдельную строку карточки', () => {
    const wrapper = mountAttachment('cars', [car(mark({ status: 'pending' }))]);

    const keyCell = wrapper.find('.el-row .el-cell--key');
    expect(keyCell.exists()).toBe(true);
    expect(keyCell.find('.supplement-line').exists()).toBe(true);
    expect(keyCell.find('.supplement-line').find(BADGE).exists()).toBe(true);
  });

  it('раунд принят: постоянная метка происхождения без подсветки строки', () => {
    const wrapper = mountAttachment('cars', [car(mark({ status: 'accepted', pending: false }))]);

    expect(wrapper.find(BADGE).text()).toBe('Доп. №2');
    expect(wrapper.find(ROW).classes().join(' ')).not.toContain('el-row--supplement-');
  });

  it.each(['rejected', 'refused', 'cancelled'])(
    'раунд закрыт отказом (%s): «Отклонено» и приглушённая строка',
    (status) => {
      const wrapper = mountAttachment('cars', [car(mark({ status }))]);

      expect(wrapper.find(BADGE).text()).toBe('Отклонено');
      expect(wrapper.find(ROW).classes()).toContain('el-row--supplement-closed');
    }
  );

  it('четыре состояния дают четыре разных класса строки', () => {
    const classFor = (status, pending = true) => mountAttachment('cars', [car(mark({ status, pending }))])
      .find(ROW).classes()
      .filter(c => c.startsWith('el-row--supplement'));

    const classes = [
      classFor('pending'),
      classFor('approved'),
      classFor('accepted', false),
      classFor('rejected'),
    ].map(list => list.join('|'));

    expect(new Set(classes).size).toBe(4);
  });

  it('раунд влит в основной круг (merged): только метка происхождения', () => {
    const wrapper = mountAttachment('cars', [car(mark({ status: 'merged' }))]);

    expect(wrapper.find(BADGE).text()).toBe('Доп. №2');
    expect(wrapper.find(ROW).classes().join(' ')).not.toContain('el-row--supplement-');
  });

  it('без номера раунда метка не печатает «№undefined»', () => {
    const wrapper = mountAttachment('cars', [car(mark({ number: null, status: 'accepted', pending: false }))]);

    expect(wrapper.find(BADGE).text()).toBe('Доп.');
  });

  it('сотрудники получают ту же метку', () => {
    const wrapper = mountAttachment('people', [employee(mark({ status: 'pending' }))]);

    expect(wrapper.find(BADGE).text()).toBe('На согласовании');
    expect(wrapper.find(ROW).classes()).toContain('el-row--supplement-pending');
  });

  // У ТМЦ нет колонки действий (там количество), поэтому метка живёт в ключевой колонке -
  // иначе третий тип вложения остался бы без признака.
  it('ТМЦ получают ту же метку', () => {
    const wrapper = mountAttachment('items', [item(mark({ status: 'approved' }))]);

    expect(wrapper.find(BADGE).text()).toBe('Ждёт принятия');
    expect(wrapper.find(ROW).classes()).toContain('el-row--supplement-approved');
  });

  // Возможный обход ЧС критичнее «строка новая»: подсветку строки в этом случае
  // забирает ЧС (правило дополнения гасится через :not), метка дополнения остаётся.
  it('строка дополнения с флагом ЧС сохраняет и флаг, и метку', () => {
    const wrapper = mountAttachment('cars', [car({
      ...mark({ status: 'pending' }),
      blacklist_similar: { flag_id: 10, matched_value: 'А124ВС', overridden: false },
    })]);

    const classes = wrapper.find(ROW).classes();
    expect(classes).toContain('el-row--flagged');
    expect(classes).toContain('el-row--supplement-pending');
    expect(wrapper.find(BADGE).text()).toBe('На согласовании');
  });

  it('метку получает только помеченная строка списка', () => {
    const wrapper = mountAttachment('cars', [
      car({ id: 1, car_number: 'А111АА' }),
      car({ id: 2, car_number: 'В222ВВ', ...mark({ status: 'pending' }) }),
    ]);

    const rows = wrapper.findAll(ROW);
    expect(rows).toHaveLength(2);
    expect(rows[0].find(BADGE).exists()).toBe(false);
    expect(rows[1].find(BADGE).text()).toBe('На согласовании');
  });
});

// jsdom не считает ни каскад, ни медиазапросы, ни перенос строк во флексе, поэтому
// правила раскладки стережём чтением SFC. Сам эффект - замером в браузере на 320/390.
describe('ApplicationAttachmentDetail — метка не наезжает на соседние поля', () => {
  it('в карточке метка занимает строку целиком и не прижимается вправо', () => {
    expect(rule(SFC, '.el-row .el-cell--key .supplement-line')).toMatch(/flex:\s*0 0 100%/);
    expect(rule(SFC, '.el-row .el-cell--key .supplement-badge')).not.toMatch(/margin-left:\s*auto/);
  });

  // Держим смысл, а не число: важно, что вертикальный отступ есть и что высоту поля
  // задаёт содержимое. Фиксированная высота тут была причиной наложений - `min-height`
  // на флекс-элементе отменяет автоматический минимум, и содержимое печаталось поверх
  // разделителя. Поэтому замок заодно запрещает её вернуть.
  it('поле карточки держит вертикальный отступ - перенос не ложится на пунктир', () => {
    const decls = rule(SFC, '.el-table .el-row .el-cell');
    const padding = decls.match(/padding:\s*(\d+)px 0/);
    expect(padding, `вертикальный отступ поля не задан: ${decls}`).not.toBeNull();
    expect(Number(padding[1])).toBeGreaterThanOrEqual(3);
    expect(decls, 'фиксированная высота поля отменяет автоминимум флекса').not.toMatch(/min-height:\s*\d/);
  });

  // Тап по строке оставляет :hover залипшим, и пузырёк подсказки повис бы поперёк
  // соседних карточек - на тач-экране показывать его нечему и незачем.
  it('подсказки по data-hint показываются только там, где есть курсор', () => {
    expect(SFC).toMatch(
      /@media \(hover: hover\) \{[^@]*\.supplement-badge\[data-hint\]:hover::after/
    );
    expect(SFC).not.toMatch(/^\.supplement-badge\[data-hint\]:hover::after/m);
  });

  // На мобильной карточке ключевая колонка машины уже своего бейджа (десктопный минимум
  // расширен, но карточка узкого экрана снова может оказаться теснее), а Badge.vue держит
  // текст в один ряд (white-space: nowrap) - пилюля "На согласовании" зажималась по
  // max-width: 100%, и буквы вылезали за фон пилюли без подложки под ними (владелец: не
  // помещается, обрезается). Замок держит перенос текста внутри бейджа - фон растёт
  // вместе со строками вместо того чтобы ронять их за свою границу.
  it('бейдж переносит текст, когда ключевая колонка уже его содержимого', () => {
    expect(rule(SFC, '.el-cell--key .supplement-line .supplement-badge'))
      .toMatch(/white-space:\s*normal/);
  });
});
