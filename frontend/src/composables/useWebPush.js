import { ref, computed, onMounted } from 'vue';
import { getWebPushStatus, subscribeWebPush, unsubscribeWebPush } from '@/api/webPush';
import {
  isPushSupported,
  getCurrentSubscription,
  subscribeToPush,
  unsubscribeLocal,
} from '@/utils/webPushSubscription';
import { needsIosHomeScreenInstall, iosNeedsSafari } from '@/utils/webPushPlatform';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Состояние и действия блока Web Push в настройках уведомлений (#974).
 * Держит признак поддержки браузером, разрешение Notification, готовность
 * push на сервере (VAPID-ключ), список подписанных устройств пользователя и
 * локальную подписку этого устройства.
 *
 * @returns {object} реактивное состояние + `enable`/`disable`
 */
export function useWebPush() {
  const supported = ref(isPushSupported());
  const iosNeedsInstall = ref(needsIosHomeScreenInstall());
  // Сторонний браузер на iOS - тупик: подключить push оттуда нельзя, и
  // подсказка про экран «Домой» ему не поможет (#974).
  const iosNeedsSafariBrowser = ref(iosNeedsSafari());
  const permission = ref(supported.value && typeof Notification !== 'undefined' ? Notification.permission : 'default');
  const serverConfigured = ref(false);
  const vapidKey = ref(null);
  const devices = ref([]);
  const subscription = ref(null);
  const loading = ref(false);
  const busy = ref(false);

  const enabledOnDevice = computed(() => Boolean(subscription.value));

  async function loadStatus() {
    loading.value = true;
    try {
      const status = await getWebPushStatus();
      vapidKey.value = status?.public_key || null;
      serverConfigured.value = Boolean(status?.enabled) && Boolean(vapidKey.value);
      devices.value = Array.isArray(status?.devices) ? status.devices : [];
    } catch (e) {
      useDeletionsStore().notify({
        prefix: 'Не удалось получить статус push-уведомлений: ',
        bold: e?.message || 'ошибка сети',
        type: 'error',
      });
    } finally {
      loading.value = false;
    }
  }

  async function refreshSubscription() {
    if (!supported.value) return;
    subscription.value = await getCurrentSubscription();
  }

  /** Запрашивает разрешение браузера (если ещё не спрашивали) и подписывает устройство. */
  async function enable() {
    if (!supported.value || busy.value || permission.value === 'denied') return;
    busy.value = true;
    try {
      permission.value = await Notification.requestPermission();
      if (permission.value !== 'granted') return;

      if (!vapidKey.value) await loadStatus();
      if (!vapidKey.value) throw new Error('На сервере не настроены push-уведомления');

      const sub = await subscribeToPush(vapidKey.value);
      const keys = sub.toJSON().keys || {};
      try {
        await subscribeWebPush({ endpoint: sub.endpoint, p256dh: keys.p256dh, auth: keys.auth });
      } catch (err) {
        // Сервер отказал регистрировать подписку - оставлять её в браузере
        // бессмысленно (пуши всё равно никто не пришлёт), снимаем сразу.
        await unsubscribeLocal(sub);
        throw err;
      }

      subscription.value = sub;
      await loadStatus();
      useDeletionsStore().notify({ bold: 'Push-уведомления', suffix: ' включены на этом устройстве' });
    } catch (e) {
      useDeletionsStore().notify({
        prefix: 'Не удалось включить push-уведомления: ',
        bold: e?.message || 'ошибка сети',
        type: 'error',
      });
    } finally {
      busy.value = false;
    }
  }

  /** Снимает подписку и в браузере, и на сервере. Разрешение браузера при этом не трогается. */
  async function disable() {
    if (busy.value) return;
    busy.value = true;
    try {
      const sub = subscription.value || await getCurrentSubscription();
      if (sub) {
        await unsubscribeWebPush(sub.endpoint);
        await unsubscribeLocal(sub);
      }
      subscription.value = null;
      await loadStatus();
      useDeletionsStore().notify({ bold: 'Push-уведомления', suffix: ' отключены на этом устройстве' });
    } catch (e) {
      useDeletionsStore().notify({
        prefix: 'Не удалось отключить push-уведомления: ',
        bold: e?.message || 'ошибка сети',
        type: 'error',
      });
    } finally {
      busy.value = false;
    }
  }

  onMounted(async () => {
    if (!supported.value) return;
    await Promise.all([loadStatus(), refreshSubscription()]);
  });

  return {
    supported,
    iosNeedsInstall,
    iosNeedsSafariBrowser,
    permission,
    serverConfigured,
    devices,
    loading,
    busy,
    enabledOnDevice,
    enable,
    disable,
  };
}
