import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import EmployeeDetailsModal from '../EmployeeDetailsModal.vue';
import { usePermissionsStore } from '@/stores/permissions';

// Раздел «Документы» (паспорт/патент) гейтится правом detail.documents по карте
// detailModalActions (#187 Фаза 2). Вспомогательные API на show=true глушим.
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false }) }));
vi.mock('@/api/blacklist', () => ({
  checkPersonBlacklist: vi.fn().mockResolvedValue({ is_blacklisted: false }),
  createPersonBlacklist: vi.fn().mockResolvedValue({}),
}));
vi.mock('exceljs', () => ({ default: {} }));

const stubs = { teleport: true, EmployeeHistoryModal: true, TableInfoModal: true, AddToBlacklistModal: true };

const PASSPORT_LABEL = 'Серия и номер паспорта';

function mountEmployee(source, setupStore) {
  setActivePinia(createPinia());
  setupStore(usePermissionsStore());
  return mount(EmployeeDetailsModal, {
    props: {
      show: true,
      employee: { id: 1, last_name: 'Иваноф', first_name: 'Иван', passport_series_number: '1234 567890', target_tables: [] },
      source,
    },
    global: { stubs },
  });
}

describe('EmployeeDetailsModal - гейтинг раздела Документы (#187 Фаза 2)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('обычный юзер с detail.documents видит документы своего сотрудника', () => {
    const w = mountEmployee('employeesview', (s) => {
      s.mode = 'normal';
      s.effective = { 'detail.documents': { value: 'allow', source: 'role' } };
    });
    expect(w.text()).toContain(PASSPORT_LABEL);
  });

  it('без права detail.documents раздел скрыт', () => {
    const w = mountEmployee('employeesview', (s) => {
      s.mode = 'normal';
      s.effective = {};
    });
    expect(w.text()).not.toContain(PASSPORT_LABEL);
  });

  it('super-admin видит документы', () => {
    const w = mountEmployee('employeesview', (s) => { s.mode = 'super'; });
    expect(w.text()).toContain(PASSPORT_LABEL);
  });

  it('в корзине документы скрыты по контексту даже у super', () => {
    const w = mountEmployee('trash', (s) => { s.mode = 'super'; });
    expect(w.text()).not.toContain(PASSPORT_LABEL);
  });
});
