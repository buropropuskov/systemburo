<script setup>
/**
 * Плавающая панель предупреждений по выбранным местам (#1183 polish).
 *
 * Одна панель справа-снизу агрегирует предупреждения всех выбранных мест/таблиц:
 * режим работы против окна пребывания срока (S5), свободный текст (S1), активные
 * окна (S4). Скрывается крестиком; при появлении НОВЫХ предупреждений (изменился
 * состав) показывается снова. Живо реагирует на быструю смену мест и времени -
 * данные приходят готовым реактивным `groups` из формы.
 */
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { pinLeavingElement } from '@/utils/listTransition';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { useUiStore } from '@/stores/ui';

const props = defineProps({
  /**
   * Группы предупреждений. Каждая:
   * { name, free: string|null, windows: string[], schedule: {presence, days:[{label,hours,open}], anyClosed}|null }
   */
  groups: {
    type: Array,
    default: () => [],
  },
  /**
   * Слой панели. По умолчанию ниже окон страницы, чтобы не закрывать их кнопки.
   * Из окна дополнения (#1685) панель показывается ПОВЕРХ него - иначе она уедет
   * под окно и предупреждение о расписании места просто не увидят.
   */
  zIndex: {
    type: Number,
    default: 990,
  },
});

/** Показываем только группы, где реально есть что сказать. */
const visibleGroups = computed(() =>
  (props.groups || []).filter(
    (g) => g && (g.free || (g.windows && g.windows.length) || (g.schedule && g.schedule.anyClosed)),
  ),
);

const dismissed = ref(false);

// На телефоне развёрнутый список занимал пол-экрана и закрывал форму - панель
// появляется невысокой плашкой (заголовок + счётчик), список раскрывается тапом.
// На десктопе места хватает, панель всегда развёрнута.
const { isNarrow } = useNarrowScreen();
const collapsed = ref(true);
const expanded = computed(() => !isNarrow.value || !collapsed.value);

function toggleCollapsed() {
  // Хвостовой click после реального жеста не должен разворачивать плашку: порядок
  // pointerup -> click браузер даёт всегда, поэтому гасим одноразовым флагом.
  if (suppressClick) {
    suppressClick = false;
    return;
  }
  if (isNarrow.value) collapsed.value = !collapsed.value;
}

/** Сигнатура состава - для решения "показать снова, если добавились новые". */
const signature = computed(() =>
  visibleGroups.value
    .map((g) => `${g.name}|${g.free || ''}|${(g.windows || []).join('~')}|${g.schedule ? g.schedule.presence + g.schedule.days.map((d) => d.label + (d.hours || []).join(',') + d.open).join('') : ''}`)
    .join('§'),
);

// Новый состав предупреждений возвращает панель, даже если её скрыли раньше.
watch(signature, (next, prev) => {
  if (next && next !== prev) {
    dismissed.value = false;
    slidingOut.value = false;
    dragX.value = 0;
  }
});

/* --- Смахивание вправо ------------------------------------------------------
 * Подсказка про режим работы места нужна один раз, и убирать её крестиком в
 * 26px на телефоне неудобно. Порог 90px и мёртвая зона 8px взяты те же, что у
 * свайпа-вниз модальных листов (`useSwipeDismiss`): жест по ощущению один и тот
 * же, отличается осью. Крестик остаётся - он единственный путь для мыши в теле
 * панели и для тех, кому жест недоступен.
 */
const SWIPE_THRESHOLD = 90;
const SWIPE_SLOP = 8;
const SLIDE_OUT_MS = 220;
/** Запас на leave-анимацию: инлайн-transform держим, пока панель не размонтируется. */
const LEAVE_MS = 320;

const dragX = ref(0);
const dragging = ref(false);
const slidingOut = ref(false);

let startX = 0;
let startY = 0;
let tracking = false;
let engaged = false;
let suppressClick = false;
let outTimer = null;
let resetTimer = null;

const panelStyle = computed(() => {
  const style = { zIndex: props.zIndex };
  if (slidingOut.value) {
    style.transform = 'translateX(115%)';
    style.opacity = 0;
  } else if (dragX.value > 0) {
    style.transform = `translateX(${dragX.value}px)`;
  }
  return style;
});

function reducedMotion() {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );
}

function cancelSwipe() {
  tracking = false;
  engaged = false;
  dragging.value = false;
  dragX.value = 0;
}

function clearTimers() {
  if (outTimer) { clearTimeout(outTimer); outTimer = null; }
  if (resetTimer) { clearTimeout(resetTimer); resetTimer = null; }
}

