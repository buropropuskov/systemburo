/**
 * Низкоуровневые обёртки над Push API / Service Worker (#974) - без
 * Vue-реактивности, чтобы их мог использовать и composable настроек
 * (useWebPush), и logout() в App.vue (Options API, вызывается вне setup()).
 */

/** Публичный путь SW - лежит в public/, Vite копирует его в корень сборки как есть. */
export const SW_URL = '/sw.js';

/**
 * @returns {boolean} браузер умеет Push API - нет смысла показывать что-либо,
 * кроме состояния "не поддерживается", если хоть одной из трёх API нет.
 */
export function isPushSupported() {
  return typeof window !== 'undefined'
    && typeof navigator !== 'undefined'
    && 'serviceWorker' in navigator
    && 'PushManager' in window
    && 'Notification' in window;
}

/**
 * VAPID-ключ сервера в base64url (RFC4648) -> Uint8Array, формат, который
 * ожидает `PushManager.subscribe({ applicationServerKey })`.
 * @param {string} base64String
 * @returns {Uint8Array}
 */
export function urlBase64ToUint8Array(base64String) {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const raw = atob(base64);
  const output = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i += 1) output[i] = raw.charCodeAt(i);
  return output;
}

/**
 * Регистрирует service worker - вызывать ТОЛЬКО при включении push (клик
 * "Включить"), не на каждый вход: уже поднятая регистрация переживает
 * перезагрузки страницы сама, без повторного register().
 */
export async function registerPushServiceWorker() {
  return navigator.serviceWorker.register(SW_URL);
}

/** Регистрация SW этого приложения для scope '/', если она уже есть (без создания новой). */
async function existingRegistration() {
  if (!isPushSupported()) return null;
  return navigator.serviceWorker.getRegistration('/');
}

/** Текущая push-подписка браузера на этом устройстве, если она есть. */
export async function getCurrentSubscription() {
  const registration = await existingRegistration();
  if (!registration) return null;
  return registration.pushManager.getSubscription();
}

/** Подписывает браузер на push через VAPID-ключ сервера, регистрируя SW при необходимости. */
export async function subscribeToPush(vapidPublicKey) {
  const registration = await registerPushServiceWorker();
  await navigator.serviceWorker.ready;
  return registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(vapidPublicKey),
  });
}

/** Снимает подписку в браузере. Серверную запись чистит вызывающий код отдельно. */
export async function unsubscribeLocal(subscription) {
  if (!subscription) return;
  await subscription.unsubscribe();
}
