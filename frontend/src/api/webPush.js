import { apiRequest } from './client';

/**
 * API клиент Web Push подписок (#974): статус (VAPID-ключ, список устройств),
 * подписка и отписка браузера. unwrap повторяет паттерн approvers.js -
 * бросает на !res.ok с сообщением бэка, чтобы 4xx/5xx не выглядели тихим успехом.
 */
async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/**
 * Публичный VAPID-ключ (`public_key`), признак готовности push на сервере
 * (`enabled` - заданы ли VAPID-ключи) и список подписанных устройств текущего
 * пользователя. Форма - `models.PushStatusResponse` (internal/models/push_subscription.go).
 */
export async function getWebPushStatus() {
  const res = await apiRequest('/notifications/push/status');
  return unwrap(res, 'Не удалось получить статус push-уведомлений');
}

/**
 * Регистрирует подписку браузера на сервере после успешного PushManager.subscribe.
 * Тело повторяет форму `PushSubscription.toJSON()` браузера (`models.PushSubscribeRequest`) -
 * endpoint и вложенные keys.p256dh/keys.auth. User-Agent сервер берёт из заголовка
 * запроса сам, в теле его передавать не нужно.
 */
export async function subscribeWebPush({ endpoint, p256dh, auth }) {
  const res = await apiRequest('/notifications/push/subscribe', {
    method: 'POST',
    body: JSON.stringify({ endpoint, keys: { p256dh, auth } }),
  });
  return unwrap(res, 'Не удалось включить push-уведомления');
}

/**
 * Снимает подписку на сервере по endpoint - идентификатор подписки, не числовой
 * id, поэтому передаётся query-параметром, а не сегментом пути.
 */
export async function unsubscribeWebPush(endpoint) {
  const res = await apiRequest(`/notifications/push/subscribe?endpoint=${encodeURIComponent(endpoint)}`, {
    method: 'DELETE',
  });
  return unwrap(res, 'Не удалось отключить push-уведомления');
}
