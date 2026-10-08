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
  recipients: ['Иванов Иван Иванович', 'Сидоров Сидор Сидорович'],
  recipient_details_available: true,
  recipient_details: [
    { user_id: 6, display_name: 'Тестовый согласующий', purpose: 'approval', required_approval: true, access_granted: true },
    { user_id: 7, display_name: 'Тестовый наблюдатель', purpose: 'view', required_approval: false, access_granted: true },
  ],
  whole: true,
  attachments: [],
  created_at: '2026-07-01T10:00:00Z',
};

describe('ForwardMessages (#967)', () => {
  beforeEach(() => {
    getForwardMessages.mockReset();
  });

  it('рендерит пересылку с автором, получателями и текстом', async () => {
    getForwardMessages.mockResolvedValue([MSG]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();

    expect(getForwardMessages).toHaveBeenCalledWith(42);
    const items = wrapper.findAll('[data-testid="forward-message-item"]');
    expect(items).toHaveLength(1);
    expect(wrapper.text()).toContain('Петров Пётр Петрович');
    expect(wrapper.text()).toContain('Прошу дополнительно согласовать заявку с вами');
    expect(wrapper.text()).toContain('Тестовый согласующий — назначен обязательным согласующим');
    expect(wrapper.text()).toContain('Тестовый наблюдатель — предоставлен доступ к просмотру');
    expect(wrapper.text()).not.toContain(MSG.recipients[0]);
    expect(wrapper.text()).toContain('Переслал(-а) заявку');
    expect(wrapper.text()).toContain('Материалы: заявка со всеми приложениями');
  });

  it('пересылка вложений показывает действие с их перечнем', async () => {
    getForwardMessages.mockResolvedValue([
      { id: 3, author_id: 5, author_name: 'Петров Пётр Петрович', message: '', recipients: ['Кузнецов Кузьма'], whole: false, attachments: ['Пропуск №12', 'Акт'], created_at: '2026-07-01T12:00:00Z' },
    ]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();

    expect(wrapper.text()).toContain('Переслал(-а) заявку');
    expect(wrapper.text()).toContain('только выбранные приложения — Пропуск №12, Акт');
    expect(wrapper.text()).toContain('Сведения о получателях недоступны');
    expect(wrapper.text()).not.toContain('Кузнецов Кузьма');
  });

  it('пересылка без текста показывает действие и кому, но не текст', async () => {
    getForwardMessages.mockResolvedValue([
      { id: 2, author_id: 5, author_name: 'Петров Пётр Петрович', message: '', recipients: ['Кузнецов Кузьма'], whole: true, attachments: [], created_at: '2026-07-01T11:00:00Z' },
    ]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();

    expect(wrapper.findAll('[data-testid="forward-message-item"]')).toHaveLength(1);
    expect(wrapper.text()).toContain('Петров Пётр Петрович');
    expect(wrapper.text()).toContain('Переслал(-а) заявку');
    expect(wrapper.text()).toContain('Сведения о получателях недоступны');
    expect(wrapper.text()).not.toContain('Кузнецов Кузьма');
    expect(wrapper.find('.forward-message-text').exists()).toBe(false);
  });

  it('скрывает блок при пустом списке', async () => {
    getForwardMessages.mockResolvedValue([]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();

    expect(wrapper.find('[data-testid="forward-messages"]').exists()).toBe(false);
  });

  it('показывает комментарий и безопасное имя как текст без HTML', async () => {
    const markup = '<img src=x onerror=alert(1)>';
    getForwardMessages.mockResolvedValue([{ ...MSG, message: markup, recipient_details: [{ ...MSG.recipient_details[0], display_name: markup }] }]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();
    expect(wrapper.find('.forward-message-text').text()).toBe(markup);
    expect(wrapper.find('.forward-message-recipient').text()).toContain(markup);
    expect(wrapper.find('img').exists()).toBe(false);
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

describe('ForwardMessages — сворачивание', () => {
  beforeEach(() => {
    getForwardMessages.mockReset();
    localStorage.clear();
  });

  it('по умолчанию свёрнут', async () => {
    getForwardMessages.mockResolvedValue([MSG]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();

    expect(wrapper.find('[data-testid="forward-messages"]').classes()).toContain('collapsed');
    expect(wrapper.find('[data-testid="forward-messages-toggle"]').attributes('aria-expanded')).toBe('false');
  });

  it('клик по заголовку разворачивает и сохраняет выбор', async () => {
    getForwardMessages.mockResolvedValue([MSG]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();

    await wrapper.find('[data-testid="forward-messages-toggle"]').trigger('click');

    expect(wrapper.find('[data-testid="forward-messages"]').classes()).not.toContain('collapsed');
    expect(wrapper.find('[data-testid="forward-messages-toggle"]').attributes('aria-expanded')).toBe('true');
    expect(localStorage.getItem('forwardThread.collapsed')).toBe('false');
  });

  it('уважает сохранённое развёрнутое состояние', async () => {
    localStorage.setItem('forwardThread.collapsed', 'false');
    getForwardMessages.mockResolvedValue([MSG]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();

    expect(wrapper.find('[data-testid="forward-messages"]').classes()).not.toContain('collapsed');
  });

  it('повторный клик снова сворачивает', async () => {
    getForwardMessages.mockResolvedValue([MSG]);
    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();
    const toggle = wrapper.find('[data-testid="forward-messages-toggle"]');

    await toggle.trigger('click'); // развернуть
    await toggle.trigger('click'); // снова свернуть

    expect(wrapper.find('[data-testid="forward-messages"]').classes()).toContain('collapsed');
    expect(toggle.attributes('aria-expanded')).toBe('false');
    expect(localStorage.getItem('forwardThread.collapsed')).toBe('true');
  });

  it('не падает, если localStorage недоступен', async () => {
    const getSpy = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('denied'); });
    const setSpy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('denied'); });
    getForwardMessages.mockResolvedValue([MSG]);

    const wrapper = mount(ForwardMessages, { props: { applicationId: 42 } });
    await flushPromises();
    // дефолт свёрнут при недоступном localStorage
    expect(wrapper.find('[data-testid="forward-messages"]').classes()).toContain('collapsed');
    // клик не роняет компонент
    await wrapper.find('[data-testid="forward-messages-toggle"]').trigger('click');
    expect(wrapper.find('[data-testid="forward-messages"]').classes()).not.toContain('collapsed');

    getSpy.mockRestore();
    setSpy.mockRestore();
  });
});
