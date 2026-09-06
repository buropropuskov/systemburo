import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import EmployeeObjectionControl from '../EmployeeObjectionControl.vue';

const setObjection = vi.fn().mockResolvedValue(undefined);
const clearObjection = vi.fn().mockResolvedValue(undefined);
vi.mock('@/api/employees', () => ({
  setEmployeeObjection: (...args) => setObjection(...args),
  clearEmployeeObjection: (...args) => clearObjection(...args),
}));

function mountControl(props = {}) {
  return mount(EmployeeObjectionControl, {
    props: { employeeId: 42, ...props },
  });
}

describe('EmployeeObjectionControl — управление возражением субъекта', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    setObjection.mockClear();
    clearObjection.mockClear();
  });

  it('без основания отметку поставить нельзя: через полгода она была бы неотличима от случайного нажатия', async () => {
    const wrapper = mountControl();
    const button = wrapper.find('[data-testid="objection-set"]');
    expect(button.attributes('disabled')).toBeDefined();

    await wrapper.find('[data-testid="objection-source-input"]').setValue('   ');
    expect(button.attributes('disabled')).toBeDefined();

    await wrapper.find('[data-testid="objection-source-input"]').setValue('письмо в бюро');
    expect(button.attributes('disabled')).toBeUndefined();
  });

  it('отметка уходит с основанием и сообщает вью, что состояние изменилось', async () => {
    const wrapper = mountControl();
    await wrapper.find('[data-testid="objection-source-input"]').setValue('  звонок в бюро  ');
    await wrapper.find('[data-testid="objection-set"]').trigger('click');
    await Promise.resolve();

    expect(setObjection).toHaveBeenCalledWith(42, 'звонок в бюро');
    expect(wrapper.emitted('changed')).toBeTruthy();
  });

  it('при возражении показывает состояние и основание вместо формы', () => {
    const wrapper = mountControl({ objectedAt: '2026-09-06T10:00:00Z', source: 'письмо на почту' });

    expect(wrapper.find('[data-testid="objection-note"]').text()).toContain('возразил против обработки');
    expect(wrapper.find('[data-testid="objection-source"]').text()).toContain('письмо на почту');
    expect(wrapper.find('[data-testid="objection-source-input"]').exists()).toBe(false);
  });

  it('снять отметку может только администратор: остальным вместо кнопки объяснение', () => {
    const applicant = mountControl({ objectedAt: '2026-09-06T10:00:00Z' });
    expect(applicant.find('[data-testid="objection-clear"]').exists()).toBe(false);
    expect(applicant.text()).toContain('администратор бюро');

    const admin = mountControl({ objectedAt: '2026-09-06T10:00:00Z', canManageAll: true });
    expect(admin.find('[data-testid="objection-clear"]').exists()).toBe(true);
  });
});
