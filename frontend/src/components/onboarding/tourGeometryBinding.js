import { createTourGeometry } from './tourGeometry';
import { findTourTarget } from './targetResolver';
import { applyStageShape, raiseActiveHighlight } from './stageShape';
import { createPopoverZoomFix } from './popoverZoom';
import { isTargetInView } from './targetWaits';

/** Host adapter: active layout never owns the navigation index or permissions. */
export function createTourGeometryBinding({ store, getDriver, getGeneration, isBusy, runTransition, waitForElement, ensureInView, onError = () => {} }) {
  let disposed = false;
  const zoom = createPopoverZoomFix({ requestRefresh: () => geometry.invalidate('popover-style') });
  const popover = () => getDriver()?.getState?.()?.popover?.wrapper || null;
  const scopeOf = (step, target) => (step?.dataReady ? findTourTarget(step.dataReady) : null)
    || target?.parentElement || document.body;
  const geometry = createTourGeometry({
    getDriver: () => isBusy() ? null : getDriver(),
    getPopover: popover,
    resolveTarget: ({ index, target }) => {
      const selector = store.steps[index]?.element;
      return selector ? findTourTarget(selector, { preferred: target }) : null;
    },
    scrollTarget: ensureInView,
    shouldScroll: target => !isTargetInView(target),
    onError,
    applyShape: applyStageShape,
    restoreHighlight: (driver, target) => {
      if (target && driver?.getActiveElement?.() === target) raiseActiveHighlight(driver);
    },
    correctPopover: wrapper => zoom.apply(wrapper),
    requestRetarget: request => retarget(request),
  });

  function stop() { geometry.stop(); zoom.detach(); }

  function commit(index) {
    const driver = getDriver(), step = store.steps[index];
    if (disposed || !store.isActive || isBusy() || !driver || !step
      || driver.obGetGlobalIndex() !== index) return;
    const generation = getGeneration(), tour = store.activeTourKey, stepId = step.id;
    const active = driver.getActiveElement?.();
    const target = active?.id === 'driver-dummy-element' ? null : (active || null);
    zoom.attach(popover());
    geometry.commit({ target, scope: scopeOf(step, target), index, stepId,
      isCurrent: () => !disposed && store.isActive && !isBusy() && getDriver() === driver
        && getGeneration() === generation && store.activeTourKey === tour
        && store.currentIndex === index && store.steps[index]?.id === stepId });
  }

  function retarget({ index, stepId, target }, wait = false) {
    const driver = getDriver(), generation = getGeneration(), tour = store.activeTourKey;
    const valid = () => !disposed && store.isActive && getDriver() === driver
      && getGeneration() === generation && store.activeTourKey === tour
      && store.currentIndex === index && store.steps[index]?.id === stepId;
    if (!driver || isBusy() || !valid()) return;
    let signal;
    const pending = runTransition(async operation => {
      signal = operation.signal;
      stop();
      const step = store.steps[index];
      if (wait && step.element) {
        await waitForElement(step.waitFor || step.element, 4000, operation.signal);
        if (!operation.isCurrent() || !valid()) return;
        target = findTourTarget(step.element);
      }
      if (target) {
        const prepared = await geometry.prepare({ target, scope: scopeOf(step, target), index, stepId, scrollTo: step.scrollTo }, operation);
        if (!prepared) return;
        target = prepared;
      }
      if (!operation.isCurrent() || !valid()) return;
      driver.obGoTo(index, !target);
    }, { intent: { tourKey: tour, stepId, direction: 'retarget' } });
    return pending?.catch(error => {
      if (!signal?.aborted && valid()) onError(error);
    });
  }

  function refresh(index) {
    const step = store.steps[index];
    if (step?.element) return retarget({ index, stepId: step.id, target: null }, true);
  }

  function prepare(target, scrollTo, operation) {
    return geometry.prepare({ target, scrollTo }, operation);
  }

  function dispose() { disposed = true; geometry.dispose(); zoom.detach(); }
  return { prepare, commit, refresh, stop, dispose };
}
