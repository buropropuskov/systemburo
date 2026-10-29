import { useRevealFirstApplication } from './useRevealFirstApplication';
import { useOnboardingStore } from '@/stores/onboarding';
import { DEMO_CENTER_APPLICATION_ID } from '@/components/onboarding/demoCenterApplication';

/** Only this view's tour-owned fictional card can be remounted by a scene. */
export function useCenterTourReveal(view) {
  const store = useOnboardingStore();
  const reveal = useRevealFirstApplication({
    list: () => store.isActive && ['accept', 'approve'].includes(store.activeTourKey)
      ? (view.applications || view.sortedApplications).filter(item => item.id === DEMO_CENTER_APPLICATION_ID) : view.sortedApplications,
    reload: () => view.fetchApplications(true),
    isOpen: () => !!view.selectedApplication, current: () => view.selectedApplication,
    open: (application, operation) => {
      const revision = store.centerDemoRevision, identity = store.contextIdentity(), wasActive = store.isActive;
      return view.openApplication(application, { isCurrent: () => (!wasActive || store.isActive)
        && (application.id !== DEMO_CENTER_APPLICATION_ID || store.isActive)
        && revision === store.centerDemoRevision && identity === store.contextIdentity()
        && (!operation || operation.isCurrent()) });
    }, close: () => view.closeDetail(),
  });
  const detach = store.registerCenterDemoOwner({
    canReplace: () => reveal.canReplaceDemo(DEMO_CENTER_APPLICATION_ID),
    replace: operation => reveal.replaceDemo(DEMO_CENTER_APPLICATION_ID, operation),
  });
  return { ...reveal, stop() { detach(); reveal.stop(); } };
}
