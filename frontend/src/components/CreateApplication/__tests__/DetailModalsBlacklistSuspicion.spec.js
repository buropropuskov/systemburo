import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import VehicleDetailsModal from '../VehicleDetailsModal.vue';
import EmployeeDetailsModal from '../EmployeeDetailsModal.vue';

// Карточки на show=true дёргают вспомогательные API (статус/история/проверка ЧС) - глушим
// их, нас интересует только блок "Подозрение на обход ЧС" (#481, срез C).
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false }) }));
vi.mock('@/api/blacklist', () => ({
  listVehicleBlacklist: vi.fn().mockResolvedValue([]),
  createVehicleBlacklist: vi.fn().mockResolvedValue({}),
  listPersonBlacklist: vi.fn().mockResolvedValue([]),
  checkPersonBlacklist: vi.fn().mockResolvedValue({ is_blacklisted: false }),
  createPersonBlacklist: vi.fn().mockResolvedValue({}),
}));
vi.mock('@/api/marks', () => ({ listMarks: vi.fn().mockResolvedValue([]) }));
vi.mock('exceljs', () => ({ default: {} }));

const stubs = {
  teleport: true,
  UnloadPlaceModal: true,
  CarHistoryModal: true,
  EmployeeHistoryModal: true,
  TableInfoModal: true,
  AddToBlacklistModal: true,
  LoaderSpinner: true,
};

function vehicleFlag(over = {}) {
  return { flag_id: 5, matched_value: 'А124ВС Toyota', matched_reason: 'похожий номер', similarity: 0.9, overridden: false, ...over };
}

function mountVehicle(vehicleOver = {}, props = {}) {
  return mount(VehicleDetailsModal, {
    props: {
      show: true,
      vehicle: { id: 1, plateNumber: 'А123ВС', mark: 'Toyota', unloadPlaces: [], ...vehicleOver },
      source: 'application',
      ...props,
    },
    global: { stubs },
  });
}

function mountEmployee(employeeOver = {}, props = {}) {
  return mount(EmployeeDetailsModal, {
    props: {
      show: true,
      employee: { id: 1, last_name: 'Иваноф', first_name: 'Иван', middle_name: 'Иванович', target_tables: [], ...employeeOver },
      source: 'application',
      ...props,
    },
    global: { stubs },
  });
}

describe('VehicleDetailsModal - блок подозрения на обход ЧС (#481, срез C)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('показывает "похоже на X" и причину при флаге в контексте заявки', () => {
    const w = mountVehicle({ blacklist_similar: vehicleFlag() }, { canOverride: true });
    const sec = w.find('.bl-suspicion-section');
    expect(sec.exists()).toBe(true);
    expect(sec.text()).toContain('Подозрение на обход чёрного списка');
    expect(sec.text()).toContain('А124ВС Toyota');
    expect(sec.text()).toContain('похожий номер');
    expect(sec.classes()).not.toContain('is-resolved');
  });

  it('не overridden + право: кнопка "Всё равно пропустить" эмитит override', async () => {
    const w = mountVehicle({ blacklist_similar: vehicleFlag() }, { canOverride: true });
    const btn = w.find('.bl-suspicion-btn--allow');
    expect(btn.exists()).toBe(true);
    await btn.trigger('click');
    expect(w.emitted('override')).toBeTruthy();
  });

  it('без права override кнопка "Всё равно пропустить" скрыта', () => {
    const w = mountVehicle({ blacklist_similar: vehicleFlag() }, { canOverride: false });
    expect(w.find('.bl-suspicion-btn--allow').exists()).toBe(false);
  });

  it('overridden + право: статус "Пропуск подтверждён", is-resolved, "Отменить" эмитит cancel-override', async () => {
    const w = mountVehicle({ blacklist_similar: vehicleFlag({ overridden: true }) }, { canCancelOverride: true });
    const sec = w.find('.bl-suspicion-section');
    expect(sec.classes()).toContain('is-resolved');
    expect(sec.text()).toContain('Пропуск подтверждён');
    const btn = w.find('.bl-suspicion-btn--cancel');
    expect(btn.exists()).toBe(true);
    await btn.trigger('click');
    expect(w.emitted('cancel-override')).toBeTruthy();
  });

  it('overridden без права отмены: "Отменить" скрыта', () => {
    const w = mountVehicle({ blacklist_similar: vehicleFlag({ overridden: true }) }, { canCancelOverride: false });
    expect(w.find('.bl-suspicion-btn--cancel').exists()).toBe(false);
  });

  it('вне контекста заявки (source != application) блок скрыт', () => {
    const w = mountVehicle({ blacklist_similar: vehicleFlag() }, { source: 'general', canOverride: true });
    expect(w.find('.bl-suspicion-section').exists()).toBe(false);
  });

  it('без флага похожести блок не рендерится', () => {
    const w = mountVehicle({}, { canOverride: true });
    expect(w.find('.bl-suspicion-section').exists()).toBe(false);
  });
});

describe('EmployeeDetailsModal - блок подозрения на обход ЧС (#481, срез C)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('показывает похожее ФИО и эмитит cancel-override при overridden', async () => {
    const flag = { flag_id: 7, matched_value: 'Иванов Иван Иванович', matched_reason: 'опечатка в фамилии', similarity: 0.92, overridden: true };
    const w = mountEmployee({ blacklist_similar: flag }, { canCancelOverride: true });
    const sec = w.find('.bl-suspicion-section');
    expect(sec.exists()).toBe(true);
    expect(sec.text()).toContain('Иванов Иван Иванович');
    expect(sec.text()).toContain('опечатка в фамилии');
    const btn = w.find('.bl-suspicion-btn--cancel');
    expect(btn.exists()).toBe(true);
    await btn.trigger('click');
    expect(w.emitted('cancel-override')).toBeTruthy();
  });

  it('не overridden + право: "Всё равно пропустить" эмитит override', async () => {
    const flag = { flag_id: 8, matched_value: 'Петров Пётр', matched_reason: '', similarity: 0.8, overridden: false };
    const w = mountEmployee({ blacklist_similar: flag }, { canOverride: true });
    const btn = w.find('.bl-suspicion-btn--allow');
    expect(btn.exists()).toBe(true);
    await btn.trigger('click');
    expect(w.emitted('override')).toBeTruthy();
  });
});
