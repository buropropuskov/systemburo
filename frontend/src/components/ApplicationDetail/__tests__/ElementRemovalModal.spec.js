import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import ElementRemovalModal from '../ElementRemovalModal.vue';

function mountModal(props = {}) {
  return mount(ElementRemovalModal, {
    props: { show: true, label: 'Машина А123ВС', ...props },
    global: { stubs: { Teleport: true } },
  });
}

describe('ElementRemovalModal', () => {
  it('без причины подтвердить нельзя: кнопка заблокирована и события нет', async () => {
    const wrapper = mountModal();
    const confirm = wrapper.find('[data-testid="removal-confirm"]');

    expect(confirm.attributes('disabled')).toBeDefined();
    await confirm.trigger('click');
    expect(wrapper.emitted('confirm')).toBeUndefined();
  });

  it('пробелы за причину не считаются', async () => {
    const wrapper = mountModal();
    await wrapper.find('[data-testid="removal-reason"]').setValue('   ');

    expect(wrapper.find('[data-testid="removal-confirm"]').attributes('disabled')).toBeDefined();
  });

  it('с причиной эмитит confirm с обрезанным текстом', async () => {
    const wrapper = mountModal();
    await wrapper.find('[data-testid="removal-reason"]').setValue('  числится в розыске  ');
    await wrapper.find('[data-testid="removal-confirm"]').trigger('click');

    expect(wrapper.emitted('confirm')).toEqual([['числится в розыске']]);
  });

  it('причина не переносится на следующую строку при повторном открытии', async () => {
    const wrapper = mountModal({ show: false });
    await wrapper.setProps({ show: true });
    await wrapper.find('[data-testid="removal-reason"]').setValue('первая причина');

    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });

    expect(wrapper.find('[data-testid="removal-reason"]').element.value).toBe('');
  });

  it('подпись убираемой строки видна в окне', () => {
    const wrapper = mountModal({ label: 'Иванов Иван Иванович' });
    expect(wrapper.find('[data-testid="removal-target"]').text()).toBe('Иванов Иван Иванович');
  });
});
