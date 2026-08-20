import { onboardingSteps, ONBOARDING_VERSION } from './onboardingSteps';
import { securityOnboardingSteps, SECURITY_ONBOARDING_VERSION } from './securityOnboardingSteps';
import { buildSecurityFactSteps, buildSecurityFinalStep } from './securityFactSteps';
import { approveOnboardingSteps, APPROVE_ONBOARDING_VERSION } from './approveOnboardingSteps';
import { acceptOnboardingSteps, ACCEPT_ONBOARDING_VERSION } from './acceptOnboardingSteps';
import { adminOnboardingSteps, ADMIN_ONBOARDING_VERSION } from './adminOnboardingSteps';

/**
 * Реестр онбординг-туров. Один пользователь может иметь право сразу на несколько
 * сценариев (принимающий с доступом к Админке), поэтому набор шагов выбирается не
 * ветвлением по типу пользователя, а этим реестром: доступность считает
 * `isAvailable(ctx)`, версия и флаг завершения - свои у каждого тура.
 *
 * Запись тура:
 * - `key`               ключ тура; он же уходит в API (`{tour, version}`) и в testid пункта меню;
 * - `title`             название в меню «Обучение»;
 * - `description`       строка-пояснение под названием в пункте меню;
 * - `version`           текущая версия набора шагов (см. hasCompleted/«Обновлён»);
 * - `steps`             статический массив шагов; ПУСТОЙ = тур ещё не написан;
 * - `buildSteps(ctx)`   необязательный сборщик, если у тура есть динамический хвост
 *                       (охраннику сегмент фактовой таблицы резолвится в рантайме);
 * - `isAvailable(ctx)`  доступен ли тур этому пользователю;
 * - `autostartPriority` порядок автозапуска: меньше = приоритетнее. Автозапускается
 *                       РОВНО ОДИН профильный тур, остальные - только вручную.
 *
 * `ctx` - плоский снимок прав и ролей, чтобы правила гейтинга проверялись без Pinia:
 * `{ isAuthenticated, isSecurity, can(key), approvalRole: { isApprover, isReviewer } }`.
 */

/** @typedef {{ isAuthenticated: boolean, isSecurity: boolean, can: (key: string) => boolean, approvalRole: { isApprover: boolean, isReviewer: boolean } }} TourContext */

export const TOURS = [
  {
    key: 'user',
    title: 'Пользователь',
    description: 'Разделы системы, свои справочники и оформление заявки',
    version: ONBOARDING_VERSION,
    steps: onboardingSteps,
    isAvailable: (ctx) => ctx.isAuthenticated,
    autostartPriority: 5,
  },
  {
    key: 'guard',
    title: 'Охранник',
    description: 'Согласованные бланки заявок и отметка въезда и выезда',
    version: SECURITY_ONBOARDING_VERSION,
    steps: securityOnboardingSteps,
    // Сегмент отметки въезда/выезда живёт на роуте фактовой таблицы, а он у
    // каждого охранника свой - резолвится в рантайме и приезжает через ctx.
    // Финал всегда замыкает тур на достижимом /accessible-attachments.
    buildSteps: (ctx) => [
      ...securityOnboardingSteps,
      ...buildSecurityFactSteps(ctx?.factTableRoute || null),
      buildSecurityFinalStep(),
    ],
    // Доступ к «Доступным мне» обязателен, как центр заявок у согласующего: на
    // этой странице живут шесть шагов и финал, а её роут закрыт гардом
    // `requiresSecurityOrAdmin` (супер-админ, охранник, право page.available).
    // Раньше в гейт входило ещё page.tables - работник с одним этим правом видел
    // тур, проходил вступление на новостях и на первом шаге сегмента уезжал в
    // личный кабинет, теряя шесть шагов и празднование.
    isAvailable: (ctx) => ctx.isSecurity || ctx.can('page.available'),
    autostartPriority: 1,
  },
  {
    key: 'approve',
    title: 'Согласующий',
    description: 'Рассмотрение заявок и ответы заявителям',
    version: APPROVE_ONBOARDING_VERSION,
    steps: approveOnboardingSteps,
    // Доступ в центр заявок обязателен: чужие заявки согласуют только оттуда, и
    // почти весь тур живёт на /center. Без права человек прошёл бы два вступительных
    // шага и упёрся в роут-гард, не увидев ни голосования, ни финала.
    isAvailable: (ctx) => (ctx.approvalRole.isReviewer || ctx.can('action.approve.application'))
      && ctx.can('page.center'),
    autostartPriority: 4,
  },
  {
    key: 'accept',
    title: 'Принимающий',
    description: 'Приём заявок и работа со своими согласованиями',
    version: ACCEPT_ONBOARDING_VERSION,
    steps: acceptOnboardingSteps,
    // Как и у согласующего: приём заявок идёт из центра, там же весь тур.
    isAvailable: (ctx) => ctx.approvalRole.isApprover && ctx.can('page.center'),
    autostartPriority: 3,
  },
  {
    key: 'admin',
    title: 'Администратор',
    description: 'Разделы Админки: справочники, права и настройки системы',
    version: ADMIN_ONBOARDING_VERSION,
    steps: adminOnboardingSteps,
    isAvailable: (ctx) => ctx.can('page.admin'),
    autostartPriority: 2,
  },
];

