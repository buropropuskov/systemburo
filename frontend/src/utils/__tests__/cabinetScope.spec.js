import { describe, it, expect } from 'vitest';
import { cabinetScopeParams } from '../cabinetScope';

/**
 * Вкладка кабинета одним набором параметров (#2339). Список и чип «Обновления» должны
 * сужаться одинаково: пока их собирали в двух местах, чип считал по всему скоупу ЛК и
 * обещал 11 обновлений там, где клик открывал 3.
 */
describe('cabinetScopeParams', () => {
  it('вкладка «Мои заявки» сужает по автору', () => {
    expect(cabinetScopeParams('my', 7, 42)).toEqual({ sender_user_id: 7 });
  });

  it('вкладка «Организация» сужает по организации', () => {
    expect(cabinetScopeParams('organization', 7, 42)).toEqual({ organization_id: 42 });
  });

  it('без известного id запрос уходит без сужения', () => {
    // Пустой набор означает «весь скоуп кабинета» - это осознанно: сузить нечем, пока
    // не приехал /users/me, и лучше показать больше, чем показать чужую вкладку.
    expect(cabinetScopeParams('my', null, 42)).toEqual({});
    expect(cabinetScopeParams('organization', 7, null)).toEqual({});
  });

  it('вкладки не смешиваются: у каждой ровно свой параметр', () => {
    expect(cabinetScopeParams('my', 7, 42).organization_id).toBeUndefined();
    expect(cabinetScopeParams('organization', 7, 42).sender_user_id).toBeUndefined();
  });
});
