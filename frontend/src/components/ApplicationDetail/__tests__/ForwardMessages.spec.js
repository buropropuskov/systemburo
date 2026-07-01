import { describe, it, expect, vi, beforeEach } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';

vi.mock('@/api/applications', () => ({
  getForwardMessages: vi.fn(),
}));

import ForwardMessages from '../ForwardMessages.vue';
import { getForwardMessages } from '@/api/applications';

const MSG = {
  id: 1,
  author_id: 5,
  author_name: 'Петров Пётр Петрович',
  message: 'Прошу дополнительно согласовать заявку с вами',
  created_at: '2026-07-01T10:00:00Z',
};

describe('ForwardMessages (#967)', () => {
  beforeEach(() => {
    getForwardMessages.mockReset();
  });

  it('рендерит список сообщений с автором и текстом', async () => {
    getForwardMessages.mockResolvedValue([MSG]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();

    expect(getForwardMessages).toHaveBeenCalledWith(42);
    const items = wrapper.findAll('[data-testid="forward-message-item"]');
    expect(items).toHaveLength(1);
    expect(wrapper.text()).toContain('Петров Пётр Петрович');
    expect(wrapper.text()).toContain('Прошу дополнительно согласовать заявку с вами');
  });

  it('скрывает блок при пустом списке', async () => {
    getForwardMessages.mockResolvedValue([]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();

    expect(wrapper.find('[data-testid="forward-messages"]').exists()).toBe(false);
  });

  it('load() перезагружает сообщения', async () => {
    getForwardMessages.mockResolvedValue([]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();
    expect(wrapper.find('[data-testid="forward-messages"]').exists()).toBe(false);

    getForwardMessages.mockResolvedValue([MSG]);
    await wrapper.vm.load();
    await flushPromises();

    expect(wrapper.findAll('[data-testid="forward-message-item"]')).toHaveLength(1);
  });

  it('при сбое запроса не роняет компонент и сохраняет прежние сообщения', async () => {
    getForwardMessages.mockResolvedValue([MSG]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();
    expect(wrapper.findAll('[data-testid="forward-message-item"]')).toHaveLength(1);

    getForwardMessages.mockRejectedValue(new Error('network'));
    await wrapper.vm.load();
    await flushPromises();

    expect(wrapper.findAll('[data-testid="forward-message-item"]')).toHaveLength(1);
  });
});
