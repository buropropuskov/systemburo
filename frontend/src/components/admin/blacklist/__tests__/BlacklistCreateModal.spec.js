import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import BlacklistCreateModal from '../BlacklistCreateModal.vue';

// BaseModal использует Teleport - стабим. VehicleNumberFormatInput грузит форматы по сети -
// стабим, номер задаём через carNumber (его v-model).
const stubs = {
  BaseModal: { template: '<div><slot /><slot name="actions" /></div>' },
  VehicleNumberFormatInput: true,
};

function mountModal(props = {}) {
  return mount(BlacklistCreateModal, {
    props: { show: true, type: 'person', createFn: vi.fn().mockResolvedValue({}), ...props },
    global: { stubs },
  });
}

describe('BlacklistCreateModal (person)', () => {
  it('canSubmit требует фамилию, имя и причину', async () => {
    const wrapper = mountModal();
    expect(wrapper.vm.canSubmit).toBe(false);
    wrapper.vm.lastName = 'Иванов';
    wrapper.vm.firstName = 'Иван';
    wrapper.vm.reason = 'причина';
    await flushPromises();
    expect(wrapper.vm.canSubmit).toBe(true);
  });

  it('submit вызывает createFn с payload и эмитит created с ФИО', async () => {
    const createFn = vi.fn().mockResolvedValue({});
    const wrapper = mountModal({ createFn });
    wrapper.vm.lastName = 'Иванов';
    wrapper.vm.firstName = 'Иван';
    wrapper.vm.middleName = 'Иванович';
    wrapper.vm.reason = 'причина';
    await wrapper.vm.submit();
    expect(createFn).toHaveBeenCalledWith({
      last_name: 'Иванов', first_name: 'Иван', middle_name: 'Иванович', reason: 'причина',
    });
    expect(wrapper.emitted('created')[0]).toEqual(['Иванов Иван Иванович']);
  });

  it('ошибка createFn (409) показывается в formError, created не эмитится', async () => {
    const createFn = vi.fn().mockRejectedValue(new Error('Этот человек уже в чёрном списке'));
    const wrapper = mountModal({ createFn });
    wrapper.vm.lastName = 'Иванов';
    wrapper.vm.firstName = 'Иван';
    wrapper.vm.reason = 'x';
    await wrapper.vm.submit();
    await flushPromises();
    expect(wrapper.vm.formError).toBe('Этот человек уже в чёрном списке');
    expect(wrapper.emitted('created')).toBeFalsy();
  });

  it('title зависит от типа', () => {
    expect(mountModal({ type: 'person' }).vm.title).toContain('человека');
    expect(mountModal({ type: 'vehicle' }).vm.title).toContain('машину');
  });
});

describe('BlacklistCreateModal (vehicle)', () => {
  it('canSubmit false без номера и марки', () => {
    const w = mountModal({ type: 'vehicle' });
    expect(w.vm.canSubmit).toBe(false);
  });

  it('canSubmit true когда номер, марка и причина заполнены', async () => {
    const w = mountModal({ type: 'vehicle' });
    w.vm.carNumber = 'А 123';
    w.vm.markId = 5;
    w.vm.reason = 'причина';
    await flushPromises();
    expect(w.vm.canSubmit).toBe(true);
  });

  it('submit берёт car_number из v-model компонента и эмитит created с номером и маркой', async () => {
    const createFn = vi.fn().mockResolvedValue({});
    const w = mountModal({ type: 'vehicle', createFn });
    w.vm.marks = [{ id: 5, name: 'BMW' }];
    w.vm.carNumber = 'А 123';
    w.vm.markId = 5;
    w.vm.reason = 'причина';
    await w.vm.submit();
    expect(createFn).toHaveBeenCalledWith({ car_number: 'А 123', mark_id: 5, reason: 'причина' });
    expect(w.emitted('created')[0]).toEqual(['А 123 BMW']);
  });
});
