import { describe, it, expect } from 'vitest';
import { normalizeSnapshotRows } from '@/utils/snapshotRows';

// FE #980 polish-r2: preview-режим CarsTable/PeopleTable на вкладке версий берёт
// обогащённую форму item (organization_name, applicationNumber, статус через
// entry/exit), а снимок хранит сырой DTO страницы (organization, application_number,
// territory_status). Нормализатор мостит формы - фиксируем маппинг ключей и проекцию
// территориального статуса на кнопки въезда/выезда.

describe('normalizeSnapshotRows', () => {
  it('cars: маппит organization/application_number в форму рендера CarsTable', () => {
    const [item] = normalizeSnapshotRows(
      [{
        id: 42,
        car_number: 'А123ВС77',
        car_brand: 'BMW',
        organization: 'ООО Ромашка',
        organization_id: 7,
        company: 'Транс',
        unload_place: 'Склад А',
        entry_date_to: '2026-07-05',
        application_number: '20260705/001',
        application_id: 100,
        territory_status: 1,
      }],
      'cars',
    );
    expect(item.organization_name).toBe('ООО Ромашка');
    expect(item.applicationNumber).toBe('20260705/001');
    expect(item.car_number).toBe('А123ВС77');
    expect(item.unload_place).toBe('Склад А');
    // territory_status=1 -> на территории: въезд отмечен, выезд нет.
    expect(item.entry_checked).toBe(true);
    expect(item.exit_checked).toBe(false);
  });

  it('cars: статус выехал/не въезжал проецируется на entry/exit взаимоисключающе', () => {
    const [exited, notEntered] = normalizeSnapshotRows(
      [
        { id: 1, car_number: 'Х1', territory_status: 2 },
        { id: 2, car_number: 'Х2', territory_status: 0 },
      ],
      'cars',
    );
    // territory_status=2 (выехал): Въезд НЕ отмечен, Выехал отмечен - как на
    // основной странице (item.entry_checked = ts===1, item.exit_checked = ts===2).
    expect(exited.entry_checked).toBe(false);
    expect(exited.exit_checked).toBe(true);
    // territory_status=0 (не въезжал): ни одной.
    expect(notEntered.entry_checked).toBe(false);
    expect(notEntered.exit_checked).toBe(false);
  });

  it('cars: пустая организация -> "Не указана", место -> "-"', () => {
    const [item] = normalizeSnapshotRows([{ id: 1, car_number: 'Х1' }], 'cars');
    expect(item.organization_name).toBe('Не указана');
    expect(item.unload_place).toBe('-');
    expect(item.unload_place_ids).toEqual([]);
  });

  it('people: маппит ФИО/citizenship_name/position в форму PeopleTable', () => {
    const [item] = normalizeSnapshotRows(
      [{
        id: 5,
        last_name: 'Петров',
        first_name: 'Пётр',
        middle_name: 'Петрович',
        organization: 'ООО В',
        position: 'Грузчик',
        citizenship_name: 'Россия',
        pass_time: '08:00 - 18:00',
        application_number: '20260705/009',
        territory_status: 1,
      }],
      'people',
    );
    expect(item.last_name).toBe('Петров');
    expect(item.position).toBe('Грузчик');
    // Рендер PeopleTable читает item.citizenshipName (camelCase).
    expect(item.citizenshipName).toBe('Россия');
    expect(item.organization_name).toBe('ООО В');
    expect(item.applicationNumber).toBe('20260705/009');
    expect(item.pass_time).toBe('08:00 - 18:00');
    expect(item.entry_checked).toBe(true);
  });

  it('гарантирует уникальность id при отсутствии/дублях (:key preview)', () => {
    const items = normalizeSnapshotRows(
      [
        { car_number: 'Х1' }, // нет id
        { id: 9, car_number: 'Х2' },
        { id: 9, car_number: 'Х3' }, // дубль id (обычная vs fact)
      ],
      'cars',
    );
    const ids = items.map((i) => i.id);
    expect(new Set(ids).size).toBe(3);
  });

  it('нечисловой/пустой rows -> []', () => {
    expect(normalizeSnapshotRows(null, 'cars')).toEqual([]);
    expect(normalizeSnapshotRows(undefined, 'people')).toEqual([]);
    expect(normalizeSnapshotRows('nope', 'cars')).toEqual([]);
  });
});
