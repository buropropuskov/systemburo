import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

import PassageRevertModal from '../PassageRevertModal.vue';
import { usePassageRevertStore } from '@/stores/passageRevert';
import { useDeletionsStore } from '@/stores/deletions';

function mountModal() {
  return mount(PassageRevertModal, {
    global: { stubs: { teleport: true, transition: false } },
  });
}

function ask(store, overrides = {}) {
  store.ask({
    kind: 'employees',
    id: 5,
    direction: 'entry',
    tableId: 42,
    subject: 'Роголев Иван',
    onDone: vi.fn(),
    ...overrides,
  });
}

describe('PassageRevertModal (#2437)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
  });

  it('без причины отменить нельзя: кнопка недоступна', async () => {
    const wrapper = mountModal();
    ask(usePassageRevertStore());
    await flushPromises();

    expect(wrapper.find('[data-testid="passage-revert-submit"]').attributes('disabled')).toBeDefined();

    await wrapper.find('[data-testid="passage-revert-reason"]').setValue('   ');
    expect(wrapper.find('[data-testid="passage-revert-submit"]').attributes('disabled')).toBeDefined();
  });

  it('готовая причина подставляется и уходит в запрос, окно закрывается', async () => {
    apiRequest.mockResolvedValue({ ok: true, json: async () => ({}) });
    const wrapper = mountModal();
    const store = usePassageRevertStore();
    const onDone = vi.fn();
    ask(store, { onDone });
    await flushPromises();

    await wrapper.findAll('.revert__reason')[0].trigger('click');
    await wrapper.find('[data-testid="passage-revert-submit"]').trigger('click');
    await flushPromises();

    const [url, options] = apiRequest.mock.calls[0];
    expect(url).toBe('/employees/5/territory-status/revert');
    expect(JSON.parse(options.body)).toMatchObject({ territory_status: 1, table_id: 42 });
    expect(JSON.parse(options.body).reason.trim()).not.toBe('');
    expect(onDone).toHaveBeenCalled();
    expect(store.request).toBeNull();
  });

  it('отказ бэка показывается человеку и окно остаётся открытым', async () => {
    apiRequest.mockResolvedValue({
      ok: false, status: 403, json: async () => ({ error: 'Отметку поставил другой пользователь' }),
    });
    const wrapper = mountModal();
    const store = usePassageRevertStore();
    const onDone = vi.fn();
    ask(store, { onDone });
    await flushPromises();

    await wrapper.find('[data-testid="passage-revert-reason"]').setValue('ошибся строкой');
    await wrapper.find('[data-testid="passage-revert-submit"]').trigger('click');
    await flushPromises();

    // Окно не закрываем: человек должен увидеть отказ и решить, что делать дальше.
    expect(store.request).not.toBeNull();
    expect(onDone).not.toHaveBeenCalled();
    expect(JSON.stringify(useDeletionsStore().items)).toContain('Отметку поставил другой пользователь');
  });

  it('новый запрос начинается с пустой причины, а не с прошлой', async () => {
    const wrapper = mountModal();
    const store = usePassageRevertStore();
    ask(store);
    await flushPromises();

    await wrapper.find('[data-testid="passage-revert-reason"]').setValue('первая причина');
    store.close();
    await flushPromises();
    ask(store, { id: 6, subject: 'Шумилин Кирилл' });
    await flushPromises();

    expect(wrapper.find('[data-testid="passage-revert-reason"]').element.value).toBe('');
  });
});
