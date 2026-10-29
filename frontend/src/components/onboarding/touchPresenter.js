import { getViewportZoom } from '@/utils/viewportScale';
import { touchPresentationLayout } from './touchPresentationLayout';

/** One presentation lease per real Driver render. It never changes the step. */
export function createTouchPresenter({ getDriver, isAlive, isPositionReady = () => true }) {
  let current = null, frame = null, observer = null, listeners = [], expanded = false;
  function listen(node, event, callback, options) {
    node?.addEventListener(event, callback, options);
    listeners.push(() => node?.removeEventListener(event, callback, options));
  }
  function schedule() {
    if (frame !== null || !current || !isAlive()) return;
    frame = requestAnimationFrame(() => { frame = null; update(); });
  }
  function setVariable(node, name, value) {
    if (node.style.getPropertyValue(name) !== value) node.style.setProperty(name, value);
  }
  function update() {
    if (!current || !isAlive() || !current.popover.wrapper.isConnected) return;
    const { popover, target, details, demo } = current;
    const viewport = window.visualViewport;
    const bodyHeight = popover.description.getBoundingClientRect().height;
    const requiredChrome = Math.max(popover.wrapper.scrollHeight, popover.wrapper.getBoundingClientRect().height) - bodyHeight;
    const minimumHeight = Math.max(120, requiredChrome + Math.min(60, popover.description.scrollHeight));
    const layout = touchPresentationLayout({ width: viewport?.width || window.innerWidth,
      height: viewport?.height || window.innerHeight, left: viewport?.offsetLeft || 0, top: viewport?.offsetTop || 0,
      target: target()?.getBoundingClientRect(), minimumHeight,
      coarse: navigator.maxTouchPoints > 0 || window.matchMedia?.('(pointer: coarse)').matches });
    popover.wrapper.classList.toggle('ob-touch', layout.touch);
    popover.wrapper.classList.toggle('ob-touch--wide', Boolean(layout.wide));
    popover.wrapper.classList.toggle('ob-touch--expanded', layout.touch && expanded);
    const measured = popover.description.clientHeight > 0;
    const hiddenContent = popover.description.scrollHeight > popover.description.clientHeight + 1;
    details.hidden = !layout.touch || (!expanded && !demo && measured && !hiddenContent);
    if (demo) demo.hidden = !layout.touch || expanded;
    if (!layout.touch) {
      popover.wrapper.classList.remove('ob-touch--expanded');
      if (popover.wrapper.style.display === 'flex') popover.wrapper.style.display = 'block';
      return;
    }
    // Respect Driver's hidden phase. Only a currently visible wrapper becomes a flex column.
    if (popover.wrapper.style.display !== 'none') popover.wrapper.style.display = 'flex';
    const zoom = getViewportZoom();
    const before = popover.wrapper.getBoundingClientRect();
    setVariable(popover.wrapper, '--ob-touch-width', `${layout.width / zoom}px`);
    setVariable(popover.wrapper, '--ob-touch-gutter', `${layout.gutter / zoom}px`);
    setVariable(popover.wrapper, '--ob-touch-height', `${layout.maxHeight / zoom}px`);
    popover.wrapper.dataset.obSpace = layout.fits ? 'available' : 'constrained';
    const chrome = Math.max(0, before.height - popover.description.getBoundingClientRect().height);
    setVariable(popover.wrapper, '--ob-touch-body', `${Math.max(24, layout.maxHeight - chrome - 4) / zoom}px`);
    const after = popover.wrapper.getBoundingClientRect();
    if (isPositionReady() && (Math.abs(before.width - after.width) > .5 || Math.abs(before.height - after.height) > .5)) getDriver()?.refresh?.();
  }
  function detach() {
    if (frame !== null) cancelAnimationFrame(frame);
    frame = null; observer?.disconnect(); observer = null;
    listeners.forEach(remove => remove()); listeners = [];
    if (current) { current.details.remove(); current.demo?.remove(); }
    current = null; expanded = false;
  }
  function attach(popover, target) {
    detach();
    if (!isAlive() || !popover.description || !popover.footerButtons) return;
    const details = document.createElement('button');
    details.type = 'button'; details.className = 'lk-button lk-button--secondary ob-touch__details';
    details.textContent = 'Подробнее'; details.setAttribute('aria-expanded', 'false');
    details.dataset.testid = 'ob-touch-details';
    let demo = null;
    const figure = popover.description.querySelector('.ob-popover__demo');
    if (figure) { demo = document.createElement('div'); demo.className = 'ob-touch__demo'; demo.append(figure.cloneNode(true)); popover.wrapper.insertBefore(demo, popover.title); }
    current = { popover, target, details, demo };
    details.addEventListener('click', () => {
      if (!isAlive()) return;
      expanded = !expanded;
      details.textContent = expanded ? 'Свернуть' : 'Подробнее';
      details.setAttribute('aria-expanded', String(expanded));
      update(); if (isPositionReady()) getDriver()?.refresh?.();
    });
    popover.footerButtons.insertBefore(details, popover.footerButtons.firstChild);
    listen(window, 'resize', schedule); listen(window.visualViewport, 'resize', schedule);
    listen(window.visualViewport, 'scroll', schedule); listen(document, 'scroll', schedule, true);
    if (typeof ResizeObserver !== 'undefined') {
      observer = new ResizeObserver(schedule); observer.observe(popover.wrapper);
      const node = target(); if (node) observer.observe(node);
    }
    update(); schedule();
  }
  return { attach, update, detach };
}
