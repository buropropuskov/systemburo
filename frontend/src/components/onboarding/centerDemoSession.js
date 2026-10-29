import { ref } from 'vue';
import { buildDemoCenterScenario, DEMO_CENTER_SCENES } from './demoCenterScenarios';

/** Data lifetime only; navigation and cancellation still belong to the host. */
export function createCenterDemoSession({ install }) {
  const revision = ref(0);
  let run = null, owner = null, sequence = 0;
  function stop() { run = null; sequence++; revision.value++; install(null); }
  function begin({ actorId, tourKey, now }) {
    stop();
    if (!['accept', 'approve'].includes(tourKey)) return false;
    const scene = tourKey === 'approve' ? DEMO_CENTER_SCENES.REVIEWER_PENDING : DEMO_CENTER_SCENES.ACCEPT_READY;
    const snapshot = buildDemoCenterScenario({ scene, actorId, now });
    run = { actorId, tourKey, now, scene, snapshots: new Map([[scene, snapshot]]), preparedOwner: null };
    install(snapshot); return true;
  }
  async function prepare(scene, operation) {
    if (!run) return;
    const currentRun = run, currentOwner = owner, request = ++sequence;
    const current = () => run === currentRun && owner === currentOwner && request === sequence && operation.isCurrent();
    if (!current()) return;
    if (owner && !owner.canReplace()) throw new Error('Закрой открытую карточку и повтори шаг обучения.');
    if (currentRun.scene === scene && currentRun.preparedOwner === owner) return;
    let snapshot = currentRun.snapshots.get(scene);
    if (!snapshot) {
      snapshot = buildDemoCenterScenario({ scene, actorId: currentRun.actorId, now: currentRun.now });
      currentRun.snapshots.set(scene, snapshot);
    }
    currentRun.scene = scene; currentRun.preparedOwner = null; revision.value++; install(snapshot);
    if (currentOwner) await currentOwner.replace({ isCurrent: current });
    if (current()) currentRun.preparedOwner = currentOwner;
  }
  function registerOwner(value) {
    sequence++;
    owner = value;
    return () => { if (owner === value) { owner = null; sequence++; } };
  }
  return { revision, begin, prepare, registerOwner, stop, active: () => !!run };
}
