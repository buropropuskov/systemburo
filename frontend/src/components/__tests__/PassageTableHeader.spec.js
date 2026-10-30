import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { defineComponent, h, onMounted, ref } from 'vue';
import { flushPromises, mount } from '@vue/test-utils';
import PassageTableHeader from '../PassageTableHeader.vue';
import RefreshButton from '../RefreshButton.vue';

const observers = [], wrappers = [];
let mountedCommands, sizes;
const Tools = defineComponent({
  name: 'PassageTableTools', emits: ['available', 'clock', 'refresh'],
  setup(_, { emit }) { onMounted(() => emit('available', false)); return () => null; },
});
const Commands = defineComponent({
  setup() { const value = ref(0); mountedCommands++; return () => h('button', { onClick: () => value.value++ }, `Режим ${value.value}`); },
});
const create = (props = {}) => {
  const w = mount(PassageTableHeader, {
    attachTo: document.body,
    props: { title: 'Номера автомобилей по заявке', compactTitle: 'По заявке', kind: 'car', hasSettings: true, ...props },
    slots: { title: '<h3>Номера автомобилей по заявке</h3>', counter: '<span>На территории: 2</span>',
      commands: () => h(Commands), extra: '<span data-testid="extra">Поиск в версии</span>' },
    global: { stubs: { PassageTableTools: Tools } },
  });
  w.element.style.cssText = 'gap:12px;padding:8px 20px';
  wrappers.push(w); return w;
};
async function resize(w, width) {
  Object.defineProperty(w.element, 'clientWidth', { configurable: true, value: width });
  observers.filter(o => o.target === w.element).forEach(o => o.callback([]));
  await flushPromises();
}
describe('internal passage table header', () => {
  beforeEach(() => {
    mountedCommands = 0;
    sizes = { title: 300, counter: 150, extra: 150, commands: 500 };
    vi.spyOn(HTMLElement.prototype, 'scrollWidth', 'get').mockImplementation(function () {
      return this.classList.contains('passage-table-header__full-title') ? sizes.title : 0;
    });
    vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockImplementation(function () {
      if (this.classList.contains('passage-table-header__counter')) return sizes.counter;
      if (this.classList.contains('passage-table-header__refresh')) return 100;
      if (this.classList.contains('passage-table-commands')) return sizes.commands;
      return this.dataset.testid === 'extra' ? sizes.extra : 0;
    });
    vi.stubGlobal('ResizeObserver', class {
      constructor(callback) { this.callback = callback; this.disconnected = false; observers.push(this); }
      observe(target) { this.target = target; }
      disconnect() { this.disconnected = true; }
    });
  });
  afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); observers.splice(0); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
  it('fits the short fact composition below 1300 rather than compacting every card', async () => {
    sizes = { title: 180, counter: 0, extra: 0, commands: 180 };
    const w = create({ hasSettings: false });
    await resize(w, 560);
    expect(w.classes()).not.toContain('passage-table-header--compact');
    w.findComponent(Tools).vm.$emit('available', true); await flushPromises();
    expect(w.classes()).not.toContain('passage-table-header--compact');
    await resize(w, 490);
    expect(w.classes()).toContain('passage-table-header--compact');
  });
  it('remeasures changed commands at the same width and preserves an already open menu when still compact', async () => {
    const w = create(); await resize(w, 900);
    await w.get('button[aria-label="Действия таблицы"]').trigger('click');
    const dropdown = w.find('.passage-table-header__menu');
    const button = w.findComponent(Commands).get('button').element;
    button.focus();
    sizes.commands = 600;
    await w.setProps({ title: 'Новое название' }); await flushPromises();
    expect(dropdown.attributes('data-dropdown-open')).toBe('true');
    expect(document.activeElement).toBe(button);
    expect(mountedCommands).toBe(1);
    sizes.commands = 0;
    await w.setProps({ hasSettings: false }); await flushPromises();
    expect(w.classes()).not.toContain('passage-table-header--compact');
  });
  it('ignores height-only ResizeObserver notifications without changing popup state', async () => {
    const w = create(); await resize(w, 768);
    await w.get('button[aria-label="Действия таблицы"]').trigger('click');
    const observer = observers.find(o => o.target === w.element);
    observer.callback([{ contentRect: { width: 768, height: 99 } }]); await flushPromises();
    expect(w.find('.passage-table-header__menu').attributes('data-dropdown-open')).toBe('true');
    expect(mountedCommands).toBe(1);
  });
  it('uses the table width even on a wide viewport and keeps counter/refresh outside the menu', async () => {
    vi.stubGlobal('innerWidth', 2496);
    const w = create(); await resize(w, 390);
    expect(w.classes()).toContain('passage-table-header--compact');
    expect(w.find('.passage-table-header__counter').text()).toBe('На территории: 2');
    expect(w.findComponent(RefreshButton).element.closest('.base-dropdown')).toBeNull();
    expect(w.get('button[aria-label="Действия таблицы"]').attributes('aria-expanded')).toBe('false');
  });
  it('keeps one action instance and its state through repeated compact/inline transitions', async () => {
    const w = create(); await resize(w, 1500);
    const instance = w.findComponent(Commands).vm;
    await w.findComponent(Commands).find('button').trigger('click');
    for (const width of [768, 1500, 1024, 1500]) {
      await resize(w, width);
      expect(w.findComponent(Commands).vm).toBe(instance);
      expect(w.findComponent(Commands).text()).toBe('Режим 1');
    }
    expect(mountedCommands).toBe(1);
  });
  it('hides an empty menu based on the tools own availability without unmounting their clock', async () => {
    const w = create({ hasSettings: false }); await resize(w, 768);
    const tools = w.findComponent(Tools), instance = tools.vm;
    expect(w.find('.passage-table-header__menu').classes()).toContain('passage-table-header__menu--empty');
    tools.vm.$emit('available', true); await flushPromises();
    await w.get('button[aria-label="Действия таблицы"]').trigger('click');
    tools.vm.$emit('available', false); await flushPromises();
    expect(w.find('.passage-table-header__menu').attributes('data-dropdown-open')).toBe('false');
    expect(w.findComponent(Tools).vm).toBe(instance);
    tools.vm.$emit('clock', 123); expect(w.emitted('clock')).toEqual([[123]]);
  });
  it('forwards manual refresh and a clock boundary separately and only once', async () => {
    const w = create(); await flushPromises();
    w.findComponent(Tools).vm.$emit('refresh');
    w.findComponent(RefreshButton).vm.$emit('refresh');
    expect(w.emitted('boundary-refresh')).toHaveLength(1);
    expect(w.emitted('refresh')).toHaveLength(1);
  });
  it('moves keyboard focus to the visible menu trigger when inline commands collapse', async () => {
    const w = create(); await resize(w, 1500);
    w.findComponent(Commands).get('button').element.focus();
    await resize(w, 768);
    expect(document.activeElement).toBe(w.get('button[aria-label="Действия таблицы"]').element);
  });
  it('returns keyboard focus to the visible trigger after Escape', async () => {
    const w = create(); await resize(w, 768);
    await w.get('button[aria-label="Действия таблицы"]').trigger('click');
    w.findComponent(Commands).get('button').element.focus();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await flushPromises();
    expect(document.activeElement).toBe(w.get('button[aria-label="Действия таблицы"]').element);
  });
  it('moves focus from the disappearing trigger to inline commands on expansion', async () => {
    const w = create(); await resize(w, 768);
    w.get('button[aria-label="Действия таблицы"]').element.focus();
    await resize(w, 1500);
    expect(document.activeElement).toBe(w.findComponent(Commands).get('button').element);
  });
  it('preserves preview actions without mounting live tools or refresh', async () => {
    const w = create({ preview: true }); await resize(w, 390);
    expect(w.findComponent(Tools).exists()).toBe(false);
    expect(w.findComponent(RefreshButton).exists()).toBe(false);
    expect(w.find('[data-testid="extra"]').text()).toBe('Поиск в версии');
    expect(w.classes()).not.toContain('passage-table-header--compact');
  });
  it('disconnects the observer when the table leaves the page', async () => {
    const w = create(); await resize(w, 768);
    const observer = observers.find(o => o.target === w.element);
    w.unmount(); wrappers.splice(wrappers.indexOf(w), 1);
    expect(observer.disconnected).toBe(true);
  });
});
