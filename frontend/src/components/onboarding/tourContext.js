import { computed, onScopeDispose, ref } from 'vue';
import { isPhoneViewport as readPhoneViewport } from '@/constants/breakpoints';

/** Read the current stores without changing the application's access policy. */
export function readTourContext(auth, permissions, {
  approvalRole, factTableRoute, hasOwnApplication, userTypeCode, securityPostContext, factRouteLoaded, accessibleContext,
  isPhoneViewport = false,
}) {
  return {
    isPhoneViewport,
    isAuthenticated: auth.isAuthenticated,
    isSecurity: userTypeCode === 'security',
    can: key => permissions.hasPermission(key),
    allowsApplicationAction: key => permissions.allowsApplicationAction(key),
    canViewAccessibleAttachments: userTypeCode === 'security' || auth.isSuperAdmin
      || permissions.hasPermission('page.available'),
    permissionsLoaded: permissions.loaded === true,
    approvalRole: approvalRole.value,
    factTableRoute: factTableRoute.value,
    securityPostContext: securityPostContext.value,
    factRouteLoaded: factRouteLoaded.value,
    accessibleContext: accessibleContext.value,
    hasOwnApplication: hasOwnApplication.value,
  };
}

/** Pinia's scope owns the listener; rotating a device keeps the same lesson ID. */
export function createTourContext(auth, permissions, data) {
  const phone = ref(readPhoneViewport());
  const update = () => { phone.value = readPhoneViewport(); };
  if (typeof window !== 'undefined') {
    window.addEventListener('resize', update, { passive: true });
    onScopeDispose(() => window.removeEventListener('resize', update));
  }
  return computed(() => readTourContext(auth(), permissions(), {
    ...data(), isPhoneViewport: phone.value,
  }));
}
