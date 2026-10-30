/**
 * Сегмент тура охранника на фактовой таблице поста: отбор таблицы, шаги отметки
 * въезда и выезда, отчёт. Вынесен из securityOnboardingSteps.js - там же лежат
 * шаги, общие для любого охранника, и вместе файл перерастал порог размера.
 *
 * Поля шага - общие для всех туров, полный список в JSDoc `onboardingSteps.js`.
 */

/** A permitted post and its optional Fact block are different capabilities.
 * The caller must establish that permissions and the table response are known.
 * Wrapped and flat /system-tables entries have the same selection policy.
 */
export function resolveSecurityPostContext(systemTables, canView) {
  if (!Array.isArray(systemTables)) return null;
  let otherType = null;
  for (const item of systemTables) {
    const t = item?.table || item;
    if (!t || !t.is_active || !t.name) continue;
    if (typeof canView === 'function' && !canView(t.name)) continue;
    const context = { route: `/table/${t.name}`, tableType: t.table_type,
      showFactTable: t.show_fact_table === true };
    if (t.table_type === 'cars') return context;
    if (!otherType) otherType = context;
  }
  return otherType;
}

/** Legacy route-only consumer; ordinary passage marks do not require the Fact flag. */
export function resolveFactTableRoute(systemTables, canView) {
  return resolveSecurityPostContext(systemTables, canView)?.route || null;
}

/**
 * Сегмент «Таблицы и отметка въезда/выезда» с ДИНАМИЧЕСКИМ route: целевую
 * фактовую таблицу резолвим в рантайме (resolveFactTableRoute) и подставляем
 * сюда, потому что у разных охранников разные доступные таблицы. Сегмент
 * добавляется в хвост `steps` стора только когда route резолвится - индексы
 * ранних шагов он не сдвигает.
 *
 * Готовность и права устанавливаются до сборки. Редирект/сбой навигации ведёт
 * в восстановление, не к успешному завершению. optionalSegment — прежняя
 * метка формата, хост больше не использует её для автоматического пропуска.
 * Строки и практические кнопки optional; обязательное объяснение Fact остаётся
 * и на пустом посту, и при отсутствии доступного живого блока.
 *
 * Шаг отчёта тоже `optional`, а не `requires`: кнопка гейтится правом
 * `table.<имя таблицы>.report`, то есть ключ у каждой таблицы свой и статическим
 * `requires` не выражается.
 *
 * @param {string|null} route `/table/<name>` целевой фактовой таблицы
 * @returns {Array<object>} обычный пост и обязательное объяснение Fact
 */
