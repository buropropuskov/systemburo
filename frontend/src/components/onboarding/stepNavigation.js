import { findTourTarget } from './targetResolver';

/** Подготовка и commit шага выполняются под одним владельцем перехода. */
export function createStepNavigation({
  driverObj, store, stepsForSegment, startIndex, lastLocal, navigate,
  onBeforeStep, onBoundaryNext, onBoundaryPrev, setStepMode, demoFallback,
}) {
  return {
    onNextClick() {
      return navigate(async (operation) => {
        const localIndex = driverObj.getActiveIndex() ?? 0;
        if (localIndex >= lastLocal && onBoundaryNext) {
          await onBoundaryNext(startIndex + localIndex);
          return;
        }
        let target = localIndex + 1;
        while (target <= lastLocal) {
          const targetId = stepsForSegment[target]?.id;
          const ready = onBeforeStep ? await onBeforeStep(startIndex + target, operation) : true;
          // Отмена запрещает не только движение, но и запись пропуска в store.
          if (!operation.isCurrent() || store.steps[startIndex + target]?.id !== targetId) return;
          if (ready !== false) {
            setStepMode(target, ready === demoFallback);
            break;
          }
          store.markSkipped(startIndex + target);
          target += 1;
        }
        if (target > lastLocal) {
          if (onBoundaryNext) await onBoundaryNext(startIndex + lastLocal);
          return;
        }
        if (target === localIndex + 1) driverObj.moveNext();
        else driverObj.moveTo(target);
      }, 'next');
    },
    onPrevClick() {
      return navigate(async (operation) => {
        const localIndex = driverObj.getActiveIndex() ?? 0;
        if (localIndex <= 0 && startIndex > 0 && onBoundaryPrev) {
          await onBoundaryPrev(startIndex);
          return;
        }
        // Назад следует пройденному маршруту, а не наличию целей в текущем DOM.
        const skipped = new Set(store.skippedIndexes);
        let target = localIndex - 1;
        while (target > 0 && skipped.has(startIndex + target)) target -= 1;
        const prev = stepsForSegment[target];
        if (onBeforeStep) {
          const ready = await onBeforeStep(startIndex + target, operation);
          if (!operation.isCurrent() || store.steps[startIndex + target]?.id !== prev?.id) return;
          setStepMode(target, ready === demoFallback || ready === false || (prev?.demo && !findTourTarget(prev.element)));
        }
        if (target === localIndex - 1) driverObj.movePrevious();
        else driverObj.moveTo(target);
      }, 'previous');
    },
  };
}
