// Service worker Web Push (#974): показывает системное уведомление, когда
// вкладка приложения закрыта, и по клику ведёт человека к заявке, из-за
// которой пришло уведомление. Лежит в public/ - Vite копирует его в корень
// сборки как есть, регистрация с default scope покрывает весь сайт.
//
// Регистрирует этот файл фронт при включении push в настройках уведомлений
// (frontend/src/utils/webPushSubscription.js) - не на каждом входе.

// Обновлённый обработчик не должен ждать, пока человек закроет все вкладки:
// без этой пары новая версия висит в waiting, а клики продолжает обрабатывать
// старая. claim() к тому же забирает под управление уже открытые страницы -
// от этого зависит navigate() ниже. Кэша тут нет (обработчика fetch тоже),
// поэтому захват открытых страниц им ничем не грозит.
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()));

// Форма пейлоада - PushPayload (internal/services/push_service.go): title,
// message, type, notification_id, application_id (опционален). Явного url
// пейлоад не несёт, и адрес заявки СПЕЦИАЛЬНО не строится здесь: у Центра
// заявок и личного кабинета разные маршруты (page.center), а service worker
// живёт вне вкладки и вне Pinia - прав пользователя не знает и знать не может.
// Ведём на нейтральный вход /?open_application=<id> - router.js дожидается
// прав и решает Центр vs личный кабинет тем же кодом, что клик по карточке
// уведомления (useNotificationNavigation.resolveApplicationRoute), см. #974.
// Картинка уведомления говорит о поводе раньше, чем человек прочтёт текст: в шторке
// сначала видно значок. Плашка у всех одна - индиговая, как знак в шапке системы, - а
// символ внутри разный, и все символы взяты из того же набора, которым нарисовано меню
// (navIcons.js): конверт у Центра заявок, щит с галочкой у прав, газета у новостей.
// Незнакомый тип получает общий знак системы, а не пустоту.
const TYPE_ICONS = {
  application_created: 'application',
  application_pending_acceptance: 'application',
  application_forwarded: 'application',
  application_approval_required: 'approval',
  application_approval_reminder: 'approval',
  application_status_changed: 'approval',
  application_supplement_ready: 'approval',
  application_supplement_decided: 'approval',
  application_question: 'question',
  application_answer: 'question',
  password_changed: 'security',
  user_banned: 'security',
  user_unbanned: 'security',
  login_blocked: 'security',
  news_published: 'content',
  document_published: 'content',
  feedback_created: 'content',
  feedback_answered: 'content',
};

function iconForType(type) {
  const name = TYPE_ICONS[type];
  return name ? `/notification-icon-${name}.png` : '/notification-icon.png';
}

// Значок строки состояния тоже свой на каждый повод: до этого он был один, и в шторке
// все уведомления выглядели одинаково, пока их не развернёшь. Знака Бюро в нём нет и
// быть не может - значок монохромный и размером с букву, двум символам там не разойтись,
// поэтому принадлежность к системе несёт крупная картинка, а значок - повод.
function badgeForType(type) {
  const name = TYPE_ICONS[type];
  return name ? `/notification-badge-${name}.png` : '/notification-badge.png';
}

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
  // tag схлопывает ПОВТОРЫ ОДНОГО СОБЫТИЯ по одной заявке: новый push с тем же
  // тегом заменяет предыдущий вместо роста стопки. В теге обязателен тип: без
  // него «требуется согласование» и «новый вопрос» по одной заявке считались бы
  // одним событием и затирали друг друга, а это разные поводы для человека.
  //
  // renotify обязателен вместе с tag. Браузер заменяет уведомление с прежним
  // тегом МОЛЧА: без звука и без всплытия, сразу в центр уведомлений. Проверка
  // на живом стенде показала ровно это - уведомления о заявке приходили
  // незаметно, тогда как без тега всплывали нормально. Задумано было «не
  // спамить повторами», а вышло «важное проходит мимо человека».
  const tag = payload.application_id
    ? `app-${payload.application_id}-${payload.type || 'event'}`
    : undefined;
  // Две РАЗНЫЕ картинки, и путать их нельзя. icon - крупная, в теле уведомления:
  // на ней знак Бюро подложкой и символ события поверх, чтобы читались и отправитель,
  // и повод. badge - монохромный значок в строке состояния Android; там помещается
  // только символ события, а цвет не важен - система красит значок сама, поэтому файл
  // белый на прозрачном.
  const options = {
    body: payload.message || '',
    icon: iconForType(payload.type),
    badge: badgeForType(payload.type),
    tag,
    renotify: Boolean(tag),
    data: { url },
  };

  event.waitUntil(self.registration.showNotification(title, options));
});

// Сколько ждём ответа вкладки, прежде чем считать её глухой. Вкладка отвечает
// синхронно в обработчике сообщения, так что задержка тут - не про скорость
// роутера, а про «слушателя нет вовсе»: бандл, загруженный до выката, о
// сообщении не знает и не ответит никогда.
const APP_NAVIGATE_TIMEOUT_MS = 700;

/**
 * Просит открытую вкладку сменить маршрут своими силами. Основной путь: переход
 * внутри приложения без перезагрузки, и работает он даже когда вкладка этому
 * worker'у не подчиняется. Ответ - признак того, что слушатель на той стороне
 * есть; молчание означает «переходи сам».
 */
function askAppToNavigate(client, url) {
  return new Promise((resolve) => {
    const channel = new MessageChannel();
    const timer = setTimeout(() => resolve(false), APP_NAVIGATE_TIMEOUT_MS);
    channel.port1.onmessage = (event) => {
      clearTimeout(timer);
      resolve(event.data?.ok === true);
    };
    client.postMessage({ type: 'push-navigate', url }, [channel.port2]);
  });
}

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const targetUrl = event.notification.data?.url || '/';

  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
      const existing = windows.find((client) => 'focus' in client);
      if (!existing) {
        await self.clients.openWindow(targetUrl);
        return;
      }

      await existing.focus();
      if (await askAppToNavigate(existing, targetUrl)) return;

      // Вкладка не ответила. navigate() перезагружает страницу целиком и
      // разрешён только для клиента под управлением этого worker'а: на стенде
      // вкладка, открытая до включения push, получала здесь отказ - окно
      // всплывало сфокусированным, но оставалось на прежней странице, и
      // человек не понимал, куда делась заявка. Отказ не тупик: открываем
      // адрес новым окном.
      try {
        if ('navigate' in existing) {
          await existing.navigate(targetUrl);
          return;
        }
      } catch (err) {
        console.warn('[sw] переход в открытой вкладке отклонён, открываю новым окном:', err);
      }
      await self.clients.openWindow(targetUrl);
    })(),
  );
});
