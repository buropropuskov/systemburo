import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import BlacklistOverrideModal from '../BlacklistOverrideModal.vue';

function mountModal(props = {}) {
  return mount(BlacklistOverrideModal, {
    props: {
      show: true,
      flag: { flag_id: 7, matched_value: 'А124ВС Toyota', matched_reason: 'похожий номер' },
      ...props,
    },
    global: { stubs: { teleport: true } },
  });
}

describe('BlacklistOverrideModal (#481, срез 6a)', () => {
  it('показывает совпавшую запись ЧС и причину', () => {
    const wrapper = mountModal();
    expect(wrapper.find('.override-matched__value').text()).toBe('А124ВС Toyota');
    expect(wrapper.find('.override-matched__reason').text()).toContain('похожий номер');
  });

  it('кнопка подтверждения заблокирована при пустом комментарии', () => {
    const wrapper = mountModal();
    const confirm = wrapper.find('.lk-button--primary');
    expect(confirm.attributes('disabled')).toBeDefined();
  });

  it('после ввода причины кнопка активна и эмитит confirm с обрезанным текстом', async () => {
    const wrapper = mountModal();
    await wrapper.find('.lk-textarea').setValue('  проверено по СТС  ');
    const confirm = wrapper.find('.lk-button--primary');
    expect(confirm.attributes('disabled')).toBeUndefined();

    await confirm.trigger('click');
    expect(wrapper.emitted('confirm')).toHaveLength(1);
    expect(wrapper.emitted('confirm')[0][0]).toBe('проверено по СТС');
  });

  it('не эмитит confirm, если комментарий только из пробелов', async () => {
    const wrapper = mountModal();
    await wrapper.find('.lk-textarea').setValue('    ');
    await wrapper.find('.lk-button--primary').trigger('click');
    expect(wrapper.emitted('confirm')).toBeUndefined();
  });

  it('submitting=true блокирует кнопку даже с заполненным комментарием', async () => {
    const wrapper = mountModal({ submitting: true });
    await wrapper.find('.lk-textarea').setValue('причина');
    expect(wrapper.find('.lk-button--primary').attributes('disabled')).toBeDefined();
  });

  it('кнопка "Отмена" эмитит close', async () => {
    const wrapper = mountModal();
    await wrapper.find('.lk-button--ghost').trigger('click');
    expect(wrapper.emitted('close')).toHaveLength(1);
  });

  it('повторное открытие сбрасывает прежний комментарий', async () => {
    const wrapper = mountModal({ show: false });
    await wrapper.setProps({ show: true });
    await wrapper.find('.lk-textarea').setValue('старое');
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    expect(wrapper.find('.lk-textarea').element.value).toBe('');
  });
});