/** Доводим плашку за правый край и только потом снимаем - иначе она мигает на месте. */
function slideOut() {
  tracking = false;
  engaged = false;
  dragging.value = false;
  suppressClick = true;
  dragX.value = 0;

  if (reducedMotion()) {
    dismissed.value = true;
    return;
  }

  slidingOut.value = true;
  outTimer = setTimeout(() => {
    outTimer = null;
    dismissed.value = true;
    // Инлайн-transform снимаем уже после leave: класс leave-to задаёт свой
    // translateY, и сброс раньше времени дал бы второй, обратный слайд.
    resetTimer = setTimeout(() => {
      resetTimer = null;
      slidingOut.value = false;
    }, LEAVE_MS);
  }, SLIDE_OUT_MS);
}

function onPointerDown(e) {
  suppressClick = false;
  if (slidingOut.value) return;
  if (e.isPrimary === false) return;
  // Мышью панель тянется только за шапку: в теле горизонтальное движение мыши -
  // это выделение текста предупреждения, перехватывать его нельзя. Пальцем
  // тянется вся плашка.
  if (e.pointerType === 'mouse' && !e.target?.closest?.('.warn-panel__head')) return;
  startX = e.clientX;
  startY = e.clientY;
  tracking = true;
  engaged = false;
  dragX.value = 0;
}

function onPointerMove(e) {
  if (!tracking) return;
  const dx = e.clientX - startX;
  const dy = e.clientY - startY;

  if (!engaged) {
    // Мёртвая зона: на дребезге пальца preventDefault съел бы тап по крестику.
    if (Math.abs(dx) < SWIPE_SLOP && Math.abs(dy) < SWIPE_SLOP) return;
    // Вертикальный жест - это прокрутка страницы и списка предупреждений.
    if (Math.abs(dx) <= Math.abs(dy)) {
      cancelSwipe();
      return;
    }
    engaged = true;
    dragging.value = true;
    // Без захвата указателя движение за пределами плашки до нас не доходит.
    e.currentTarget?.setPointerCapture?.(e.pointerId);
  }

  if (dx <= 0) {
    dragX.value = 0;
    return;
  }
  dragX.value = dx;
  if (e.cancelable) e.preventDefault();
  if (dx > SWIPE_THRESHOLD) slideOut();
}

function onPointerUp() {
  if (!tracking) return;
  if (engaged) suppressClick = true;
  // Не дотянул до порога - плашка пружинит на место (transition вернёт её).
  cancelSwipe();
}

onBeforeUnmount(clearTimers);

const ui = useUiStore();

// Панель плавающая (position: fixed) и во время онбординга ложится прямо на
// подсвеченный блок сроков. Предупреждения при этом всё равно относятся к
// демо-вложению тура, а не к настоящей заявке, - прятать нечего жалеть.
const shown = computed(
  () => visibleGroups.value.length > 0 && !dismissed.value && !ui.tourActive,
);
</script>

