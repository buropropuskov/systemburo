import { describe, it, expect } from 'vitest';
import {
  canEditApplicationDates,
  formatPeriod,
  periodFormErrors,
  periodFormFromAttachment,
  periodPayloadFromForm,
} from '../entryWindow';

const ATTACHMENT = {
  entry_date_from: '2099-10-01',
  entry_date_to: '2099-10-03',
  entry_time_from: '09:00:00',
  entry_time_to: '18:00:00',
};

describe('canEditApplicationDates', () => {
  it('открыта до принятия и итога согласования', () => {
    expect(canEditApplicationDates({ status: 'Непрочитано', confirmation: null })).toBe(true);
    expect(canEditApplicationDates({ status: 'В обработке', confirmation: 'Согласование' })).toBe(true);
  });

  it('закрыта в работе, после итога согласования и без заявки', () => {
    expect(canEditApplicationDates({ status: 'В работе', confirmation: null })).toBe(false);
    expect(canEditApplicationDates({ status: 'В обработке', confirmation: 'Согласовано' })).toBe(false);
    expect(canEditApplicationDates({ status: 'В обработке', confirmation: 'Не согласовано' })).toBe(false);
    expect(canEditApplicationDates({ status: 'Отказано', confirmation: null })).toBe(false);
    expect(canEditApplicationDates(null)).toBe(false);
  });
});

describe('окно вложения <-> форма дат', () => {
  it('период раскладывается в поля «с/по» и обратно без потерь', () => {
    const form = periodFormFromAttachment(ATTACHMENT);
    expect(form).toEqual({
      isOneDay: false, startDate: '01.10.2099', endDate: '03.10.2099', singleDate: '',
      startTime: '09:00', endTime: '18:00',
    });
    expect(periodPayloadFromForm(form)).toEqual(ATTACHMENT);
  });

  it('один день идёт через singleDate', () => {
    const form = periodFormFromAttachment({ ...ATTACHMENT, entry_date_to: '2099-10-01' });
    expect(form.isOneDay).toBe(true);
    expect(form.singleDate).toBe('01.10.2099');
    expect(periodPayloadFromForm(form).entry_date_to).toBe('2099-10-01');
  });

  it('человеку окно показывается датами и временем без секунд', () => {
    expect(formatPeriod(ATTACHMENT)).toBe('01.10.2099 09:00 - 03.10.2099 18:00');
  });
});

describe('periodFormErrors', () => {
  const valid = periodFormFromAttachment(ATTACHMENT);

  it('корректное окно без ошибок', () => {
    expect(periodFormErrors(valid)).toEqual({});
  });

  it('пустые поля названы по ключам DateRangeSection', () => {
    const errors = periodFormErrors({ ...valid, startDate: '', startTime: '9' });
    expect(Object.keys(errors).sort()).toEqual(['startDate', 'startTime']);
  });

  it('прошедший срок не уходит на сервер: сравнение по часам бюро', () => {
    expect(periodFormErrors(valid, '2099-10-03T18:00:00').endDate).toMatch(/истёк/);
    expect(periodFormErrors(valid, '2099-10-03T17:59:59')).toEqual({});
  });

  it('окончание раньше начала', () => {
    expect(periodFormErrors({ ...valid, startDate: '05.10.2099' }).endDate).toMatch(/раньше/);
  });

  it('в один день время окончания обязано быть позже начала', () => {
    const oneDay = periodFormFromAttachment({ ...ATTACHMENT, entry_date_to: '2099-10-01' });
    expect(periodFormErrors({ ...oneDay, startTime: '18:00', endTime: '09:00' }).endTime).toMatch(/позже/);
    expect(periodFormErrors({ ...oneDay, startTime: '09:00', endTime: '09:00' }).endTime).toMatch(/позже/);
  });
});
