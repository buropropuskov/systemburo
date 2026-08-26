import { describe, it, expect } from 'vitest';
import { sortApplications } from '../applicationSort';

// Сортировка списка ЛК вынесена из UserApplications.vue как есть: поведение колонок
// и порядок по умолчанию должны совпадать с тем, что было в компоненте.

const apps = [
  { id: 1, application_number: 'A-3', sending_datetime: '2026-01-02T10:00:00Z', status: 'Отклонена', confirmation: 'Отклонено', sender_name: 'Петров' },
  { id: 2, application_number: 'A-1', sending_datetime: '2026-01-03T10:00:00Z', status: 'В обработке', confirmation: 'Согласовано', sender_name: 'Иванов' },
  { id: 3, application_number: 'A-2', sending_datetime: '2026-01-01T10:00:00Z', status: 'Закрыта', confirmation: 'Согласование', sender_name: 'Сидоров' },
];

const numbers = (list) => list.map((a) => a.application_number);

describe('sortApplications', () => {
  it('без выбранной колонки - по дате подачи, новые сверху', () => {
    expect(numbers(sortApplications(apps, null))).toEqual(['A-1', 'A-3', 'A-2']);
  });

  it('по номеру заявки в обе стороны', () => {
    expect(numbers(sortApplications(apps, 'application_number', 'asc'))).toEqual(['A-1', 'A-2', 'A-3']);
    expect(numbers(sortApplications(apps, 'application_number', 'desc'))).toEqual(['A-3', 'A-2', 'A-1']);
  });

  it('по отправителю берёт sender_full_name, когда sender_name пуст', () => {
    const mixed = [
      { application_number: 'A-1', sender_name: '', sender_full_name: 'Яковлев' },
      { application_number: 'A-2', sender_name: 'Абрамов' },
    ];
    expect(numbers(sortApplications(mixed, 'sender_name', 'asc'))).toEqual(['A-2', 'A-1']);
  });

  it('неизвестная колонка не меняет порядок и не мутирует исходный массив', () => {
    const source = [...apps];
    expect(numbers(sortApplications(apps, 'нет такой'))).toEqual(['A-3', 'A-1', 'A-2']);
    expect(apps).toEqual(source);
  });
});
