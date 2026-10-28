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
const styles = [];
const create = (props = {}) => { const w = mount(ThemeControls, { props, attachTo: document.body }); wrappers.push(w); return w; };
beforeEach(() => { localStorage.clear(); setActivePinia(createPinia()); vi.clearAllMocks(); });
afterEach(() => {
  wrappers.splice(0).forEach(w => w.unmount());
  styles.splice(0).forEach(style => style.remove());
  document.body.innerHTML = '';
});
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

it('places settings above the mobile navigation layer', () => {
  const w = create();
  const layer = w.findComponent({ name: 'BaseDropdown' }).props('menuZIndex');
  expect(layer).toBeGreaterThan(10000);
  expect(layer).toBeLessThan(11000);
});

it('keeps mobile settings inside navigation and closes when switching layout', async () => {
  const w = create({ inline: true });
  await w.get('button').trigger('click');
  expect(w.find('.theme-options').exists()).toBe(true);
  expect(w.findComponent({ name: 'BaseDropdown' }).props('teleport')).toBe(false);
  await w.setProps({ inline: false });
  expect(document.querySelector('.theme-options')).toBeNull();
  await w.get('button').trigger('click');
  expect(w.find('.theme-options').exists()).toBe(false);
  expect(document.querySelector('.theme-options')).not.toBeNull();
});
it('updates the disclosure arrow with the setting state', async () => {
  const w = create({ inline: true });
  expect(w.get('.theme-arrow').classes()).not.toContain('theme-arrow--open');
  await w.get('button').trigger('click');
  expect(w.get('.theme-arrow').classes()).toContain('theme-arrow--open');
  expect(w.get('button').attributes('aria-expanded')).toBe('true');
  await w.get('button').trigger('click');
  expect(w.get('.theme-arrow').classes()).not.toContain('theme-arrow--open');
});

it.each([
  { inline: false, closed: 'rotate(0deg)', opened: 'rotate(180deg)' },
  { inline: true, closed: 'rotate(90deg)', opened: 'rotate(270deg)' },
])('points toward the actual panel layout (inline=$inline)', async ({ inline, closed, opened }) => {
  const source = fs.readFileSync(resolve(__dirname, '../ThemeControls.vue'), 'utf8');
  const compiled = compileStyle({ source: parse(source).descriptor.styles[0].content,
    filename: 'ThemeControls.vue', id: ThemeControls.__scopeId, scoped: true });
  const style = document.createElement('style'); style.textContent = compiled.code;
  styles.push(style); document.head.append(style);
  const wrapper = create({ inline });
  const arrow = () => wrapper.get('.theme-arrow').element;
  expect(getComputedStyle(arrow()).transform).toBe(closed);
  await wrapper.get('button').trigger('click');
  expect(getComputedStyle(arrow()).transform).toBe(opened);
  await wrapper.get('button').trigger('click');
  expect(getComputedStyle(arrow()).transform).toBe(closed);
});
