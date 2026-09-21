import { describe, it, expect } from 'vitest';
import {
  passageSourceLabel,
  passageSourceVariant,
  activePassageTables,
  removedPassageTables,
} from '../passageSource';

const имяПоста = (id) => `Пост ${id}`;

describe('подпись источника места прохода', () => {
  it('различает все три пути появления привязки', () => {
    expect(passageSourceLabel('application')).toBe('из заявки');
    expect(passageSourceLabel('approver')).toBe('назначил принимающий');
    expect(passageSourceLabel('manual')).toBe('добавлено вручную');
  });

  it('молчит, когда источник неизвестен', () => {
    expect(passageSourceLabel(null)).toBe('');
    expect(passageSourceLabel(undefined)).toBe('');
    expect(passageSourceLabel('что-то новое')).toBe('');
  });

  it('выделяет только указанное самим заявителем', () => {
    expect(passageSourceVariant('application')).toBe('primary');
    expect(passageSourceVariant('approver')).toBe('neutral');
    expect(passageSourceVariant('manual')).toBe('neutral');
  });
});

describe('разбор мест прохода', () => {
  it('понимает объекты с источником и подписывает их', () => {
    const [пост] = activePassageTables([{ id: 7, name: 'КПП №4', source: 'approver' }], имяПоста);
    expect(пост).toMatchObject({ id: 7, name: 'КПП №4', source: 'approver', sourceLabel: 'назначил принимающий' });
  });

  it('понимает голые идентификаторы и берёт название со стороны', () => {
    const [пост] = activePassageTables([5], имяПоста);
    expect(пост).toMatchObject({ id: 5, name: 'Пост 5', source: null, sourceLabel: '' });
  });

  it('не выдумывает источник, если его не прислали', () => {
    const [пост] = activePassageTables([{ id: 3, name: 'Проверка' }], имяПоста);
    expect(пост.source).toBeNull();
    expect(пост.sourceVariant).toBe('neutral');
  });

  it('пустой список остаётся пустым', () => {
    expect(activePassageTables(null, имяПоста)).toEqual([]);
  });
});

describe('снятые места прохода', () => {
  const история = [
    { action_type: 'unbound_from_table', table_id: 1, table_name: 'КПП №4' },
    { action_type: 'unbound_from_table', table_id: 1, table_name: 'КПП №4' },
    { action_type: 'moved_between_tables', table_id: 2, table_name: 'ПОСТ №72' },
    { action_type: 'entry', table_id: 9, table_name: 'Проверка' },
  ];

  it('показывает снятые один раз и не трогает активные', () => {
    const активные = activePassageTables([{ id: 2, name: 'ПОСТ №72', source: 'manual' }], имяПоста);
    expect(removedPassageTables(история, активные, имяПоста)).toEqual([{ id: 1, name: 'КПП №4' }]);
  });

  it('молчит там, где источник неизвестен: в заявке истории привязок нет', () => {
    const активные = activePassageTables([2], имяПоста);
    expect(removedPassageTables(история, активные, имяПоста)).toEqual([]);
  });

  it('переживает историю неожиданной формы', () => {
    const активные = activePassageTables([{ id: 4, source: 'manual' }], имяПоста);
    expect(removedPassageTables(undefined, активные, имяПоста)).toEqual([]);
  });
});
