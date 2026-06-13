import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import AddToBlacklistModal from '../AddToBlacklistModal.vue';

// marks подгружаются в vehicle edit-режиме - мокаем, чтобы не ходить в сеть.
vi.mock('@/api/marks', () => ({ listMarks: () => Promise.resolve([{ id: 5, name: 'BMW' }]) }));

// BaseModal рендерит через Teleport - стабим, чтобы контент был в обёртке для запросов.
// VehicleNumberFormatInput сам грузит форматы по сети - стабим, carNumber выставляет watcher.
const stubs = {
  BaseModal: { template: '<div class="base-modal"><slot /><slot name="actions" /></div>' },
  FormField: { template: '<div class="form-field"><slot /></div>' },
  VehicleNumberFormatInput: true,
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

  describe('режим редактирования (mode=edit)', () => {
    it('заголовок и кнопка - для редактирования', () => {
      const wrapper = mountModal({ mode: 'edit', type: 'person' });
      expect(wrapper.vm.title).toBe('Редактировать человека');
      const btn = wrapper.findAll('button').find((b) => b.text().includes('Сохранить'));
      expect(btn).toBeTruthy();
    });

    it('предзаполняет причину и идентичность из initial* при открытии', async () => {
      const wrapper = mountModal({
        show: false, mode: 'edit', type: 'person', initialReason: 'прежняя причина',
        initialEntity: { last_name: 'Иванов', first_name: 'Иван', middle_name: 'Иванович' },
      });
      await wrapper.setProps({ show: true });
      expect(wrapper.vm.reason).toBe('прежняя причина');
      expect(wrapper.vm.lastName).toBe('Иванов');
      expect(wrapper.vm.firstName).toBe('Иван');
    });

    it('confirm (person) эмитит объект с ФИО и причиной', async () => {
      const wrapper = mountModal({
        show: false, mode: 'edit', type: 'person',
        initialEntity: { last_name: 'Иванов', first_name: 'Иван', middle_name: '' },
      });
      await wrapper.setProps({ show: true });
      await wrapper.find('textarea').setValue('новая причина');
      await wrapper.findAll('button').find((b) => b.text().includes('Сохранить')).trigger('click');
      expect(wrapper.emitted('confirm')[0]).toEqual([
        { last_name: 'Иванов', first_name: 'Иван', middle_name: '', reason: 'новая причина' },
      ]);
    });

    it('confirm (vehicle) эмитит объект с номером, маркой и причиной', async () => {
      const wrapper = mountModal({
        show: false, mode: 'edit', type: 'vehicle',
        initialEntity: { car_number: 'А777АА77', mark_id: 5 },
      });
      await wrapper.setProps({ show: true });
      await wrapper.find('textarea').setValue('угон');
      expect(wrapper.vm.markId).toBe(5);
      wrapper.vm.confirm();
      expect(wrapper.emitted('confirm')[0]).toEqual([
        { car_number: 'А777АА77', mark_id: 5, reason: 'угон' },
      ]);
    });
  });
});
