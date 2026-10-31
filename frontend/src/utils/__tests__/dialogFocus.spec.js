import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick } from 'vue';
import { createDialogFocus, registerFocusPortal, releaseFocusPortal } from '../dialogFocus';

const owners = [];
beforeEach(() => {
  // jsdom не рассчитывает геометрию. Только моделируем видимость DOM для этой серии.
  vi.spyOn(HTMLElement.prototype, 'getClientRects').mockImplementation(function () {
    for (let node = this; node; node = node.parentElement) {
      if (getComputedStyle(node).display === 'none') return [];
    }
    return [{}];
  });
});
afterEach(() => { owners.forEach(releaseFocusPortal); owners.length = 0; document.body.replaceChildren(); vi.restoreAllMocks(); });
function fixture(html = '<button id="first">Первый</button><input id="last">') {
  document.body.innerHTML = `<button id="trigger">Открыть</button><div id="root" tabindex="-1">${html}</div><button id="outside">Фон</button>`;
  const root = document.querySelector('#root'), trigger = document.querySelector('#trigger');
  trigger.focus();
  let top = true;
  const focus = createDialogFocus(() => root, () => top);
  const tab = (shiftKey = false, prevented = false) => {
    const event = new KeyboardEvent('keydown', { key: 'Tab', shiftKey, cancelable: true });
    if (prevented) event.preventDefault();
    focus.handleTab(event);
    return event;
  };
  return { root, trigger, focus, tab, setTop(value) { top = value; } };
}
function portal(owner, html) {
  const menu = document.createElement('div'); menu.innerHTML = html; document.body.append(menu);
  owners.push(owner); registerFocusPortal(owner, () => menu);
  return menu;
}

describe('dialog keyboard focus', () => {
  it('focuses the container without focusing an input or scrolling', async () => {
    const f = fixture(), spy = vi.spyOn(f.root, 'focus'); f.focus.open(); await nextTick();
    expect(document.activeElement).toBe(f.root);
    expect(spy).toHaveBeenCalledWith({ preventScroll: true });
  });
  it('cycles in both directions and returns to the caller', async () => {
    const f = fixture(); f.focus.open(); await nextTick();
    f.tab(); expect(document.activeElement.id).toBe('first');
    f.tab(); expect(document.activeElement.id).toBe('last');
    f.tab(); expect(document.activeElement.id).toBe('first');
    f.tab(true); expect(document.activeElement.id).toBe('last');
    f.focus.close(); expect(document.activeElement).toBe(f.trigger);
  });
  it('skips hidden, inert, disabled and negative-tabindex controls', async () => {
    const f = fixture('<button disabled>Нет</button><div hidden><button>Нет</button></div><div inert><button>Нет</button></div><div style="display:none"><button>Нет</button></div><button tabindex="-1">Нет</button><button id="yes">Да</button>');
    f.focus.open(); await nextTick(); f.tab(); expect(document.activeElement.id).toBe('yes');
    f.tab(); expect(document.activeElement.id).toBe('yes');
  });
  it('keeps an empty dialog focusable', async () => {
    const f = fixture('<p>Текст</p>'); f.focus.open(); await nextTick();
    expect(f.tab().defaultPrevented).toBe(true); expect(document.activeElement).toBe(f.root);
  });
  it('lets Tab scroll a newly focused control into view', async () => {
    const f = fixture(), spy = vi.spyOn(document.querySelector('#first'), 'focus');
    f.focus.open(); await nextTick(); f.tab();
    expect(spy).toHaveBeenCalledWith();
  });
  it('does not steal focus or Tab from a higher owner', async () => {
    const f = fixture(); f.setTop(false); f.focus.open(); await nextTick();
    expect(document.activeElement).toBe(f.trigger); expect(f.tab().defaultPrevented).toBe(false);
    document.querySelector('#outside').focus(); f.focus.close(); expect(document.activeElement.id).toBe('outside');
  });
  it('cancels a pending opening and ignores an already handled Tab', async () => {
    const f = fixture(); f.focus.open(); f.focus.close(); await nextTick(); expect(document.activeElement).toBe(f.trigger);
    f.focus.open(); await nextTick(); f.tab(false, true); expect(document.activeElement).toBe(f.root);
  });
  it('does not restore a removed trigger or steal external focus on close', async () => {
    const f = fixture(); f.focus.open(); await nextTick(); f.trigger.remove(); f.focus.close();
    expect(document.activeElement).toBe(f.root);
    f.focus.open(); await nextTick(); document.querySelector('#outside').focus(); f.focus.close();
    expect(document.activeElement.id).toBe('outside');
  });
  it('inserts a teleported menu after its owner, including nested menus', async () => {
    const f = fixture('<div id="owner"><button id="open">Меню</button></div><button id="after">После</button>');
    const menu = portal(document.querySelector('#owner'), '<input id="search"><div id="nested"><button id="nested-open">Ещё</button></div>');
    portal(menu.querySelector('#nested'), '<button id="nested-item">Команда</button>');
    f.focus.open(); await nextTick();
    for (const id of ['open', 'search', 'nested-open', 'nested-item', 'after', 'open']) { f.tab(); expect(document.activeElement.id).toBe(id); }
    menu.querySelector('#search').focus(); f.focus.close(); expect(document.activeElement).toBe(f.trigger);
  });
  it('excludes a released portal and does not duplicate inline content', async () => {
    const f = fixture('<div id="owner"><button id="first">Первый</button><div id="inline"><button id="last">Последний</button></div></div>');
    const owner = document.querySelector('#owner'); owners.push(owner); registerFocusPortal(owner, () => document.querySelector('#inline'));
    f.focus.open(); await nextTick(); f.tab(); f.tab(); f.tab(); expect(document.activeElement.id).toBe('first');
    const menu = portal(owner, '<input id="gone">'); releaseFocusPortal(owner); f.tab(); expect(document.activeElement.id).toBe('last');
    expect(menu.contains(document.activeElement)).toBe(false);
  });
  it('uses the checked radio and respects positive tabindex order', async () => {
    const f = fixture('<input type="radio" name="g" id="no"><input type="radio" name="g" checked id="radio"><button tabindex="2" id="two">2</button><button tabindex="1" id="one">1</button>');
    f.focus.open(); await nextTick();
    for (const id of ['one', 'two', 'radio', 'one']) { f.tab(); expect(document.activeElement.id).toBe(id); }
  });
});
