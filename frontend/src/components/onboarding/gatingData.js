import { computed, ref, watch } from 'vue';
import { getSecurityPostContext } from '@/api/onboarding';
import { getMyApprovalRole } from '@/api/approvers';
import { getUserApplicationsPaginated, getAccessibleAttachments } from '@/api/applications';
import { getMe } from '@/api/auth';

/** Per-user resources: successful emptiness is data, a failed request is not. */
export function createGatingData({ factPolicyKey = () => '', availablePolicyKey = () => '' } = {}) {
  let generation = 0;
  const resources = [];
  function resource(initial, read, key = () => '') {
    const fresh = () => initial && typeof initial === 'object' ? { ...initial } : initial;
    const value = ref(fresh()), loaded = ref(false);
    let pending = null;
    async function ensure(refresh = false) {
      const current = generation, policy = key();
      if (pending?.generation === current && pending.policy === policy) return pending.promise;
      if (loaded.value && !refresh) return true;
      if (refresh) loaded.value = false;
      const request = { generation: current, policy };
      request.promise = Promise.resolve().then(async () => {
        if (current !== generation) return false;
        try {
          const result = await read();
          if (current !== generation || policy !== key()) return false;
          value.value = result; loaded.value = true;
          return true;
        } catch { return false; }
        finally { if (pending === request) pending = null; }
      });
      pending = request;
      return request.promise;
    }
    const result = { value, loaded, ensure, reset() { value.value = fresh(); loaded.value = false; pending = null; } };
    resources.push(result);
    return result;
  }
  const approval = resource({ isApprover: false, isReviewer: false }, async () => {
    const data = await getMyApprovalRole();
    if (typeof data?.is_approver !== 'boolean' || typeof data?.is_reviewer !== 'boolean') throw new Error('Unknown learning role');
    return { isApprover: data.is_approver, isReviewer: data.is_reviewer };
  });
  const post = resource(null, getSecurityPostContext, factPolicyKey);
  watch(factPolicyKey, () => { post.loaded.value = false; }, { flush: 'sync' });
  const available = resource(null, async () => {
    const { items } = await getAccessibleAttachments({ page: 1, per_page: 1 });
    if (!Array.isArray(items)) throw new Error('Unknown available attachment list');
    return { hasItems: items.length > 0 };
  }, availablePolicyKey);
  watch(availablePolicyKey, () => { available.loaded.value = false; }, { flush: 'sync' });
  const own = resource(false, async () => {
    const current = generation;
    // Avoid the auth -> onboarding -> auth static module cycle.
    const { useAuthStore } = await import('@/stores/auth');
    if (current !== generation) throw new Error('Stale learning context');
    const senderId = useAuthStore().userId;
    if (!senderId) throw new Error('Unknown learning identity');
    const { items, meta } = await getUserApplicationsPaginated({ page: 1, per_page: 1, sender_user_id: senderId });
    if (!Array.isArray(items) || (meta?.total !== undefined && !Number.isFinite(meta.total))) throw new Error('Unknown application list');
    return Boolean(meta?.total > 0 || items.length);
  });
  const type = resource(null, async () => {
    const data = await getMe();
    if (typeof data?.user_type_code !== 'string' || !data.user_type_code) throw new Error('Unknown learning type');
    return data.user_type_code;
  });
  function reset() { generation++; resources.forEach(item => item.reset()); }
  return {
    approvalRole: approval.value, approvalRoleLoaded: approval.loaded, ensureApprovalRole: approval.ensure,
    securityPostContext: post.value, factTableRoute: computed(() => post.value.value?.route || null),
    factRouteLoaded: post.loaded, ensureFactRoute: post.ensure,
    accessibleContext: available.value, accessibleContextLoaded: available.loaded, ensureAccessibleContext: available.ensure,
    hasOwnApplication: own.value, ownApplicationLoaded: own.loaded, ensureOwnApplication: own.ensure,
    userTypeCode: type.value, userTypeLoaded: type.loaded, ensureUserType: type.ensure,
    getGeneration: () => generation, reset,
  };
}
