import { nextTick } from 'vue';

// DOM-связь меню с его владельцем сохраняется и после Teleport в body.
const portals = new Map();
export function registerFocusPortal(owner, getPortal) { portals.set(owner, getPortal); }
export function releaseFocusPortal(owner) { portals.delete(owner); }

const selector = 'a[href],area[href],button,input,select,textarea,iframe,summary,[tabindex],[contenteditable="true"],audio[controls],video[controls]';
function rendered(node) {
  if (!node?.isConnected || node.closest('[hidden],[inert]') || !node.getClientRects().length) return false;
  return !['hidden', 'collapse'].includes(getComputedStyle(node).visibility);
}

function ownedRoots(root) {
  const roots = [root];
  for (let index = 0; index < roots.length; index++) {
    for (const [owner, getPortal] of portals) {
      if (!roots[index]?.contains(owner) || !rendered(owner)) continue;
      const portal = getPortal();
      if (portal && !roots.includes(portal)) roots.push(portal);
    }
  }
  return roots;
}

function owns(root, element) {
  return !!root && ownedRoots(root).some(node => node?.contains(element));
}

function tabbable(root) {
  const found = [], visited = new Set();
  function visit(node) {
    if (!node || visited.has(node)) return;
    visited.add(node);
    if (node.matches(selector) && !node.matches(':disabled') && rendered(node)
      && (node.tabIndex >= 0 || (node.isContentEditable && !node.hasAttribute('tabindex')))) found.push(node);
    for (const child of node.children) visit(child);
    const portal = portals.get(node)?.();
    if (portal && rendered(node)) visit(portal);
  }
  visit(root);
  // В группе radio Tab останавливается на выбранном либо первом доступном.
  const items = found.filter(node => {
    if (node.type !== 'radio' || !node.name) return true;
    const group = found.filter(other => other.type === 'radio' && other.name === node.name && other.form === node.form);
    return node === (group.find(other => other.checked) || group[0]);
  });
  return items.filter(node => node.tabIndex > 0).sort((a, b) => a.tabIndex - b.tabIndex)
    .concat(items.filter(node => node.tabIndex <= 0));
}

export function createDialogFocus(getRoot, isTop) {
  let trigger = null, opened = false, version = 0;
  return {
    open() {
      if (opened || typeof document === 'undefined') return;
      opened = true;
      trigger = document.activeElement;
      const current = ++version;
      nextTick(() => {
        if (!opened || current !== version || !isTop()) return;
        const root = getRoot();
        // Контейнер не вызывает клавиатуру телефона и не прокручивает длинную форму.
        if (root && !owns(root, document.activeElement)) root.focus({ preventScroll: true });
      });
    },
    close() {
      const active = opened && isTop();
      opened = false;
      version++;
      const target = trigger;
      trigger = null;
      if (active && owns(getRoot(), document.activeElement) && rendered(target) && !target.matches(':disabled')) {
        target.focus({ preventScroll: true });
      }
    },
    handleTab(event) {
      if (event.key !== 'Tab' || event.defaultPrevented || !opened || !isTop()) return;
      const root = getRoot();
      if (!root) return;
      const items = tabbable(root), index = items.indexOf(document.activeElement);
      const next = index < 0 ? (event.shiftKey ? items.length - 1 : 0)
        : (index + (event.shiftKey ? -1 : 1) + items.length) % items.length;
      event.preventDefault();
      // Tab должен показывать нижние поля длинной формы, как нативный переход.
      if (items[next]) items[next].focus();
      else root.focus({ preventScroll: true });
    },
  };
}
