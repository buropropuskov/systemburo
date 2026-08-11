import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  pickOverflowFields,
  columnMinWidth,
  measureRowAvailableWidth,
  DEFAULT_COLUMN_MIN_WIDTH,
} from '../tableColumnFit';

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

// #1097 S8 (волна 4): раскладку считаем по строке заголовков, а не по ширине всей
// области - иначе отступы строки и зазоры между ячейками уходят в запас, и на
// планшетной ширине в раскладке остаётся столбец, который в неё не влезает.
describe('measureRowAvailableWidth', () => {
  const row = ({ width, cells = 0, pad = '10px', gap = '4px' }) => {
    const el = document.createElement('div');
    Object.defineProperty(el, 'clientWidth', { value: width });
    el.dataset.pad = pad;
    el.dataset.gap = gap;
    for (let i = 0; i < cells; i += 1) el.appendChild(document.createElement('div'));
    return el;
  };

  beforeEach(() => {
    // jsdom не считает раскладку: подставляем геометрию, объявленную в компоненте.
    vi.spyOn(window, 'getComputedStyle').mockImplementation((el) => ({
      paddingLeft: el.dataset.pad,
      paddingRight: el.dataset.pad,
      columnGap: el.dataset.gap,
    }));
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('вычитает боковые отступы строки и зазоры между ячейками', () => {
    // Таблица проходной: 13 ячеек -> 12 зазоров по 4px, плюс 10+10 отступов строки.
    expect(measureRowAvailableWidth(row({ width: 990, cells: 13 }))).toBe(922);
  });

  it('схлопнутая по приоритету ячейка остаётся flex-элементом - её зазор тоже считается', () => {
    // Скрытие столбца освобождает его ширину, но не зазор: ячейка остаётся в DOM
    // с нулевой шириной, и gap с обеих сторон от неё никуда не девается.
    const plain = row({ width: 500, cells: 5 });
    const collapsed = row({ width: 500, cells: 5 });
    collapsed.children[1].className = 'col col--collapsed';
    collapsed.children[3].className = 'col col--collapsed';

    expect(measureRowAvailableWidth(plain)).toBe(464);
    expect(measureRowAvailableWidth(collapsed)).toBe(464);
  });

  it('у единственной ячейки зазоров нет', () => {
    expect(measureRowAvailableWidth(row({ width: 300, cells: 1 }))).toBe(280);
  });

  it('скрытая строка даёт 0 - вызывающий берёт прежний источник ширины', () => {
    expect(measureRowAvailableWidth(row({ width: 0, cells: 13 }))).toBe(0);
    expect(measureRowAvailableWidth(null)).toBe(0);
    expect(measureRowAvailableWidth(undefined)).toBe(0);
  });

  it('не уходит в минус на узкой строке с широкими отступами', () => {
    expect(measureRowAvailableWidth(row({ width: 20, cells: 6, pad: '40px' }))).toBe(0);
  });
});
