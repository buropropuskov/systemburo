<template>
  <section
    ref="root"
    class="admin-page"
  >
    <slot />
  </section>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue';
import { getViewportZoom } from '@/utils/viewportScale';

/**
 * Полноширинная обёртка страниц /admin/*: растягивает вложенный компонент
 * управления на всю доступную высоту вьюпорта (под шапкой) и не даёт ему
 * вылезти за край - чтобы из-за height:100% не появлялся скролл всей страницы
 * и пустой хвост снизу. Высоту меряем фактически (innerHeight - top карточки),
 * а не 100dvh минус хардкод шапки: шапка in-flow и может менять высоту.
 */
const root = ref(null);
let resizeObserver = null;
let lastHeight = -1;

/**
 * На телефоне обёртка высоту НЕ фиксирует и скролл себе не забирает.
 *
 * Замер стенда (390x844, «Доступные мне»): `documentElement.scrollHeight`
 * равнялся `clientHeight`, то есть окно не прокручивалось вовсе, а
 * прокручивалась эта обёртка - 789px видимой части при содержимом 1453.
 * Вложенный скроллпорт не отдаёт инерцию, не сворачивает адресную строку
 * браузера и залипает на границе, поэтому жест ощущается как «то скроллит,
 * то зависает, то дёргает» (претензия пользователя, волна 5).
 *
 * Порог 767.98 - тот же, на котором таблицы становятся карточками
 * (`responsive-tables.css`), иначе ровно на 768 собирается гибрид.
 */
const MOBILE_QUERY = '(max-width: 767.98px)';

function isMobileViewport() {
  return typeof window !== 'undefined'
    && typeof window.matchMedia === 'function'
    && window.matchMedia(MOBILE_QUERY).matches;
}

function applyHeight() {
  const el = root.value;
  if (!el) return;
  if (isMobileViewport()) {
    // Инлайновую высоту снимаем: она могла остаться от десктопной ширины
    // после поворота экрана или изменения размера окна.
    if (el.style.height) el.style.height = '';
    lastHeight = -1;
    return;
  }
  // rect.top под корневым zoom - в device-px, а innerHeight - НЕзумленный;
  // делим доступную device-высоту на zoom, чтобы получить CSS-высоту (иначе
  // элемент выходит в zoom раз выше экрана - пустой длинный хвост снизу).
  const top = el.getBoundingClientRect().top;
  const height = Math.max(0, Math.round((window.innerHeight - top) / getViewportZoom()));
  // Защита от ResizeObserver-петли: пишем стиль только при реальном изменении.
  if (height === lastHeight) return;
  lastHeight = height;
  el.style.height = `${height}px`;
}

onMounted(async () => {
  await nextTick();
  applyHeight();
  window.addEventListener('resize', applyHeight);
  // Пересчёт при изменении высоты шапки (анонс, перенос строки). Наблюдаем
  // саму шапку, а не body - иначе Teleport-модалки (их в проекте десятки)
  // будили бы лишний reflow на каждое открытие.
  const header = document.querySelector('.theheader');
  if (header && typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(applyHeight);
    resizeObserver.observe(header);
  }
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', applyHeight);
  if (resizeObserver) {
    resizeObserver.disconnect();
    resizeObserver = null;
  }
});
</script>

<style scoped>
.admin-page {
  padding: 16px;
  box-sizing: border-box;
  /* Высоту задаёт скрипт под доступный вьюпорт; overflow здесь держит скролл
     внутри обёртки, а не на всей странице, если контент выше доступного. */
  overflow: auto;
}

/* Карточка компонента управления занимает всю высоту обёртки. Корень делаем
   flex-колонкой, а мастер-детейл (.content-container) - flex:1, чтобы тело
   тянулось на всю высоту вместо фикс. 450/500px и снизу не оставался пустой
   хвост. Внутренний скролл списка (.table-body overflow:auto) не трогаем. */
.admin-page :deep(.dashboard-card) {
  height: 100%;
  border-radius: 35px;
  display: flex;
  flex-direction: column;
}

/* height: auto намеренно переопределяет фикс-высоту .content-container
   (450/500/540px), заданную внутри самих компонентов. */
.admin-page :deep(.content-container) {
  flex: 1 1 auto;
  min-height: 0;
  height: auto;
}

/* Шапка карточки не сжимается во flex-колонке: иначе высокий контент в теле
   (открытые детали мастер-детейла) выдавливал бы и плющил заголовок вместо
   внутреннего скролла тела (.table-body / .tab-content уже overflow:auto). */
.admin-page :deep(.management-header) {
  flex-shrink: 0;
}

/* --- Телефон: один скроллпорт на экран (#1097 волна 5) ----------------------
   Прокручивается страница, и только она. Высоту здесь не держим (её снимает
   applyHeight), карточку не растягиваем на всю высоту и внутренние области
   тела не делаем прокручиваемыми - иначе на экране оказывается два-три
   вложенных скроллпорта, и палец попадает то в один, то в другой.
   `!important` обязателен: правила-источники объявлены внутри самих
   компонентов управления, а те - lazy route-чанки, чей scoped-CSS грузится
   позже общего бандла (урок #1097 S9a). */
@media (max-width: 767.98px) {
  .admin-page {
    height: auto !important;
    max-height: none !important;
    overflow: visible !important;
    padding: var(--gutter, 12px);
  }

  .admin-page :deep(.dashboard-card) {
    height: auto !important;
    border-radius: var(--radius-lg, 20px);
  }

  .admin-page :deep(.content-container),
  .admin-page :deep(.table-body),
  .admin-page :deep(.tab-content) {
    height: auto !important;
    max-height: none !important;
    overflow: visible !important;
  }
}
</style>