export function buildSecurityFactSteps(route, post) {
  // Only the legacy route-only call implies cars; an unknown descriptor does not.
  const tableType = post ? post.tableType : 'cars';
  const showFactTable = post ? post.showFactTable === true : true;
  const liveFact = Boolean(route) && tableType === 'cars' && showFactTable;
  const factIntro = {
    id: 'sec-fact-intro',
    route: liveFact ? route : FINAL_ROUTE,
    element: liveFact ? '[data-testid="fact-table"]' : null,
    ...(liveFact ? { dataOwner: 'fact-list' } : { section: 'fact-example', sectionTitle: 'По факту' }),
    demo: 'factTable',
    scrollTo: 'start',
    title: 'Если номер машины заранее неизвестен — «По факту»',
    description: 'Список «по факту» нужен, когда заявка есть, а номер машины заранее указать не могли. При въезде вводят формат и номер Т/С; марку можно не указывать.'
      + (liveFact ? '' : ' Здесь показан учебный пример: он не меняет данные и не открывает недоступную таблицу.'),
  };
  if (!route || !['cars', 'people'].includes(tableType)) return [factIntro];
  const steps = [
    {
      id: 'sec-table-instruction',
      route,
      element: '[data-testid="ob-table-instruction"]',
      optional: true,
      // Сохранённая legacy-метка; отказ маршрута теперь обрабатывает recovery.
      optionalSegment: true,
      side: 'bottom',
      align: 'start',
      title: 'Кнопки таблицы',
      description:
        'Мы в таблице поста. «Инструкция» объясняет порядок работы. Доступные действия: ручное добавление, экспорт, отчёт за сутки. Если есть кнопка «История», она открывает журнал проходов и проездов поста. При нехватке места команды находятся в «⋯». «Обновить» перечитывает список; отборы находятся над ним или в окне «Фильтры».',
    },
    {
      id: 'sec-pass-intro',
      route,
      // Основная работа поста - таблица «по заявке»: в ней машины и люди из
      // согласованных заявок. Тур раньше показывал только блок «по факту»
      // (ручной ввод) и молчал про главную таблицу.
      element: '[data-testid="cars-table"]',
      optional: true,
      scrollTo: 'start',
      title: 'Список «по заявке»',
      description:
        'Те, кому пропуск уже согласован и кто должен приехать на ваш пост. Здесь вы и отмечаете въезд и выезд.',
    },
    {
      id: 'sec-pass-row',
      route,
      element: '[data-testid="ob-pass-row"]',
      optional: true,
      side: 'bottom',
      align: 'start',
      title: 'Строка списка',
      description:
        'Строка - одна запись из заявки: номер и марка машины или ФИО человека, рядом организация, номер заявки и срок действия пропуска. По самой строке открывается карточка записи - там история отметок и переход в заявку.',
    },
    {
      id: 'sec-pass-entry',
      route,
      element: '[data-testid="ob-pass-entry"]',
      optional: true,
      side: 'bottom',
      align: 'start',
      title: 'Отметка въезда',
      description:
        'Кнопкой «Въезд» отмечаете, что машина заехала. Свою ошибочную отметку следующие пятнадцать минут можно снять кнопкой «Отменить», указав причину. Дальше отметку убирает только администратор.',
    },
    {
      id: 'sec-pass-exit',
      route,
      element: '[data-testid="ob-pass-exit"]',
      optional: true,
      side: 'bottom',
      align: 'start',
      title: 'Отметка выезда',
      description:
        'После отметки въезда становится активной кнопка «Выезд» - ею отмечают, что машина покинула территорию.',
    },
    {
      id: 'sec-on-territory',
      route,
      element: '[data-testid="ob-on-territory"]',
      optional: true,
      side: 'bottom',
      align: 'end',
      title: 'Сколько машин на территории',
      description:
        'Счётчик показывает, сколько машин заехало и ещё не выехало. Он меняется сам, как только вы отмечаете въезд или выезд.',
    },
    ...(liveFact ? [factIntro] : []),
    {
      id: 'sec-fact-report',
      route,
      reveal: { open: 'table-actions' },
      element: '[data-testid="pass-report-button"]',
      optional: true,
      side: 'bottom',
      align: 'end',
      title: 'Отчёт за сутки',
      description: 'Кнопка «Отчёт» подводит итог за отчётные сутки. Сейчас откроем его.',
      advanceWhen: '[data-testid="ob-pass-report"]',
    },
    {
      id: 'sec-fact-report-window',
      route,
      element: '[data-testid="ob-pass-report"]',
      optional: true,
      side: 'left',
      // Тур открывает отчёт сам - иначе рассказ о нём остаётся словами.
      reveal: { open: 'pass-report' },
      title: 'Что в отчёте',
      description:
        'Сколько заехало и выехало за отчётные сутки. Если отмечал не один человек, видно, кто сколько отметил; «Показать прошлые дни» открывает прошедшие сутки, а «Скачать в Excel» сохраняет отчёт файлом.',
    },
  ];
  if (tableType === 'people') {
    const selectors = {
      'sec-pass-intro': '[data-testid="people-table"]',
      'sec-pass-row': '[data-testid="people-table"] .items-body .rt-row',
      'sec-pass-entry': '[data-testid="people-table"] .items-body [data-label="Вход"] button',
      'sec-pass-exit': '[data-testid="people-table"] .items-body [data-label="Выход"] button',
    };
    for (const step of steps) if (selectors[step.id]) step.element = selectors[step.id];
    steps.find(step => step.id === 'sec-pass-intro').description = 'Люди из согласованных заявок, которым разрешён проход на ваш пост. В этом списке отмечают вход и выход.';
    const entry = steps.find(step => step.id === 'sec-pass-entry');
    entry.title = 'Отметка входа';
    entry.description = 'Кнопкой «Вход» отмечают проход человека. Свою ошибочную отметку можно отменить в течение пятнадцати минут, указав причину; позже её исправляет администратор.';
    const exit = steps.find(step => step.id === 'sec-pass-exit');
    exit.title = 'Отметка выхода';
    exit.description = 'После входа кнопкой «Выход» отмечают, что человек покинул территорию.';
  }
  const result = tableType === 'people' ? steps.filter(step => step.id !== 'sec-on-territory') : steps;
  return liveFact ? result : [...result, factIntro];
}

/** Route раздела «Доступные мне» - всегда достижим охранником (canViewAccessibleAttachments). */
export const ACCESSIBLE_ATTACHMENTS_ROUTE = '/accessible-attachments';

/** Страница, на которой заканчиваются все пять туров, - там же кнопка «Обучение». */
export const FINAL_ROUTE = '/news';

/**
 * Финальный шаг security-тура: празднование (`celebrate`) и подсветка кнопки
 * «Обучение», которой тур запускают заново. Шаг строится отдельно (не лежит в
 * базовом массиве), чтобы быть ПОСЛЕДНИМ независимо от того, добавлен ли
 * динамический сегмент фактовой таблицы.
 *
 * Финал всегда на «Обзоре и новостях» - там живёт кнопка «Обучение», и эта
 * страница достижима любому вошедшему. Сегмент фактовой таблицы опционален и
 * может быть недостижим (роут-гард `/table/:name`): если бы финал жил на его
 * route, охранник без доступа к таблицам не дошёл бы до празднования. На
 * достижимой странице финал показывается всегда - после перехода с фактовой
 * таблицы либо перепрыгивания недостижимого сегмента (см. OnboardingTour).
 *
 * @returns {object} финальный шаг
 */
export function buildSecurityFinalStep() {
  return {
    id: 'sec-finish',
    route: FINAL_ROUTE,
    element: '[data-testid="ob-start-button"]',
    celebrate: true,
    title: 'Готово!',
    description:
      'Вернулись на «Обзор и новости». Чтобы повторить объяснения, откройте этот тур кнопкой «Обучение».',
  };
}
