import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import ThemeControls from '../ThemeControls.vue';
import { useThemeStore } from '@/stores/theme';
import { saveTheme } from '@/api/theme';
vi.mock('@/api/theme', () => ({ getTheme: vi.fn(), saveTheme: vi.fn().mockResolvedValue({}) }));
beforeEach(() => { localStorage.clear(); setActivePinia(createPinia()); vi.clearAllMocks(); });
describe('independent theme controls', () => {
  it.each(['light', 'dark', 'glass-light', 'glass-dark'])('preserves both axes from %s', async (id) => {
    localStorage.setItem('app-theme', id);
    const wrapper = mount(ThemeControls);
    const theme = useThemeStore();
    const dark = id.endsWith('dark');
    const alternative = id.startsWith('glass-');
    await wrapper.get('[data-testid="nav-style-toggle"]').trigger('click');
    await flushPromises();
    const next = (alternative ? '' : 'glass-') + (dark ? 'dark' : 'light');
    expect(theme.current).toBe(next);
    expect(saveTheme).toHaveBeenLastCalledWith(next);
    await wrapper.get('[data-testid="nav-theme-toggle"]').trigger('keydown', { key: ' ' });
    await flushPromises();
    expect(theme.current).toBe((alternative ? '' : 'glass-') + (dark ? 'light' : 'dark'));
    expect(wrapper.get('[data-testid="nav-theme-toggle"]').attributes('aria-checked')).toBe(String(!dark));
    wrapper.unmount();
  });
  it('has keyboard controls without nested focusable switches', () => {
    const wrapper = mount(ThemeControls);
    expect(wrapper.findAll('[role="switch"][tabindex="0"]')).toHaveLength(2);
    expect(wrapper.findAll('.nav-theme-switch[inert]')).toHaveLength(2);
    wrapper.unmount();
  });
});
