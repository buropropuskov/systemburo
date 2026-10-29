import { ref } from 'vue';
import { syncDemoBackend } from './demoBackend';
import { createCenterDemoSession } from './centerDemoSession';

/** One owner for form examples and center scenes; no progress or access policy. */
export function createTourDemoData({ getTourKey, getActorId }) {
  const demoAttachmentType = ref(null);
  const center = createCenterDemoSession({ install: snapshot => snapshot
    ? syncDemoBackend(true, false, getTourKey(), { snapshot }) : syncDemoBackend(false) });
  const isCenter = key => ['accept', 'approve'].includes(key);
  function canStart(key) { return !isCenter(key) || (Number.isSafeInteger(getActorId()) && getActorId() > 0); }
  function start(key, hasOwnApplication) {
    if (isCenter(key)) center.begin({ actorId: getActorId(), tourKey: key, now: Date.now() });
    else syncDemoBackend(true, hasOwnApplication, key);
  }
  function stop() { center.stop(); }
  return { start, stop, canStart, api: { demoAttachmentType,
    setDemoAttachment(type) { demoAttachmentType.value = type || null; },
    centerDemoRevision: center.revision, prepareCenterDemo: center.prepare, registerCenterDemoOwner: center.registerOwner } };
}
