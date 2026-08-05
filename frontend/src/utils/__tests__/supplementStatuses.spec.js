import { describe, it, expect } from 'vitest';
import {
  SUPPLEMENT_OPEN_STATUSES,
  SUPPLEMENT_REVOCABLE_STATUSES,
  supplementStatusText,
  supplementStatusClass,
  supplementCountsLabel,
  isOpenSupplement,
} from '../supplementStatuses';

describe('supplementStatuses - статусы раунда дополнения (#1685)', () => {
  it('перечни открытых и отзывных статусов повторяют бэк', () => {
    expect(SUPPLEMENT_OPEN_STATUSES).toEqual(['pending', 'approved']);
    expect(SUPPLEMENT_REVOCABLE_STATUSES).toEqual(['pending', 'approved', 'rejected']);
    expect(isOpenSupplement('pending')).toBe(true);
    expect(isOpenSupplement('accepted')).toBe(false);
  });

  it('каждый статус получает подпись, неизвестный не рисуется сырым кодом', () => {
    expect(supplementStatusText('merged')).toBe('Влито в заявку');
    expect(supplementStatusText('pending')).toBe('На согласовании');
    expect(supplementStatusText('approved')).toBe('Согласовано');
    expect(supplementStatusText('rejected')).toBe('Отказано в согласовании');
    expect(supplementStatusText('accepted')).toBe('Принято');
    expect(supplementStatusText('refused')).toBe('Отказано');
    expect(supplementStatusText('cancelled')).toBe('Снято');
    expect(supplementStatusText('что-то новое')).toBe('Неизвестно');
  });

  it('отказ согласующего и отказ принимающего красятся одинаково, влитое и снятое - нейтрально', () => {
    expect(supplementStatusClass('rejected')).toBe('supplement-status--rejected');
    expect(supplementStatusClass('refused')).toBe('supplement-status--rejected');
    expect(supplementStatusClass('merged')).toBe('supplement-status--neutral');
    expect(supplementStatusClass('cancelled')).toBe('supplement-status--neutral');
  });
});

describe('supplementCountsLabel - состав раунда (#1685)', () => {
  it('склоняет по числу и опускает нулевые типы', () => {
    expect(supplementCountsLabel({ vehicles: 1, employees: 0, items: 0 })).toBe('1 машина');
    expect(supplementCountsLabel({ vehicles: 2, employees: 5, items: 0 })).toBe('2 машины, 5 сотрудников');
    expect(supplementCountsLabel({ vehicles: 0, employees: 0, items: 3 })).toBe('3 позиции ТМЦ');
  });

  it('11-14 берут третью форму, а не «одиннадцать машина»', () => {
    expect(supplementCountsLabel({ vehicles: 11 })).toBe('11 машин');
    expect(supplementCountsLabel({ employees: 14 })).toBe('14 сотрудников');
    expect(supplementCountsLabel({ vehicles: 21 })).toBe('21 машина');
  });

  it('пустой состав даёт пустую строку - потребитель строку «Добавлено:» тогда не рисует', () => {
    expect(supplementCountsLabel({ vehicles: 0, employees: 0, items: 0 })).toBe('');
    expect(supplementCountsLabel(undefined)).toBe('');
  });
});
