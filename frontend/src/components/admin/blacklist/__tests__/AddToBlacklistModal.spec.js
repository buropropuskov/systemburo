import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import AddToBlacklistModal from '../AddToBlacklistModal.vue';

// BaseModal рендерит через Teleport - стабим, чтобы контент был в обёртке для запросов.
const stubs = {
  BaseModal: { template: '<div class="base-modal"><slot /><slot name="actions" /></div>' },
  FormField: { template: '<div class="form-field"><slot /></div>' },
};

function mountModal(props = {}) {
  return mount(AddToBlacklistModal, {
    props: { show: true, type: 'vehicle', entityLabel: 'А777АА777 BMW', ...props },
    global: { stubs },
  });
}

describe('AddToBlacklistModal', () => {
  it('заголовок и подпись зависят от типа', () => {
    const veh = mountModal({ type: 'vehicle' });
    expect(veh.vm.title).toBe('Добавить машину в чёрный список');
    expect(veh.vm.entityCaption).toBe('Машина');
    const per = mountModal({ type: 'person', entityLabel: 'Иванов Иван' });
    expect(per.vm.title).toBe('Добавить человека в чёрный список');
    expect(per.vm.entityCaption).toBe('Человек');
  });

  it('показывает entityLabel', () => {
    const wrapper = mountModal({ entityLabel: 'А777АА777 BMW' });
    expect(wrapper.text()).toContain('А777АА777 BMW');
  });

  it('кнопка добавления заблокирована без причины', async () => {
    const wrapper = mountModal();
    const addBtn = wrapper.findAll('button').find((b) => b.text().includes('Добавить'));
    expect(addBtn.attributes('disabled')).toBeDefined();
    await wrapper.find('textarea').setValue('угон');
    expect(addBtn.attributes('disabled')).toBeUndefined();
  });

  it('confirm эмитит trimmed причину', async () => {
    const wrapper = mountModal();
    await wrapper.find('textarea').setValue('  нарушение режима  ');
    await wrapper.findAll('button').find((b) => b.text().includes('Добавить')).trigger('click');
    expect(wrapper.emitted('confirm')).toBeTruthy();
    expect(wrapper.emitted('confirm')[0]).toEqual(['нарушение режима']);
  });

  it('не эмитит confirm и close при saving', async () => {
    const wrapper = mountModal({ saving: true });
    await wrapper.find('textarea').setValue('причина');
    wrapper.vm.confirm();
    wrapper.vm.close();
    expect(wrapper.emitted('confirm')).toBeFalsy();
    expect(wrapper.emitted('close')).toBeFalsy();
  });

  it('сбрасывает причину при повторном открытии', async () => {
    const wrapper = mountModal({ show: false });
    await wrapper.setProps({ show: true });
    wrapper.vm.reason = 'старое';
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    expect(wrapper.vm.reason).toBe('');
  });
});