/** Пустой контекст - гейтинг и сборка шагов не должны падать до загрузки прав. */
const EMPTY_CONTEXT = {
  isAuthenticated: false,
  isSecurity: false,
  can: () => false,
  approvalRole: { isApprover: false, isReviewer: false },
};

/**
 * @param {Partial<TourContext>} [ctx]
 * @returns {TourContext}
 */
function normalizeContext(ctx) {
  return {
    ...EMPTY_CONTEXT,
    ...ctx,
    can: typeof ctx?.can === 'function' ? ctx.can : EMPTY_CONTEXT.can,
    approvalRole: { ...EMPTY_CONTEXT.approvalRole, ...ctx?.approvalRole },
  };
}

/**
 * @param {string} key
 * @returns {object|null} запись реестра или null
 */
export function getTour(key) {
  return TOURS.find((t) => t.key === key) || null;
}

/**
 * Собрать шаги тура: статический массив либо динамический сборщик. Фильтрация по
 * правам (`requires`) идёт отдельно, в сторе - тут только состав.
 *
 * @param {object|string} tourOrKey
 * @param {object} [ctx] дополнительные данные сборщику (напр. factTableRoute)
 * @returns {Array<object>}
 */
export function buildTourSteps(tourOrKey, ctx = {}) {
  const tour = typeof tourOrKey === 'string' ? getTour(tourOrKey) : tourOrKey;
  if (!tour) return [];
  return tour.buildSteps ? tour.buildSteps(ctx) : tour.steps;
}

/**
 * Написан ли тур. Пустой массив шагов = запись-заготовка под будущий срез: такой
 * тур не должен ни висеть пустым пунктом в меню, ни автозапускаться.
 *
 * @param {object} tour
 * @returns {boolean}
 */
export function isTourReady(tour) {
  return Boolean(tour?.steps?.length);
}

/**
 * Туры, доступные пользователю: написанные И прошедшие гейт по правам. Порядок -
 * как в реестре (он же порядок пунктов меню).
 *
 * @param {Partial<TourContext>} ctx
 * @returns {Array<object>}
 */
export function availableTours(ctx) {
  const c = normalizeContext(ctx);
  return TOURS.filter((t) => isTourReady(t) && t.isAvailable(c));
}

/**
 * Тур для автозапуска: самый приоритетный из доступных и непройденных. Ровно один -
 * остальные доступные пользователь запускает вручную из меню.
 *
 * @param {Partial<TourContext>} ctx
 * @param {(tourKey: string) => boolean} isCompleted
 * @returns {object|null}
 */
export function pickAutostartTour(ctx, isCompleted) {
  return availableTours(ctx)
    .filter((t) => !isCompleted(t.key))
    .sort((a, b) => a.autostartPriority - b.autostartPriority)[0] || null;
}

/**
 * Все шаги всех туров, включая динамические хвосты. Нужен guard-тесту на
 * существование селекторов: статический `steps` не покрывает шаги, которые
 * собираются в рантайме, а именно там якорь гниёт незаметнее всего.
 *
 * @returns {Array<object>}
 */
export function allTourSteps() {
  return TOURS.flatMap((t) => buildTourSteps(t, { factTableRoute: '/table/selector-lock' }));
}
