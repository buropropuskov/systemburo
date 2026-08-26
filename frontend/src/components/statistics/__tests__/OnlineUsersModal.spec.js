import { describe, it, expect, vi, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import OnlineUsersModal from '../OnlineUsersModal.vue';

// teleport: true оставляет контент модалки внутри wrapper, иначе он улетает в body.
const mountModal = (props = {}) =>
  mount(OnlineUsersModal, {
    props: { show: true, ...props },
    global: { stubs: { teleport: true, LoaderSpinner: true } },
  });

const USERS = [
  { id: 1, login: 'ivanov', full_name: 'Иванов Иван Иванович', role: 'Руководитель', user_type: 'Арендатор', last_seen: new Date('2026-06-20T11:58:00Z').toISOString() },
  { id: 2, login: 'petrov', full_name: '', role: '', user_type: 'Охранник', last_seen: new Date('2026-06-20T11:30:00Z').toISOString() },
];

describe('OnlineUsersModal', () => {
  afterEach(() => vi.useRealTimers());

  it('рендерит строку на каждого пользователя: ФИО, роль/тип, @логин', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-06-20T12:00:00Z'));

    const wrapper = mountModal({ users: USERS });
    const rows = wrapper.findAll('.ou-row');
    expect(rows).toHaveLength(2);

    expect(rows[0].text()).toContain('Иванов Иван Иванович');
    expect(rows[0].text()).toContain('Руководитель');
    expect(rows[0].text()).toContain('@ivanov');
    expect(rows[0].text()).toContain('2 мин назад');

    // нет ФИО -> заголовок = логин; нет роли -> показывается тип
    expect(rows[1].text()).toContain('petrov');
    expect(rows[1].text()).toContain('Охранник');
    expect(rows[1].text()).toContain('30 мин назад');

    // счётчик в заголовке
    expect(wrapper.find('.ou-modal__count').text()).toContain('2');
  });

  it('пустой список -> "Сейчас никого онлайн", строк нет', () => {
    const wrapper = mountModal({ users: [] });
    expect(wrapper.findAll('.ou-row')).toHaveLength(0);
    expect(wrapper.find('.ou-modal__state').text()).toBe('Сейчас никого онлайн');
  });

  it('ошибка -> текст ошибки, без счётчика', () => {
    const wrapper = mountModal({ users: [], error: 'Не удалось загрузить список' });
    expect(wrapper.find('.ou-modal__state--error').text()).toBe('Не удалось загрузить список');
    expect(wrapper.find('.ou-modal__count').exists()).toBe(false);
  });

  it('loading -> спиннер вместо списка', () => {
    const wrapper = mountModal({ loading: true });
    expect(wrapper.findComponent({ name: 'LoaderSpinner' }).exists() || wrapper.find('loaderspinner-stub').exists()).toBe(true);
    expect(wrapper.findAll('.ou-row')).toHaveLength(0);
  });

  it('show=false -> ничего не рендерит', () => {
    const wrapper = mountModal({ show: false, users: USERS });
    expect(wrapper.find('.ou-modal').exists()).toBe(false);
  });

  it('кнопка закрытия эмитит close', async () => {
    const wrapper = mountModal({ users: [] });
    await wrapper.find('.ou-modal__close').trigger('click');
    expect(wrapper.emitted('close')).toHaveLength(1);
  });

  it('Escape эмитит close', async () => {
    const wrapper = mountModal({ users: [] });
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await wrapper.vm.$nextTick();
    expect(wrapper.emitted('close')).toHaveLength(1);
  });
});
