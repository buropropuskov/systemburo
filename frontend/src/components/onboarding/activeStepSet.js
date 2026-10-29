import { computed, ref, watch } from 'vue';
import { buildTourSteps } from './tours';
import { isStepApplicable } from './stepApplicability';
import { resolveStepIndex } from './tourProgress';

export function applicableTourSteps(tour, context) {
  return buildTourSteps(tour, context).filter(step => isStepApplicable(step, context));
}

/** One producer publishes a fully rebased learning set after each Vue permission batch. */
export function createActiveStepSet({
  activeTour, tourContext, accessReady, isActive, currentIndex, currentStepId, skippedIndexes, saveProgress,
}) {
  const revision = ref(0);
  let observedIds = [], observedKey = '', observedReady = false;
  const keyFor = items => JSON.stringify(items.map(step =>
    [step.id, step.route, step.element, step.waitFor, step.dataReady, step.dataOwner, step.dataPrerequisite, step.dataAvailability, step.reveal?.prepare,
      step.demo, step.demoScene, step.section, step.sectionTitle, step.title, step.description]));
  // Availability follows real entry/action gates; unknown access is not a denial.
  // Keep the last known active set until the permission source becomes known again.
  const steps = computed(previous => {
    if (!activeTour.value) return [];
    if (!accessReady.value) return previous || [];
    return applicableTourSteps(activeTour.value, tourContext.value);
  });

  function capture() {
    observedIds = steps.value.map(step => step.id);
    observedKey = keyFor(steps.value);
    observedReady = accessReady.value;
  }
  watch([steps, accessReady], ([next, ready]) => {
    if (!isActive.value) return;
    const key = keyFor(next);
    if (ready === observedReady && key === observedKey) return;
    if (ready) {
      const skippedIds = skippedIndexes.value.map(index => observedIds[index]);
      currentIndex.value = resolveStepIndex(currentStepId.value, observedIds, next, currentIndex.value);
      currentStepId.value = next[currentIndex.value]?.id || null;
      skippedIndexes.value = skippedIds.map(id => next.findIndex(step => step.id === id)).filter(index => index >= 0);
      saveProgress(next);
    }
    capture();
    // Host sync-subscribers may now cancel/rebuild from one consistent snapshot.
    revision.value++;
  }, { flush: 'pre' });

  function reset() { observedIds = []; observedKey = ''; observedReady = false; }
  return { steps, revision, capture, reset };
}
