// Service worker Web Push (#974): показывает системное уведомление, когда
// вкладка приложения закрыта, и по клику ведёт человека к заявке, из-за
// которой пришло уведомление. Лежит в public/ - Vite копирует его в корень
// сборки как есть, регистрация с default scope покрывает весь сайт.
//
// Регистрирует этот файл фронт при включении push в настройках уведомлений
// (frontend/src/utils/webPushSubscription.js) - не на каждом входе.

// Форма пейлоада - PushPayload (internal/services/push_service.go): title,
// message, type, notification_id, application_id (опционален). Явного url
// пейлоад не несёт, и адрес заявки СПЕЦИАЛЬНО не строится здесь: у Центра
// заявок и личного кабинета разные маршруты (page.center), а service worker
// живёт вне вкладки и вне Pinia - прав пользователя не знает и знать не может.
// Ведём на нейтральный вход /?open_application=<id> - router.js дожидается
// прав и решает Центр vs личный кабинет тем же кодом, что клик по карточке
// уведомления (useNotificationNavigation.resolveApplicationRoute), см. #974.
self.addEventListener('push', (event) => {
  let payload;
  try {
    payload = event.data ? event.data.json() : {};
  } catch {
    // Тело не JSON - fallback на голый текст, чтобы уведомление всё равно показалось.
    payload = { message: event.data ? event.data.text() : '' };
  }

  const title = payload.title || 'Бюро пропусков';
  const url = payload.url || (payload.application_id ? `/?open_application=${payload.application_id}` : '/');
  const options = {
    body: payload.message || '',
    icon: '/icon.jpg',
    // tag схлопывает повторные уведомления по одной заявке - новый push с тем
    // же tag заменяет предыдущий в системном центре вместо роста стопки.
    tag: payload.application_id ? `application-${payload.application_id}` : undefined,
    data: { url },
  };

  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const targetUrl = event.notification.data?.url || '/';

  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
      const existing = windows.find((client) => 'focus' in client);
      if (existing) {
        await existing.focus();
        if ('navigate' in existing) await existing.navigate(targetUrl);
        return;
      }
      await self.clients.openWindow(targetUrl);
    })(),
  );
});
