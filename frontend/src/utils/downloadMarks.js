import { reactive } from 'vue';

// Page-memory only. One expiry timer for all files, no persistence or requests.
const state = reactive({ owner: null, generation: 0, entries: new Map() });
let timer = null;
function scheduleExpiry() {
  clearTimeout(timer); timer = null;
  if (!state.entries.size) return;
  const expiry = Math.min(...state.entries.values());
  timer = setTimeout(() => {
    const now = Date.now();
    for (const [key, until] of state.entries) if (until <= now) state.entries.delete(key);
    scheduleExpiry();
  }, Math.max(0, expiry - Date.now()));
}
export const downloadMarks = {
  setOwner(owner) {
    if (state.owner === owner) return;
    state.owner = owner; state.generation++; state.entries.clear(); scheduleExpiry();
  },
  lease() { return { owner: state.owner, generation: state.generation }; },
  mark(key, lease) {
    if (!lease.owner || lease.owner !== state.owner || lease.generation !== state.generation) return;
    state.entries.set(key, Date.now() + 5 * 60 * 1000); scheduleExpiry();
  },
  has(key) { return (state.entries.get(key) || 0) > Date.now(); },
};
export function downloadMarkKey(applicationId, file) {
  return JSON.stringify([applicationId, file.id, file.updated_at || '', file.file_size, file.file_name]);
}