<template>
  <Teleport to="body">
    <transition name="warn-panel">
      <aside
        v-if="shown"
        class="warn-panel"
        :class="{ 'warn-panel--collapsed': !expanded, 'is-dragging': dragging }"
        :style="panelStyle"
        data-testid="schedule-warning-panel"
        role="status"
        aria-live="polite"
        data-swipe-own
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerUp"
      >
        <header
          class="warn-panel__head"
          :class="{ 'warn-panel__head--tappable': isNarrow }"
          data-testid="schedule-warning-head"
          :aria-expanded="expanded ? 'true' : 'false'"
          @click="toggleCollapsed"
        >
          <span class="warn-panel__title">
            <span class="warn-panel__title-text">
              Предупреждение
              <span
                v-if="isNarrow"
                class="warn-panel__count"
                data-testid="schedule-warning-count"
              >{{ visibleGroups.length }}</span>
            </span>
            <!-- Жест сам себя не объявляет: без подписи про смахивание не догадаются. -->
            <span
              v-if="isNarrow"
              class="warn-panel__hint"
            >Смахните вправо, чтобы убрать</span>
          </span>
          <svg
            v-if="isNarrow"
            class="warn-panel__chevron"
            :class="{ 'warn-panel__chevron--up': !expanded }"
            width="12"
            height="12"
            viewBox="0 0 12 12"
            fill="none"
            aria-hidden="true"
          >
            <path
              d="M2 4.5 6 8.5l4-4"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
          <button
            type="button"
            class="warn-panel__close"
            aria-label="Скрыть предупреждения"
            data-testid="schedule-warning-close"
            @click.stop="dismissed = true"
          >
            <svg
              width="12"
              height="12"
              viewBox="0 0 14 14"
              fill="none"
              aria-hidden="true"
            >
              <path
                d="M13 1 1 13M1 1l12 12"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
              />
            </svg>
          </button>
        </header>

        <!-- Плавное раскрытие: grid-rows 0fr <-> 1fr (высота контента анимируется
             без magic-number, паттерн раскрывающихся списков сайдбара). -->
        <div
          class="warn-panel__reveal"
          :class="{ 'warn-panel__reveal--open': expanded }"
        >
          <div class="warn-panel__reveal-inner">
            <transition-group
              tag="div"
              name="warn-group"
              class="warn-panel__body"
              @before-leave="pinLeavingElement"
            >
              <section
                v-for="group in visibleGroups"
                :key="group.id || group.name"
                class="warn-group"
                :class="{ 'warn-group--rule': group.rule }"
              >
                <p class="warn-group__name">
                  {{ group.name }}
                </p>

                <!-- Режим работы против окна пребывания (S5) -->
                <div
                  v-if="group.schedule && group.schedule.anyClosed"
                  class="warn-schedule"
                >
                  <p class="warn-schedule__lead">
                    Режим работы · Вы указали <b>{{ group.schedule.presence }}</b>
                  </p>
                  <ul class="warn-schedule__days">
                    <li
                      v-for="day in group.schedule.days"
                      :key="day.label"
                      class="warn-day"
                      :class="{ 'warn-day--closed': !day.open }"
                    >
                      <span class="warn-day__name">{{ day.label }}</span>
                      <span class="warn-day__hours">
                        <span
                          v-for="(hour, hi) in day.hours"
                          :key="hi"
                          class="warn-day__hour"
                        >{{ hour }}</span>
                      </span>
                      <span
                        v-if="!day.open"
                        class="warn-day__badge"
                      >вне графика</span>
                    </li>
                  </ul>
                </div>

                <!-- Свободный текст (S1) + активные окна (S4) - обычным текстом -->
                <template v-if="group.free || (group.windows && group.windows.length)">
                  <p
                    v-if="group.free"
                    class="warn-note"
                  >
                    {{ group.free }}
                  </p>
                  <p
                    v-for="(win, wi) in group.windows"
                    :key="'w' + wi"
                    class="warn-note"
                  >
                    {{ win }}
                  </p>
                </template>
              </section>
            </transition-group>
          </div>
        </div>
      </aside>
    </transition>
  </Teleport>
</template>

<style scoped>
.warn-panel {
  position: fixed;
  right: 20px;
  bottom: 20px;
  /* Слой приходит инлайн-стилем (см. шаблон), а не v-bind() в этом блоке: панель
     уходит в Teleport, и переменная от v-bind до неё не доезжает - z-index молча
     становится auto, и панель проваливается под любой элемент с z-index: 1
     (sticky-шапка списка Т/С, ряд дат). */
  width: 360px;
  max-width: calc(100vw - 32px);
  max-height: calc(var(--app-vh, 1vh) * 60);
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border: 1px solid var(--color-border, var(--border));
  border-radius: var(--radius-lg, 20px);
  box-shadow: 0 12px 32px var(--shadow-drop);
  overflow: hidden;
  font-family: inherit;
  /* Смахивание вправо: анимируем только transform и opacity. pan-y оставляет
     браузеру вертикальную прокрутку (страницы и списка внутри панели), а
     горизонтальное движение достаётся обработчику. */
  transition: transform 0.22s ease, opacity 0.22s ease;
  touch-action: pan-y;
}

/* Пока тянут пальцем - панель идёт за ним 1:1, без сглаживания. */
.warn-panel.is-dragging {
  transition: none;
}

/* Свёрнутая плашка - это ТОЛЬКО жёлтая шапка, поэтому рамка панели уходит в тот же
   тон: с нейтральной рамкой жёлтый блок выглядел вставленным в чужую серую рамку. */
.warn-panel--collapsed {
  border-color: color-mix(in srgb, var(--warning) 42%, var(--surface));
}

.warn-panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  background: var(--warning-bg);
  /* Только нижняя граница: собственная рамка по всем сторонам дублировала рамку
     панели по бокам, а сверху срезалась её закруглением (overflow: hidden). */
  border-bottom: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.warn-panel__title {
  display: inline-flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  min-width: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--warning-text);
}

.warn-panel__title-text {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

/* Подпись про жест: тише заголовка, но своей строкой - в остаток она не влезает. */
.warn-panel__hint {
  font-size: 11px;
  font-weight: 500;
  opacity: 0.75;
}

.warn-panel__count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: var(--radius-pill, 999px);
  background: color-mix(in srgb, var(--warning) 18%, var(--surface));
  color: var(--warning-text);
  font-size: 11px;
  font-weight: 700;
}

.warn-panel__head--tappable {
  cursor: pointer;
  -webkit-tap-highlight-color: transparent;
}

