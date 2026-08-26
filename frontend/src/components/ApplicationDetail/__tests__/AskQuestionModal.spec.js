import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import AskQuestionModal from '../AskQuestionModal.vue';

const ATTACHMENTS = [
  { id: 1, attachment_display_name: 'Автозаявка №1' },
  { id: 2, attachment_display_name: 'Проведение работ' },
];

// BaseModal не рендерит контент при show=false; reset() срабатывает на false->true.
async function mountOpened(props = {}) {
  const wrapper = mount(AskQuestionModal, {
    props: { show: false, attachments: ATTACHMENTS, ...props },
    global: { stubs: { teleport: true } },
  });
  await wrapper.setProps({ show: true });
  return wrapper;
}

describe('AskQuestionModal (#973)', () => {
  it('по умолчанию вложения не выбраны', async () => {
    const wrapper = await mountOpened();
    expect(wrapper.vm.selectedAttachmentIds).toEqual([]);
  });

  it('canSend требует и тему, и текст', async () => {
    const wrapper = await mountOpened();
    expect(wrapper.vm.canSend).toBe(false);
    await wrapper.find('[data-testid="ask-modal-subject"]').setValue('Тема');
    expect(wrapper.vm.canSend).toBe(false);
    await wrapper.find('[data-testid="ask-modal-text"]').setValue('Вопрос');
    expect(wrapper.vm.canSend).toBe(true);
  });

  it('«Выбрать все» переключает вложения', async () => {
    const wrapper = await mountOpened();
    const all = wrapper.find('[data-testid="ask-modal-attachments-all"]');
    await all.setValue(true);
    expect(wrapper.vm.selectedAttachmentIds).toEqual([1, 2]);
    await all.setValue(false);
    expect(wrapper.vm.selectedAttachmentIds).toEqual([]);
  });

  it('send эмитит {subject, text, attachment_ids} с обрезкой', async () => {
    const wrapper = await mountOpened();
    await wrapper.find('[data-testid="ask-modal-subject"]').setValue('  Прицеп  ');
    await wrapper.find('[data-testid="ask-modal-text"]').setValue('  Есть прицеп?  ');
    await wrapper.find('[data-testid="ask-modal-attachment"] input[type="checkbox"]').setValue(true);
    await wrapper.find('[data-testid="ask-modal-send"]').trigger('click');

    const ev = wrapper.emitted('send');
    expect(ev).toHaveLength(1);
    expect(ev[0][0]).toEqual({ subject: 'Прицеп', text: 'Есть прицеп?', attachment_ids: [1] });
  });

  it('повторное открытие сбрасывает поля', async () => {
    const wrapper = await mountOpened();
    await wrapper.find('[data-testid="ask-modal-subject"]').setValue('черновик');
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    expect(wrapper.vm.subject).toBe('');
    expect(wrapper.vm.selectedAttachmentIds).toEqual([]);
  });
});
