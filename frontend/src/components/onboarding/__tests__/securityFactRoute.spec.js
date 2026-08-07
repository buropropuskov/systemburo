import { describe, it, expect, beforeEach, vi } from 'vitest';

/**
 * Резолв фактовой таблицы для тура охраны на границе с сетью и правами.
 * Чистый отбор проверяется на resolveFactTableRoute (securityOnboardingSteps.spec.js),
 * здесь - проводка: что уходит в запрос, чем отсеивается недоступная таблица и что
 * происходит, пока права не загружены.
 */

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/stores/permissions', () => ({ usePermissionsStore: () => permissions }));

import { apiRequest } from '@/api/client';
import { getSecurityFactRoute } from '@/api/onboarding';

// Ссылка на объект берётся внутри фабрики лениво (вызов происходит уже в тесте),
// поэтому hoisting vi.mock ей не мешает.
const permissions = { loaded: true, hasPermission: vi.fn(() => true) };

const table = (name, type) => ({
  table: { name, table_type: type, show_fact_table: true, is_active: true },
});

function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) };
}

describe('getSecurityFactRoute', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    permissions.loaded = true;
    permissions.hasPermission = vi.fn(() => true);
  });

  it('берёт список системных таблиц и отдаёт машинную фактовую', async () => {
    apiRequest.mockResolvedValue(okJson([table('people_1', 'people'), table('kpp_1', 'cars')]));

    await expect(getSecurityFactRoute()).resolves.toBe('/table/kpp_1');
    expect(apiRequest).toHaveBeenCalledWith('/system-tables');
  });

  it('таблица без права table.<name>.view пропускается - тур не поведёт на роут-гард', async () => {
    apiRequest.mockResolvedValue(okJson([table('kpp_1', 'cars'), table('people_1', 'people')]));
    permissions.hasPermission = vi.fn((key) => key !== 'table.kpp_1.view');

    await expect(getSecurityFactRoute()).resolves.toBe('/table/people_1');
    expect(permissions.hasPermission).toHaveBeenCalledWith('table.kpp_1.view');
  });

  it('ни одной доступной таблицы - null', async () => {
    apiRequest.mockResolvedValue(okJson([table('kpp_1', 'cars')]));
    permissions.hasPermission = vi.fn(() => false);

    await expect(getSecurityFactRoute()).resolves.toBe(null);
  });

  it('права ещё не загружены - отбор идёт без них, иначе сегмент потерялся бы', async () => {
    apiRequest.mockResolvedValue(okJson([table('kpp_1', 'cars')]));
    permissions.loaded = false;
    permissions.hasPermission = vi.fn(() => false);

    await expect(getSecurityFactRoute()).resolves.toBe('/table/kpp_1');
    expect(permissions.hasPermission).not.toHaveBeenCalled();
  });

  it('ошибка ответа и исключение сети дают null, а не роняют тур', async () => {
    apiRequest.mockResolvedValue({ ok: false, status: 403, json: vi.fn() });
    await expect(getSecurityFactRoute()).resolves.toBe(null);

    apiRequest.mockRejectedValue(new Error('network'));
    await expect(getSecurityFactRoute()).resolves.toBe(null);
  });
});
