import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import BlacklistCreateModal from '../BlacklistCreateModal.vue';

// BaseModal использует Teleport - стабим, чтобы не тянуть портал в тест.
const stubs = {
  BaseModal: { template: '<div><slot /><slot name="actions" /></div>' },
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
  const fmt = [{ format: { id: 1, name: 'РФ' }, cells: [{ cell_type: 'letters', max_length: 1 }, { cell_type: 'numbers', max_length: 3 }] }];

  it('canSubmit false без формата, ячеек и марки', () => {
    const w = mountModal({ type: 'vehicle' });
    expect(w.vm.canSubmit).toBe(false);
  });

  it('canSubmit true когда формат, ячейки, марка и причина заполнены', async () => {
    const w = mountModal({ type: 'vehicle' });
    w.vm.formats = fmt;
    w.vm.selectedFormatId = 1;
    w.vm.numberParts = ['А', '123'];
    w.vm.markId = 5;
    w.vm.reason = 'причина';
    await flushPromises();
    expect(w.vm.canSubmit).toBe(true);
  });

  it('submit собирает car_number из ячеек и эмитит created с номером и маркой', async () => {
    const createFn = vi.fn().mockResolvedValue({});
    const w = mountModal({ type: 'vehicle', createFn });
    w.vm.formats = fmt;
    w.vm.selectedFormatId = 1;
    w.vm.marks = [{ id: 5, name: 'BMW' }];
    w.vm.numberParts = ['А', '123'];
    w.vm.markId = 5;
    w.vm.reason = 'причина';
    await w.vm.submit();
    expect(createFn).toHaveBeenCalledWith({ car_number: 'А 123', mark_id: 5, reason: 'причина' });
    expect(w.emitted('created')[0]).toEqual(['А 123 BMW']);
  });
});

describe('BlacklistCreateModal — окно последствий перед внесением', () => {
  function mountForImpact(createFn) {
    return mount(BlacklistCreateModal, {
      props: { show: true, type: 'person', createFn },
      global: { stubs },
    });
  }

  it('пока окно последствий открыто, запись не создаётся; подтверждение доводит её до конца', async () => {
    const createFn = vi.fn().mockResolvedValue({});
    const wrapper = mountForImpact(createFn);
    wrapper.vm.lastName = 'Иванов';
    wrapper.vm.firstName = 'Иван';
    wrapper.vm.reason = 'причина';
    wrapper.vm.impact = { matches: 2, tables: ['КПП №4'], rows: [] };
    wrapper.vm.showImpact = true;
    await flushPromises();

    expect(createFn).not.toHaveBeenCalled();

    wrapper.vm.confirmImpact();
    await flushPromises();

    expect(createFn).toHaveBeenCalledTimes(1);
    expect(wrapper.emitted('created')).toBeTruthy();
    expect(wrapper.vm.showImpact).toBe(false);
  });

  it('отказ в окне не вносит запись', async () => {
    const createFn = vi.fn().mockResolvedValue({});
    const wrapper = mountForImpact(createFn);
    wrapper.vm.showImpact = true;
    await flushPromises();

    wrapper.vm.showImpact = false;
    await flushPromises();

    expect(createFn).not.toHaveBeenCalled();
    expect(wrapper.emitted('created')).toBeUndefined();
  });
});
