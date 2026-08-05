import { describe, it, expect } from 'vitest';
import { approvalStatusText, approvalStatusClass, useApprovalStatus } from '../useApprovalStatus';

describe('useApprovalStatus - общий словарь голосов согласующих (#1685)', () => {
  it('переводит три состояния голоса', () => {
    expect(approvalStatusText('approved')).toBe('Согласовано');
    expect(approvalStatusText('rejected')).toBe('Отказано');
    expect(approvalStatusText('pending')).toBe('Ожидание');
  });

  it('даёт каждому статусу свой класс бейджа', () => {
    expect(approvalStatusClass('approved')).toBe('status-approved');
    expect(approvalStatusClass('rejected')).toBe('status-rejected');
    expect(approvalStatusClass('pending')).toBe('status-pending');
  });

  it('незнакомый и пустой статус остаются «неизвестными» - приведение к pending дело потребителя', () => {
    expect(approvalStatusText(undefined)).toBe('Неизвестно');
    expect(approvalStatusText(null)).toBe('Неизвестно');
    expect(approvalStatusText('whatever')).toBe('Неизвестно');
    expect(approvalStatusClass(null)).toBe('status-default');
  });

  it('отдаёт хелперы под именами методов, которые ждут шаблоны Options API', () => {
    const { getStatusText, getStatusClass } = useApprovalStatus();
    expect(getStatusText('approved')).toBe('Согласовано');
    expect(getStatusClass('rejected')).toBe('status-rejected');
  });
});
