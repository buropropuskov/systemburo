import { describe, it, expect } from 'vitest';
import { pickOverflowFields, columnMinWidth, DEFAULT_COLUMN_MIN_WIDTH } from '../tableColumnFit';

// #1307: столбцы не сжимаются до нечитаемых - лишние скрываются, начиная с
// наименее важных, а при равной важности - с правых.

const FIELDS = ['car_number', 'car_brand', 'organization', 'company', 'application_id'];
const PRIORITIES = { car_number: 1, car_brand: 3, organization: 3, company: 4, application_id: 4 };
const ORDERS = { car_number: 0, car_brand: 1, organization: 2, company: 3, application_id: 4 };

const fit = (available, extra = {}) =>
  pickOverflowFields({ fields: FIELDS, available, priorities: PRIORITIES, orders: ORDERS, ...extra });

describe('pickOverflowFields (#1307)', () => {
  it('ничего не скрывает, когда всё помещается', () => {
    expect(fit(2000)).toEqual([]);
  });

  it('первым уходит наименее важный столбец, и это правый из равных', () => {
    // Сумма минимумов: 120+100+150+150+135 = 655.
    // company и application_id имеют приоритет 4, правее - application_id.
    expect(fit(600)).toEqual(['application_id']);
  });

  it('скрывает по очереди, пока не поместится', () => {
    // 655 -> без application_id 520 -> без company 370.
    expect(fit(400)).toEqual(['application_id', 'company']);
  });

  it('при равном приоритете правый столбец уходит раньше левого', () => {
    const hidden = pickOverflowFields({
      fields: ['car_brand', 'organization'],
      available: 200,
      priorities: { car_brand: 3, organization: 3 },
      orders: { car_brand: 1, organization: 2 },
      keepAtLeast: 1,
    });
    expect(hidden).toEqual(['organization']);
  });

  it('поле без приоритета считается важным и скрывается последним', () => {
    const hidden = pickOverflowFields({
      fields: ['car_number', 'company'],
      available: 150,
      priorities: { company: 4 },
      orders: { car_number: 0, company: 1 },
      keepAtLeast: 1,
    });
    expect(hidden).toEqual(['company']);
  });

  it('учитывает ширину служебных столбцов', () => {
    // 655 помещается в 700, но со служебными столбцами (216) уже нет:
    // 871 -> без application_id 736 -> без company 586.
    expect(fit(700)).toEqual([]);
    expect(fit(700, { reserved: 216 })).toEqual(['application_id', 'company']);
  });

  it('оставляет минимум столбцов даже когда места совсем нет', () => {
    expect(fit(50)).toEqual(['application_id', 'company', 'organization']);
    expect(fit(50, { keepAtLeast: 1 })).toEqual(['application_id', 'company', 'organization', 'car_brand']);
  });

  it('без измеренной ширины ничего не скрывает - иначе столбцы моргнут на первом кадре', () => {
    expect(fit(0)).toEqual([]);
    expect(pickOverflowFields({ fields: FIELDS, available: undefined })).toEqual([]);
  });

  it('для незнакомого поля берётся ширина по умолчанию', () => {
    expect(columnMinWidth('car_number')).toBe(120);
    expect(columnMinWidth('какое_то_поле')).toBe(DEFAULT_COLUMN_MIN_WIDTH);
  });
});
