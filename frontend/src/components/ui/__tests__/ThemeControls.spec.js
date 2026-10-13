import fs from 'node:fs';
import { resolve } from 'node:path';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { parse, compileStyle } from '@vue/compiler-sfc';
import ThemeControls from '../ThemeControls.vue';
import { useThemeStore } from '@/stores/theme';
import { saveTheme } from '@/api/theme';
vi.mock('@/api/theme', () => ({ getTheme: vi.fn(), saveTheme: vi.fn().mockResolvedValue({}) }));
const wrappers = [];
const create = (props = {}) => { const w = mount(ThemeControls, { props, attachTo: document.body }); wrappers.push(w); return w; };
beforeEach(() => { localStorage.clear(); setActivePinia(createPinia()); vi.clearAllMocks(); });
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); document.body.innerHTML = ''; });
async function change(id, checked) {
  const input = document.querySelector(`[data-testid="${id}"] input`);
  expect(input).not.toBeNull();
  input.checked = checked;
  input.dispatchEvent(new Event('change', { bubbles: true }));
  await flushPromises();
}

describe('single dropdown theme setting', () => {
  it.each(['light', 'dark', 'glass-light', 'glass-dark'])('preserves both axes from %s', async (id) => {
    localStorage.setItem('app-theme', id);
    const wrapper = create();
    const theme = useThemeStore();
    const dark = id.endsWith('dark');
    const alternative = id.startsWith('glass-');
    expect(wrapper.findAll('button')).toHaveLength(1);
    expect(wrapper.get('button').text()).toBe('Оформление');
    await wrapper.get('[data-testid="nav-theme-toggle"]').trigger('click');
    await change('theme-alternative-toggle', !alternative);
    const next = (alternative ? '' : 'glass-') + (dark ? 'dark' : 'light');
    expect(theme.current).toBe(next);
    expect(saveTheme).toHaveBeenLastCalledWith(next);
    expect(document.querySelector('.base-dropdown__menu')).not.toBeNull();
    await change('theme-dark-toggle', !dark);
    expect(theme.current).toBe((alternative ? '' : 'glass-') + (dark ? 'light' : 'dark'));
    expect(document.querySelector('[data-testid="theme-dark-toggle"] input').checked).toBe(!dark);
  });
  it('closes on Escape and outside click', async () => {
    const w = create();
    await w.get('button').trigger('click');
    expect(document.querySelector('.base-dropdown__menu')).not.toBeNull();
    expect(document.activeElement).toBe(document.querySelector('[data-testid="theme-alternative-toggle"] input'));
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await flushPromises();
    expect(document.querySelector('.base-dropdown__menu')).toBeNull();
    expect(document.activeElement).toBe(w.get('button').element);
    await w.get('button').trigger('click');
    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await flushPromises();
    expect(document.querySelector('.base-dropdown__menu')).toBeNull();
  });
  it('compiles the expanded selector for the label, not just its ancestor', () => {
    const source = fs.readFileSync(resolve(__dirname, '../ThemeControls.vue'), 'utf8');
    const style = parse(source).descriptor.styles[0];
    const css = compileStyle({ source: style.content, filename: 'ThemeControls.vue', id: 'data-v-theme-test', scoped: true }).code;
    expect(css).toContain('.nav-menu.expanded .nav-text[data-v-theme-test]');
  });
  it('disables the trigger and prevents a blocked user changing theme', async () => {
    const w = create({ disabled: true });
    const store = useThemeStore();
    const initial = store.current;
    expect(w.get('button').attributes('disabled')).toBeDefined();
    await w.get('button').trigger('click');
    await w.get('button').trigger('keydown', { key: 'Enter' });
    await flushPromises();
    expect(document.querySelector('.base-dropdown__menu')).toBeNull();
    expect(store.current).toBe(initial);
    expect(saveTheme).not.toHaveBeenCalled();
  });
  it('closes an already open setting if navigation becomes disabled', async () => {
    const w = create();
    await w.get('button').trigger('click');
    expect(document.querySelector('.base-dropdown__menu')).not.toBeNull();
    await w.setProps({ disabled: true });
    expect(document.querySelector('.base-dropdown__menu')).toBeNull();
    expect(saveTheme).not.toHaveBeenCalled();
  });
});