/* Плавное раскрытие списка: высота контента через grid-rows 0fr <-> 1fr,
   inner с overflow:hidden обрезает контент в промежуточных кадрах. */
.warn-panel__reveal {
  display: grid;
  grid-template-rows: 1fr;
  min-height: 0;
}

.warn-panel__reveal-inner {
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.warn-panel__chevron {
  flex-shrink: 0;
  margin-left: auto;
  color: var(--warning-text);
  transition: transform 0.2s ease;
}

.warn-panel__chevron--up {
  transform: rotate(180deg);
}

.warn-panel__close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  padding: 0;
  border: none;
  border-radius: var(--radius-pill, 999px);
  background: transparent;
  color: var(--warning-text);
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.warn-panel__close:hover {
  background: color-mix(in srgb, var(--warning) 14%, var(--surface));
}

/* Полосу прокрутки не показываем: панель узкая, и полоса съедала её ширину,
   поджимая текст предупреждений. Прокрутка колесом и жестом остаётся. */
.warn-panel__body {
  position: relative;
  min-height: 0;
  padding: 8px 16px 14px;
  overflow-y: auto;
  scrollbar-width: none;
}

.warn-panel__body::-webkit-scrollbar {
  display: none;
}

.warn-group {
  padding: 12px 0;
  border-bottom: 1px solid var(--color-border, var(--border));
}

/* Правило подачи, а не режим работы места: своя подложка и полоса слева, иначе
   в общем списке оно читается как ещё одно место и теряется (#2320). */
.warn-group--rule {
  margin: 4px 0;
  padding: 12px;
  border: 1px solid color-mix(in srgb, var(--danger) 45%, var(--surface));
  border-left: 3px solid var(--danger);
  border-radius: 12px;
  background: color-mix(in srgb, var(--danger) 10%, var(--surface));
}

.warn-group--rule .warn-group__name {
  color: var(--danger);
}

.warn-group:last-child {
  border-bottom: none;
}

.warn-group__name {
  margin: 0 0 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text, var(--text));
}

.warn-schedule {
  padding: 8px 10px;
  background: var(--warning-bg);
  border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
  border-left: 3px solid var(--warning);
  border-radius: var(--radius-sm, 8px);
}

.warn-schedule__lead {
  margin: 0 0 6px;
  font-size: 12px;
  color: var(--warning-text);
}

.warn-schedule__days {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.warn-day {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 12px;
  color: var(--color-text, var(--text));
}

.warn-day__name {
  min-width: 54px;
  font-weight: 600;
}

.warn-day__hours {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  color: var(--text);
}

.warn-day__hour {
  display: block;
}

.warn-day--closed .warn-day__hours {
  color: var(--warning-text);
}

.warn-day__badge {
  flex-shrink: 0;
  padding: 1px 8px;
  border-radius: var(--radius-pill, 999px);
  background: var(--danger-bg);
  color: var(--danger-text);
  font-size: 10px;
  font-weight: 600;
}

.warn-note {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--text);
  line-height: 1.4;
}

.warn-panel-enter-active,
.warn-panel-leave-active {
  transition: transform 0.22s ease, opacity 0.22s ease;
}

.warn-panel-enter-from,
.warn-panel-leave-to {
  transform: translateY(12px);
  opacity: 0;
}

/* Появление/исчезновение каждого места по отдельности, соседи плавно смещаются. */
.warn-group-enter-active,
.warn-group-move {
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.warn-group-leave-active {
  position: absolute;
  left: 16px;
  right: 16px;
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.warn-group-enter-from,
.warn-group-leave-to {
  opacity: 0;
  transform: translateX(14px);
}

/* Смахивание при выключенных анимациях убирает плашку сразу (см. reducedMotion в
   скрипте), поэтому здесь гасим и переходы - иначе она уезжает вхолостую. */
@media (prefers-reduced-motion: reduce) {
  .warn-panel,
  .warn-panel-enter-active,
  .warn-panel-leave-active,
  .warn-group-enter-active,
  .warn-group-leave-active,
  .warn-group-move {
    transition: none;
  }
}

@media (max-width: 768px) {
  .warn-panel {
    right: 12px;
    left: 12px;
    bottom: 12px;
    width: auto;
    max-width: none;
    max-height: 52dvh;
  }

  /* Свёрнутая плашка раскрывается плавно; граница живёт на теле (border-top),
     чтобы уезжать под шапку вместе с контентом, а не мигать на самой шапке. */
  .warn-panel__head {
    border-bottom: none;
  }

  .warn-panel__body {
    border-top: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
  }

  .warn-panel__reveal {
    grid-template-rows: 0fr;
    transition: grid-template-rows 0.28s ease;
  }

  .warn-panel__reveal--open {
    grid-template-rows: 1fr;
  }
}
</style>
