import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import ApplicationAttachmentDetail from '../ApplicationAttachmentDetail.vue';

const ROW = '[data-testid="attachment-element-row"]';
const BADGE = '[data-testid="attachment-supplement-badge"]';

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

  it('раунд ждёт согласующих: «Новое, на согласовании» и предупреждающий вид строки', () => {
    const wrapper = mountAttachment('cars', [car(mark({ status: 'pending' }))]);

    expect(wrapper.find(BADGE).text()).toBe('Новое, на согласовании');
    expect(wrapper.find(BADGE).classes()).toContain('badge--warning');
    expect(wrapper.find(BADGE).attributes('data-hint')).toContain('Дополнение №2');
    expect(wrapper.find(ROW).classes()).toContain('el-row--supplement-pending');
  });

  it('раунд согласован: «Согласовано, ждёт принятия» и информационный вид строки', () => {
    const wrapper = mountAttachment('cars', [car(mark({ status: 'approved' }))]);

    expect(wrapper.find(BADGE).text()).toBe('Согласовано, ждёт принятия');
    expect(wrapper.find(BADGE).classes()).toContain('badge--info');
    expect(wrapper.find(ROW).classes()).toContain('el-row--supplement-approved');
  });

  // На мобилке карточка уводит метку на свою строку правилом по .el-cell--key: без
  // этого класса правило не имело бы цели, а метка жала бы гос. номер.
  it('метка стоит в ключевой ячейке, помеченной классом для карточной раскладки', () => {
    const wrapper = mountAttachment('cars', [car(mark({ status: 'pending' }))]);

    const keyCell = wrapper.find('.el-row .el-cell--key');
    expect(keyCell.exists()).toBe(true);
    expect(keyCell.find(BADGE).exists()).toBe(true);
  });

  it('раунд принят: постоянная метка происхождения без подсветки строки', () => {
    const wrapper = mountAttachment('cars', [car(mark({ status: 'accepted', pending: false }))]);

    expect(wrapper.find(BADGE).text()).toBe('Доп. №2');
    expect(wrapper.find(ROW).classes().join(' ')).not.toContain('el-row--supplement-');
  });

  it.each(['rejected', 'refused', 'cancelled'])(
    'раунд закрыт отказом (%s): «Дополнение отклонено» и приглушённая строка',
    (status) => {
      const wrapper = mountAttachment('cars', [car(mark({ status }))]);

      expect(wrapper.find(BADGE).text()).toBe('Дополнение отклонено');
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

    expect(wrapper.find(BADGE).text()).toBe('Новое, на согласовании');
    expect(wrapper.find(ROW).classes()).toContain('el-row--supplement-pending');
  });

  // У ТМЦ нет колонки действий (там количество), поэтому метка живёт в ключевой колонке -
  // иначе третий тип вложения остался бы без признака.
  it('ТМЦ получают ту же метку', () => {
    const wrapper = mountAttachment('items', [item(mark({ status: 'approved' }))]);

    expect(wrapper.find(BADGE).text()).toBe('Согласовано, ждёт принятия');
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
    expect(wrapper.find(BADGE).text()).toBe('Новое, на согласовании');
  });

  it('метку получает только помеченная строка списка', () => {
    const wrapper = mountAttachment('cars', [
      car({ id: 1, car_number: 'А111АА' }),
      car({ id: 2, car_number: 'В222ВВ', ...mark({ status: 'pending' }) }),
    ]);

    const rows = wrapper.findAll(ROW);
    expect(rows).toHaveLength(2);
    expect(rows[0].find(BADGE).exists()).toBe(false);
    expect(rows[1].find(BADGE).text()).toBe('Новое, на согласовании');
  });
});
