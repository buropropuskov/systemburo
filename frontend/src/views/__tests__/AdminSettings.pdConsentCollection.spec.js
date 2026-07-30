import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// Сбор согласий (#1567): администратор видит, сколько человек подтвердили текущую
// редакцию и кто ещё нет. Числа приходят с сервера - фронт их не пересчитывает,
// потому что серверная формула совпадает с гейтом, а своя разошлась бы с ним.

const getSettings = vi.fn();
vi.mock('@/api/settings', () => ({
  getSettings: (...a) => getSettings(...a),
  updateSetting: vi.fn(),
}));

vi.mock('@/api/dataProcessing', () => ({
  getDataProcessingMeta: vi.fn().mockResolvedValue(null),
  fetchDataProcessingBlob: vi.fn(),
  uploadDataProcessingDoc: vi.fn(),
  deleteDataProcessingDoc: vi.fn(),
  downloadDataProcessingDoc: vi.fn(),
}));

const getPDConsentSettings = vi.fn();
const getPDConsentCollection = vi.fn();
const requirePDConsentAgain = vi.fn();
vi.mock('@/api/pdConsent', () => ({
  getPDConsentSettings: (...a) => getPDConsentSettings(...a),
  savePDConsentText: vi.fn(),
  setPDConsentRequired: vi.fn(),
  requirePDConsentAgain: (...a) => requirePDConsentAgain(...a),
  getPDConsentCollection: (...a) => getPDConsentCollection(...a),
}));

vi.mock('@/utils/documentTextExtract', () => ({
  extractDocumentHtml: vi.fn(),
  UnsupportedDocumentError: class extends Error {},
}));

import AdminSettings from '../AdminSettings.vue';
import { useUiStore } from '@/stores/ui';
import RefreshButton from '@/components/RefreshButton.vue';

const renderSlot = { template: '<div><slot /></div>' };

const collection = (over = {}) => ({
  active: true,
  version: 2,
  total: 10,
  accepted: 7,
  pending: 3,
  pending_users: [
    { id: 2, username: 'petrov', full_name: 'Петров Пётр', organization: 'Феникс' },
    { id: 3, username: 'sidorov', full_name: 'Сидоров Сидор', organization: '' },
    { id: 4, username: 'ivanov', full_name: 'Иванов Иван', organization: 'Феникс' },
  ],
  ...over,
});

async function openSection() {
  getSettings.mockResolvedValue([]);
  const wrapper = shallowMount(AdminSettings, {
    global: { stubs: { SkeletonTransition: renderSlot } },
  });
  await flushPromises();
  wrapper.vm.activeSection = 'data-processing';
  await flushPromises();
  return wrapper;
}

describe('AdminSettings - сбор согласий', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    [getSettings, getPDConsentSettings, getPDConsentCollection, requirePDConsentAgain]
      .forEach((m) => m.mockReset());
    getPDConsentSettings.mockResolvedValue({ text: '<p>Текст</p>', version: 2, required: true });
    getPDConsentCollection.mockResolvedValue(collection());
  });

  it('показывает сводку и список не подтвердивших', async () => {
    const wrapper = await openSection();

    expect(getPDConsentCollection).toHaveBeenCalled();
    const counts = wrapper.get('[data-testid="pdc-collection-counts"]').text();
    expect(counts).toContain('7');
    expect(counts).toContain('10');
    expect(counts).toContain('70%');

    const pending = wrapper.get('[data-testid="pdc-collection-pending"]').text();
    expect(pending).toContain('Петров Пётр');
    expect(pending).toContain('petrov');
    expect(pending).toContain('Феникс');
  });

  // Пока запрос согласия выключен, подтвердить его нельзя в принципе: «0 из N»
  // было бы ложным сигналом «все игнорируют».
  it('при выключенном запросе показывает, что сбор не идёт, а не нулевой процент', async () => {
    getPDConsentCollection.mockResolvedValue(
      collection({ active: false, total: 10, accepted: 0, pending: 10 }),
    );
    const wrapper = await openSection();

    expect(wrapper.find('[data-testid="pdc-collection-inactive"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="pdc-collection-counts"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="pdc-collection-pending"]').exists()).toBe(false);
  });

  it('когда подтвердили все, списка нет и сказано об этом', async () => {
    getPDConsentCollection.mockResolvedValue(
      collection({ total: 5, accepted: 5, pending: 0, pending_users: [] }),
    );
    const wrapper = await openSection();

    expect(wrapper.find('[data-testid="pdc-collection-pending"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="pdc-collection-export"]').exists()).toBe(false);
    expect(wrapper.get('[data-testid="pdc-collection"]').text()).toContain('Подтвердили все');
  });

  // Сводка не должна ронять секцию: текст и тумблер важнее, они работают и без неё.
  it('сбой загрузки сводки прячет блок, но не ломает секцию', async () => {
    getPDConsentCollection.mockRejectedValue(new Error('503'));
    const wrapper = await openSection();

    expect(wrapper.find('[data-testid="pdc-collection"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="pdc-save"]').exists()).toBe(true);
  });

  // После подъёма редакции состав согласившихся обнуляется - сводка обязана
  // перечитаться, иначе администратор смотрит на числа прошлой редакции.
  it('подъём редакции перечитывает сводку', async () => {
    const wrapper = await openSection();
    getPDConsentCollection.mockClear();
    requirePDConsentAgain.mockResolvedValue({ text: '<p>Текст</p>', version: 3, required: true });
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);

    await wrapper.get('[data-testid="pdc-require-again"]').trigger('click');
    await flushPromises();

    expect(getPDConsentCollection).toHaveBeenCalledTimes(1);
  });

  it('кнопка обновления перечитывает сводку', async () => {
    const wrapper = await openSection();
    getPDConsentCollection.mockClear();

    // RefreshButton - общий компонент проекта; в shallowMount он заглушка, поэтому
    // дёргаем его событие, а не клик по стабу.
    await wrapper.findComponent(RefreshButton).vm.$emit('refresh');
    await flushPromises();

    expect(getPDConsentCollection).toHaveBeenCalledTimes(1);
  });

  // Пустая система (никого, кого закрывает гейт) - деление на ноль, а не «0%».
  it('нулевой знаменатель не даёт NaN', async () => {
    getPDConsentCollection.mockResolvedValue(
      collection({ total: 0, accepted: 0, pending: 0, pending_users: [] }),
    );
    const wrapper = await openSection();

    expect(wrapper.get('[data-testid="pdc-collection-counts"]').text()).toContain('100%');
  });
});
