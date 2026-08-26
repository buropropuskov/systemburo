import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import BlankSelector from '../BlankSelector.vue';

// #883: inline-переименование вложения. Имя редактируется в чипе, эмитится наверх
// (attachment-renamed) и оттуда уходит в форму/payload/поиск.

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
];

async function mountSelector(attachments = ATTACHMENTS) {
  const w = shallowMount(BlankSelector, { props: { attachments } });
  await flushPromises();
  return w;
}

describe('BlankSelector - inline-переименование вложения (#883)', () => {
  it('commitRename эмитит attachment-renamed с новым именем и выходит из режима', async () => {
    const w = await mountSelector();
    w.vm.startRename(ATTACHMENTS[0]);
    expect(w.vm.editingKey).toBe('a1');
    w.vm.editingName = 'Грузовики';
    w.vm.commitRename(ATTACHMENTS[0]);

    const ev = w.emitted('attachment-renamed');
    expect(ev).toBeTruthy();
    expect(ev[0][0]).toMatchObject({ display_name: 'Грузовики' });
    expect(ev[0][0].attachment.local_id).toBe('a1');
    expect(w.vm.editingKey).toBe(null);
  });

  it('пустое имя не эмитит и откатывает редактирование', async () => {
    const w = await mountSelector();
    w.vm.startRename(ATTACHMENTS[0]);
    w.vm.editingName = '   ';
    w.vm.commitRename(ATTACHMENTS[0]);
    expect(w.emitted('attachment-renamed')).toBeFalsy();
    expect(w.vm.editingKey).toBe(null);
  });

  it('имя без изменений не эмитит', async () => {
    const w = await mountSelector();
    w.vm.startRename(ATTACHMENTS[0]);
    w.vm.editingName = 'Автомобили №1';
    w.vm.commitRename(ATTACHMENTS[0]);
    expect(w.emitted('attachment-renamed')).toBeFalsy();
  });

  it('cancelRename выходит без эмита', async () => {
    const w = await mountSelector();
    w.vm.startRename(ATTACHMENTS[0]);
    w.vm.editingName = 'Другое имя';
    w.vm.cancelRename();
    expect(w.emitted('attachment-renamed')).toBeFalsy();
    expect(w.vm.editingKey).toBe(null);
  });

  it('blur после Enter не дублирует эмит', async () => {
    const w = await mountSelector();
    w.vm.startRename(ATTACHMENTS[0]);
    w.vm.editingName = 'Грузовики';
    w.vm.commitRename(ATTACHMENTS[0]); // Enter
    w.vm.commitRename(ATTACHMENTS[0]); // blur по удалению инпута
    expect(w.emitted('attachment-renamed').length).toBe(1);
  });

  it('удаление редактируемого вложения сбрасывает режим (watch)', async () => {
    const w = await mountSelector();
    w.vm.startRename(ATTACHMENTS[0]);
    expect(w.vm.editingKey).toBe('a1');
    await w.setProps({ attachments: [] });
    expect(w.vm.editingKey).toBe(null);
  });
});
