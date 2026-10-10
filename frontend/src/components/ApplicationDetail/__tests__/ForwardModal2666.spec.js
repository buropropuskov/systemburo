import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import ForwardModal from '../ForwardModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';

const users = [
  { id: 1, username: 'candidate_one', first_name: 'Тестовый', last_name: 'Первый', organization: 'Org A', company: 'Company A', position: 'Role A' },
  { id: 2, username: 'candidate_two', first_name: 'Тестовый', last_name: 'Второй', organization: 'Org A', company: 'Company B', position: 'Role B' },
  { id: 3, username: 'masked_candidate', first_name: null, last_name: null, middle_name: null, pd_hidden: true, organization: 'Org B', company: 'Company B', position: null },
];
const wrappers = [];
async function open(props = {}) {
  const wrapper = mount(ForwardModal, { props: { show: false, allUsers: users, attachments: [{ id: 8, attachment_display_name: 'Материал' }], ...props }, attachTo: document.body, global: { stubs: { teleport: true } } });
  wrappers.push(wrapper);
  await wrapper.setProps({ show: true });
  return wrapper;
}
afterEach(() => {
  wrappers.splice(0).forEach(wrapper => wrapper.unmount());
  vi.useRealTimers();
});

describe('Пересылка #2666', () => {
  it('Tab-переход фокуса на кандидата не удаляет его через 200мс; уход наружу закрывает список', async () => {
    vi.useFakeTimers();
    const wrapper = await open();
    const input = wrapper.find('[data-testid="forward-modal-search"]');
    // Реальный focus вызывает focus/blur/focusout с relatedTarget, как переход Tab.
    input.element.focus();
    await wrapper.vm.$nextTick();
    const first = wrapper.find('[data-testid="forward-modal-user-option"]');
    first.element.focus();
    expect(document.activeElement).toBe(first.element);
    await vi.advanceTimersByTimeAsync(201);
    expect(wrapper.vm.showDropdown).toBe(true);
    expect(wrapper.find('[data-testid="forward-modal-user-option"]').element).toBe(first.element);
    expect(document.activeElement).toBe(first.element);
    await first.trigger('keydown', { key: 'Enter' });
    expect(wrapper.vm.selectedUsers).toHaveLength(1);

    input.element.focus();
    await wrapper.vm.$nextTick();
    wrapper.find('[data-testid="forward-modal-user-option"]').element.focus();
    wrapper.find('[data-testid="forward-modal-button-cancel"]').element.focus();
    await vi.advanceTimersByTimeAsync(201);
    expect(wrapper.vm.showDropdown).toBe(false);
    expect(wrapper.find('[data-testid="forward-modal-user-option"]').exists()).toBe(false);
  });

  it('не фокусирует поиск и не раскрывает список при открытии; явный фокус открывает', async () => {
    const wrapper = await open();
    const input = wrapper.find('[data-testid="forward-modal-search"]');
    expect(document.activeElement).not.toBe(input.element);
    expect(wrapper.vm.showDropdown).toBe(false);
    expect(wrapper.find('[data-testid="forward-modal-user-option"]').exists()).toBe(false);
    await input.trigger('focus');
    expect(wrapper.findAll('[data-testid="forward-modal-user-option"]')).toHaveLength(3);
    const option = wrapper.find('[data-testid="forward-modal-user-option"]');
    expect(option.attributes('tabindex')).toBe('0');
    await option.trigger('keydown', { key: 'Enter' });
    expect(wrapper.vm.selectedUsers).toHaveLength(1);
  });

  it('использует три общих dropdown, сочетает фильтры с поиском и исключением доступных', async () => {
    const wrapper = await open({ existingViewers: [{ user_id: 3 }] });
    expect(wrapper.findAllComponents(BaseDropdown)).toHaveLength(3);
    await wrapper.setData({ filters: { organization: 'Org A', company: 'Company B', position: 'Role B' }, searchQuery: 'candidate_two' });
    expect(wrapper.vm.filteredUsers.map(user => user.id)).toEqual([2]);
    await wrapper.setData({ searchQuery: 'candidate_one' });
    expect(wrapper.vm.filteredUsers).toEqual([]);
    await wrapper.find('.reset-filters-btn').trigger('click');
    expect(wrapper.vm.filters).toEqual({ organization: '', company: '', position: '' });
    expect(wrapper.vm.searchQuery).toBe('candidate_one');
    expect(wrapper.vm.filteredUsers.map(user => user.id)).toEqual([1]);
  });

  it('не придумывает ФИО скрытому пользователю, сбрасывает фильтры при переоткрытии', async () => {
    const wrapper = await open();
    await wrapper.setData({ searchQuery: 'masked_candidate' });
    expect(wrapper.vm.filteredUsers.map(user => user.id)).toEqual([3]);
    expect(wrapper.vm.getUserDisplayName(users[2])).toBe('masked_candidate');
    await wrapper.setData({ filters: { organization: 'Org A', company: '', position: '' } });
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    expect(wrapper.vm.filters).toEqual({ organization: '', company: '', position: '' });
    expect(wrapper.vm.searchQuery).toBe('');
    expect(wrapper.vm.showDropdown).toBe(false);
  });

  it('сохраняет область выбора и предупреждения; блокирует отправку без материалов', async () => {
    const wrapper = await open();
    expect(wrapper.find('.selected-forward-users').exists()).toBe(true);
    expect(wrapper.find('.forward-empty').exists()).toBe(true);
    expect(wrapper.find('.forward-validation-slot').exists()).toBe(true);
    expect(wrapper.find('#forward-attachments-warning').exists()).toBe(false);
    wrapper.vm.addUser(users[0]);
    await wrapper.vm.$nextTick();
    await wrapper.find('[data-testid="forward-modal-attachments-all"]').setValue(false);
    const hint = wrapper.find('#forward-attachments-warning');
    expect(hint.attributes('role')).toBe('alert');
    expect(hint.isVisible()).toBe(true);
    expect(hint.text()).toBe('⚠ Выберите хотя бы одно вложение для пересылки');
    expect(wrapper.find('[data-testid="forward-modal-button-send"]').attributes('disabled')).toBeDefined();
    expect(wrapper.find('[data-testid="forward-modal-attachments"]').attributes('aria-describedby')).toBe('forward-attachments-warning');
  });

  it('отозванная всегда view-only даже с оставшимися флагами согласования', async () => {
    const wrapper = await open({ withdrawn: true, readerOnly: false });
    expect(wrapper.find('[data-testid="forward-modal-reader-note"]').text()).toBe('Заявка отозвана. Пересылка доступна только для просмотра; назначение согласующих недоступно.');
    await wrapper.setData({ selectedUsers: [{ ...users[0], requires_approval: true, required_approval: true }] });
    expect(wrapper.find('[data-testid="forward-modal-user-settings"]').exists()).toBe(false);
    await wrapper.find('[data-testid="forward-modal-button-send"]').trigger('click');
    expect(wrapper.emitted('send')[0][0].users).toEqual([{ user_id: 1, required_approval: false, can_view: true }]);
    expect(wrapper.find('[data-testid="forward-modal-warning"]').text()).not.toContain('(принимающие)');
  });
});
