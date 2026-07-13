import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';

// FE-S4 (#706): место разгрузки на уровне заявки. Единый выбор синхронизируется
// во все cars-вложения, уходит в items-DTO; для items-без-машин обязателен.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));

const PLACES = [
  { id: 1, name: 'Ворота 1', status: 'active' },
  { id: 2, name: 'Ворота 2', status: 'active' },
  { id: 3, name: 'Док 3', status: 'inactive' },
];

async function mountApp() {
  const w = shallowMount(CreateApplication);
  await flushPromises();
  w.vm.allUnloadingPlaces = PLACES;
  return w;
}

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
});

describe('CreateApplication - место разгрузки на уровне заявки (#706)', () => {
  it('onApplicationUnloadPlacesChange раскатывает выбор во все машины cars-вложений', async () => {
    const w = await mountApp();
    w.vm.attachments = [{ local_id: 'a1', attachment_type: 'cars' }];
    w.vm.vehiclesByAttachment = {
      a1: [
        { id: 1, unloadPlaces: [], unloadingPlace: '' },
        { id: 2, unloadPlaces: [], unloadingPlace: '' },
      ],
    };

    w.vm.onApplicationUnloadPlacesChange([1, 2]);

    expect(w.vm.applicationUnloadPlaces).toEqual([1, 2]);
    expect(w.vm.vehiclesByAttachment.a1[0].unloadPlaces).toEqual([1, 2]);
    expect(w.vm.vehiclesByAttachment.a1[1].unloadPlaces).toEqual([1, 2]);
    // Отображаемая строка машины - первое место + "и др." при нескольких.
    expect(w.vm.vehiclesByAttachment.a1[0].unloadingPlace).toBe('Ворота 1 и др.');
  });

  it('одно место - строка без "и др."', async () => {
    const w = await mountApp();
    w.vm.attachments = [{ local_id: 'a1', attachment_type: 'cars' }];
    w.vm.vehiclesByAttachment = { a1: [{ id: 1, unloadPlaces: [], unloadingPlace: '' }] };

    w.vm.onApplicationUnloadPlacesChange([2]);

    expect(w.vm.vehiclesByAttachment.a1[0].unloadingPlace).toBe('Ворота 2');
  });

  it('showItemsUnloadPlaces/itemsUnloadRequired: только items без машин', async () => {
    const w = await mountApp();

    w.vm.attachments = [{ local_id: 'i1', attachment_type: 'items' }];
    expect(w.vm.showItemsUnloadPlaces).toBe(true);
    expect(w.vm.itemsUnloadRequired).toBe(true);

    // Появилась машина - выбор мест переезжает в форму авто.
    w.vm.attachments = [
      { local_id: 'i1', attachment_type: 'items' },
      { local_id: 'c1', attachment_type: 'cars' },
    ];
    expect(w.vm.showItemsUnloadPlaces).toBe(false);
    expect(w.vm.itemsUnloadRequired).toBe(false);
  });

  it('#529: items-без-машин без места блокирует отправку (submitValidation + tooltipSections)', async () => {
    const w = await mountApp();
    w.vm.attachments = [{ local_id: 'i1', attachment_type: 'items', attachment_display_name: 'Разовый пропуск' }];
    w.vm.itemsByAttachment = { i1: [{ id: 1, itemName: 'Ящик', quantity: 1 }] };

    expect(w.vm.submitValidation.some(r => r.includes('место разгрузки'))).toBe(true);
    const itemsSection = w.vm.tooltipSections.find(s => s.attachmentType === 'items');
    expect(itemsSection.messages).toContain('Не выбрано место разгрузки');

    w.vm.onApplicationUnloadPlacesChange([1]);

    expect(w.vm.submitValidation.some(r => r.includes('место разгрузки'))).toBe(false);
    const itemsSectionAfter = w.vm.tooltipSections.find(s => s.attachmentType === 'items');
    expect(itemsSectionAfter ? itemsSectionAfter.messages : []).not.toContain('Не выбрано место разгрузки');
  });

  it('место не обязательно для items, когда в заявке есть машины', async () => {
    const w = await mountApp();
    w.vm.attachments = [
      { local_id: 'i1', attachment_type: 'items', attachment_display_name: 'ТМЦ' },
      { local_id: 'c1', attachment_type: 'cars', attachment_display_name: 'Авто' },
    ];
    w.vm.itemsByAttachment = { i1: [{ id: 1, itemName: 'Ящик', quantity: 1 }] };

    expect(w.vm.submitValidation.some(r => r.includes('место разгрузки'))).toBe(false);
  });

  it('applicationUnloadPlaces переживает save/restore localStorage', async () => {
    const w = await mountApp();
    w.vm.attachments = [{ local_id: 'i1', attachment_type: 'items' }];
    w.vm.applicationUnloadPlaces = [1, 2];
    w.vm.saveToLocalStorage();

    // Свежий монтаж: mounted() вызывает restoreFromLocalStorage.
    const w2 = await mountApp();
    expect(w2.vm.applicationUnloadPlaces).toEqual([1, 2]);
  });
});
