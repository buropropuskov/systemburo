import { setReadInterceptor } from '@/api/readInterceptor';
import {
  DEMO_APPLICATION_ID,
  DEMO_ATTACHMENT_ID,
  buildDemoApplication,
  buildDemoResponsibleUsers,
  buildDemoAttachments,
  buildDemoItems,
} from '@/components/onboarding/demoApplication';

/**
 * Подмена ответов на время тура: у человека без своих заявок список кабинета и
 * карточка показывают примерную заявку, а не пустоту.
 *
 * Что подменяем - ровно те чтения, из которых складывается рассказ про заявку:
 * список кабинета, ответственные, состав, дополнения и вопросы. Всё остальное
 * идёт на живой бэкенд как обычно, запись не трогаем совсем.
 *
 * Подмена включается только когда своих заявок нет, и снимается по концу тура -
 * следующий же запрос списка приносит настоящие данные, примерная заявка
 * исчезает сама, без чистки за собой.
 */

const LIST_PATH = /^\/applications\/user(\?|$)/;
const DETAIL_PATH = new RegExp(`^/applications/${DEMO_APPLICATION_ID}(/|\\?|$)`);
const ATTACHMENT_PATH = new RegExp(`^/attachments/${DEMO_ATTACHMENT_ID}(/|\\?|$)`);

const ok = (data, meta) => (meta ? { success: true, data, meta } : { success: true, data });

/**
 * @param {{ organization?: string, company?: string, fullName?: string }} ctx
 * @returns {(path: string) => object|null}
 */
export function createDemoResponder(ctx = {}) {
  const application = buildDemoApplication(ctx);
  const responsibleUsers = buildDemoResponsibleUsers();
  return (path, method = 'GET') => {
    const own = DETAIL_PATH.test(path) || ATTACHMENT_PATH.test(path);
    // Запись по примерной заявке дальше экрана не уходит: такой заявки на сервере
    // нет, и отметка о прочтении возвращалась отказом посреди обучения.
    if (method !== 'GET') return own ? { success: true, data: null } : null;
    if (LIST_PATH.test(path)) return ok([application], { total: 1, page: 1, per_page: 30 });
    if (ATTACHMENT_PATH.test(path)) {
      if (path.includes('/items')) return ok(buildDemoItems());
      return ok([]);
    }
    if (!own) return null;
    if (path.includes('/responsible-users')) return ok(responsibleUsers);
    if (path.includes('/attachments')) return ok(buildDemoAttachments());
    if (path.includes('/details')) return ok({ ...application, responsible_users: responsibleUsers });
    // Прочее по этой заявке - пусто: дополнений, вопросов, файлов и зрителей у
    // примера нет, и шаги про них рассказывают, а не показывают чужие данные.
    return ok([]);
  };
}

/**
 * Привести подмену в соответствие с туром. Пример поднимается только тому, у кого
 * своей заявки нет: остальным он стёр бы настоящий список кабинета. Гаснет вместе
 * с туром - следующий запрос идёт на живой бэкенд, и пример исчезает сам.
 *
 * @param {boolean} tourActive идёт ли обучение
 * @param {boolean} [hasOwnApplication] есть ли у человека своя заявка
 */
export function syncDemoBackend(tourActive, hasOwnApplication = false) {
  setReadInterceptor(tourActive && !hasOwnApplication ? createDemoResponder() : null);
}
