import { TOURS, getTour } from './tours';

/** Context dependencies follow the chosen tour, never an unrelated endpoint. */
export function createContextReadiness({ auth, permissions, data, getContext }) {
  const identity = () => `${data.getGeneration()}:${auth.userId ?? auth.username ?? ''}`;
  // Only this actor-scoped resource rejects replies from a previous identity.
  const knownType = () => data.userTypeCode.value;
  const result = (missing = []) => ({ status: missing.length ? 'blocked' : 'ready', missing });
  function prerequisites(key) {
    const missing = [];
    if (!permissions.loaded) return ['permissions'];
    if (key === 'guard') {
      if (!permissions.hasPermission('page.available') && !knownType()) missing.push('type');
      if (!data.factRouteLoaded.value) missing.push('post');
    }
    if (key === 'user' && !data.ownApplicationLoaded.value) missing.push('own');
    if ((key === 'accept' || (key === 'approve' && !permissions.hasPermission('action.approve.application')))
      && !data.approvalRoleLoaded.value) missing.push('approval');
    return missing;
  }
  function availability(key) {
    const context = getContext();
    if (key === 'user') return true; // Authentication/consent are owned by the launch entry point.
    return Boolean(getTour(key)?.isAvailable(context));
  }
  function status(key) {
    if (!getTour(key)) return { status: 'unavailable', tourKey: key };
    const missing = prerequisites(key);
    if (missing.length) return { ...result(missing), tourKey: key };
    return { status: availability(key) ? 'ready' : 'unavailable', tourKey: key };
  }
  async function resolve(missing) {
    await Promise.all(missing.map(key => ({
      permissions: () => permissions.fetchPermissions(), type: data.ensureUserType,
      post: data.ensureFactRoute, own: data.ensureOwnApplication, approval: data.ensureApprovalRole,
    })[key]()));
  }
  async function ensureSelectionContext() {
    const owner = identity();
    await resolve(permissions.loaded ? [] : ['permissions']);
    if (owner !== identity()) return { status: 'stale' };
    if (!permissions.loaded) return result(['permissions']);
    if (!permissions.hasPermission('page.available') && !knownType()) await resolve(['type']);
    if (owner !== identity()) return { status: 'stale' };
    if (!permissions.hasPermission('page.available') && !knownType()) return result(['type']);
    const priority = [...TOURS].sort((a, b) => a.autostartPriority - b.autostartPriority);
    for (const tour of priority) {
      // An unknown approval role matters only once higher-priority candidates were excluded.
      if (['accept', 'approve'].includes(tour.key) && permissions.hasPermission('page.center')
        && !data.approvalRoleLoaded.value) {
        await resolve(['approval']);
        if (owner !== identity()) return { status: 'stale' };
        if (!data.approvalRoleLoaded.value) return result(['approval']);
      }
      if (tour.isAvailable(getContext())) return result();
    }
    return result();
  }
  async function prepareTour(key) {
    const owner = identity();
    await resolve(permissions.loaded ? [] : ['permissions']);
    if (owner !== identity()) return { status: 'stale' };
    if (!permissions.loaded) return { ...result(['permissions']), tourKey: key };
    // Do not fetch data for a page whose absence is already known.
    if (key === 'admin' && !permissions.hasPermission('page.admin')) return { status: 'unavailable', tourKey: key };
    if (['accept', 'approve'].includes(key) && !permissions.hasPermission('page.center')) return { status: 'unavailable', tourKey: key };
    if (key === 'guard' && !permissions.hasPermission('page.available') && !knownType()) await resolve(['type']);
    if (owner !== identity()) return { status: 'stale' };
    if (key === 'guard' && !permissions.hasPermission('page.available') && !knownType()) return { ...result(['type']), tourKey: key };
    if (key === 'guard' && knownType() && !getTour(key).isAvailable(getContext())) return { status: 'unavailable', tourKey: key };
    await resolve(prerequisites(key));
    if (owner !== identity()) return { status: 'stale' };
    return status(key);
  }
  return { ensureSelectionContext, prepareTour, status, knownType, identity,
    isReady: key => prerequisites(key).length === 0 };
}

/** Only gates relevant to selecting a post invalidate that cached context. */
export function factPolicyKey(permissions) {
  const relevant = key => key === 'page.tables' || /^table\..+\.view$/.test(key);
  return JSON.stringify([permissions.loaded, permissions.mode,
    Object.entries(permissions.effective).filter(([key]) => relevant(key)).sort(([a], [b]) => a.localeCompare(b)),
    [...permissions.denied].filter(relevant).sort()]);
}
