import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import BlankSelector from '../BlankSelector.vue';

// Форма может открыть вложение мимо клика по чипу (восстановление черновика после F5,
// удаление выбранного). Подсветка идёт за пропсом activeAttachment, иначе форма
// заполнена, а в списке слева визуально ничего не выбрано.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({
    ok: true,
    json: vi.fn().mockResolvedValue([
      { id: 1, title: 'Автомобили', name: 'cars', display_name: 'Автомобили', attachment_type: 'cars' },
    ]),
  }),
}));

beforeEach(() => {
  setActivePinia(createPinia());
});

const ATTACHMENTS = [
  { local_id: 'a1', title: 'Автомобили', name: 'cars_1', display_name: 'Автомобили №1', attachment_type: 'cars' },
  { local_id: 'a2', title: 'Автомобили', name: 'cars_2', display_name: 'Автомобили №2', attachment_type: 'cars' },
];

describe('BlankSelector - подсветка следует за выбором родителя', () => {
  it('вложение, выбранное родителем при монтировании, подсвечено', async () => {
    const w = shallowMount(BlankSelector, {
      props: { attachments: ATTACHMENTS, activeAttachment: ATTACHMENTS[0] },
    });
    await flushPromises();

    expect(w.vm.isSelected(ATTACHMENTS[0])).toBe(true);
    expect(w.vm.isSelected(ATTACHMENTS[1])).toBe(false);
  });

  it('список приехал после выбора родителя - подсветка появляется', async () => {
    const w = shallowMount(BlankSelector, {
      props: { attachments: [], activeAttachment: ATTACHMENTS[0] },
    });
    await flushPromises();
    expect(w.vm.selectedAttachment).toBeNull();

    await w.setProps({ attachments: ATTACHMENTS });
    await flushPromises();

    expect(w.vm.isSelected(ATTACHMENTS[0])).toBe(true);
  });

  it('смена выбора родителем переносит подсветку', async () => {
    const w = shallowMount(BlankSelector, {
      props: { attachments: ATTACHMENTS, activeAttachment: ATTACHMENTS[0] },
    });
    await flushPromises();

    await w.setProps({ activeAttachment: ATTACHMENTS[1] });
    expect(w.vm.isSelected(ATTACHMENTS[1])).toBe(true);
    expect(w.vm.isSelected(ATTACHMENTS[0])).toBe(false);
  });

  it('родитель сбросил выбор - подсветка снимается', async () => {
    const w = shallowMount(BlankSelector, {
      props: { attachments: ATTACHMENTS, activeAttachment: ATTACHMENTS[0] },
    });
    await flushPromises();

    await w.setProps({ activeAttachment: null });
    expect(w.vm.selectedAttachment).toBeNull();
  });

  it('клик по чипу подсвечивает и эмитит наверх', async () => {
    const w = shallowMount(BlankSelector, {
      props: { attachments: ATTACHMENTS, activeAttachment: null },
    });
    await flushPromises();

    w.vm.selectAttachment(ATTACHMENTS[1]);
    expect(w.vm.isSelected(ATTACHMENTS[1])).toBe(true);
    expect(w.emitted('attachment-selected')[0][0]).toMatchObject({ local_id: 'a2' });
  });
});
