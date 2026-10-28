import { onBeforeUnmount, onMounted } from 'vue';
import eventBus from '@/eventBus';
import { createDrawerOwnership } from '@/utils/drawerOwnership';

const bodyLockOwners = new Set();

/** The actual drawer state owner; tour events request state rather than toggle DOM. */
export function useMobileDrawer(instance) {
  const bus = instance.proxy.$bus || eventBus;
  const identity = {};
  const write = open => {
    instance.proxy.mobileOpen = open;
    if (!open) instance.proxy.$refs.themeControls?.close();
    if (open) bodyLockOwners.add(identity);
    else bodyLockOwners.delete(identity);
    document.body.classList.toggle('nav-drawer-open', bodyLockOwners.size > 0);
  };
  const ownership = createDrawerOwnership({ read: () => instance.proxy.mobileOpen, write });
  const setMobileOpen = open => ownership.set(open);
  const toggleMobile = () => setMobileOpen(!instance.proxy.mobileOpen);
  const openMobile = () => setMobileOpen(true);
  const onRequest = request => {
    if (!request || typeof request !== 'object'
      || request.element !== instance.proxy.$refs.navigation || !request.element) return;
    request.handled = true;
    if (request.restore) ownership.restore(request.token);
    else ownership.claim(request.open, request.token, request.previousToken);
  };
  onMounted(() => {
    bus.on('mobile-nav-toggle', toggleMobile);
    bus.on('mobile-nav-set', onRequest);
  });
  onBeforeUnmount(() => {
    bus.off('mobile-nav-toggle', toggleMobile);
    bus.off('mobile-nav-set', onRequest);
    ownership.dispose();
    const ownedLock = bodyLockOwners.delete(identity);
    if (ownedLock) document.body.classList.toggle('nav-drawer-open', bodyLockOwners.size > 0);
  });
  return { setMobileOpen, toggleMobile, openMobile };
}
