<script setup>
import { computed, onMounted, ref, onBeforeUnmount, nextTick } from 'vue';
import { useOnboardingStore } from '@/stores/onboarding';
import { useDeletionsStore } from '@/stores/deletions';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import OnboardingStartDialog from './OnboardingStartDialog.vue';
import { tourChapters } from './stepsFlow';

/**
 * Кнопка «Обучение» с выбором тура. Туров у одного человека может быть несколько
 * (принимающий с доступом к Админке), поэтому кнопка открывает список - но только
 * когда есть из чего выбирать: при единственном доступном туре клик запускает его
 * сразу, лишний шаг там ничего не даёт.
 *
 * Список строится на BaseDropdown (teleport-режим): у него уже есть переворот меню
 * вверх при нехватке места снизу, клампинг высоты по вьюпорту и компенсация
 * корневого zoom - на мобилке и на мониторах шире 1440 своё позиционирование
 * пришлось бы чинить теми же правками (#1150, #1255).
 */

const store = useOnboardingStore();
const notifications = useDeletionsStore();
const preparing = ref(false);
const selectedTour = ref(null);
const chapters = computed(() => {
  const steps = selectedTour.value ? store.stepsForTour(selectedTour.value) : [];
  return tourChapters(steps).filter(chapter => !steps[chapter.start]?.celebrate)
    .map(chapter => ({ id: steps[chapter.start].id, name: chapter.title }));
});
let mounted = true;
let dialogGeneration = 0;
onBeforeUnmount(() => { mounted = false; dialogGeneration += 1; });
function closeStartDialog() {
  dialogGeneration += 1;
  selectedTour.value = null;
  preparing.value = false;
}

const tours = computed(() => store.availableTours);
const canShow = computed(() => store.canShowTour && tours.value.length > 0);
const isSingle = computed(() => tours.value.length === 1);

/** @param {string} key ключ тура */
async function startTour(key) {
  if (!key || preparing.value) return;
  // Ждём гейтинг-контекст: у охраны в нём резолвится route фактовой таблицы, от
  // которого зависит длина тура. Без ожидания счётчик на первых шагах врёт.
  preparing.value = true;
  const generation = ++dialogGeneration;
  const identity = store.contextIdentity();
  try {
    const result = await store.prepareTour(key);
    if (!mounted || generation !== dialogGeneration || identity !== store.contextIdentity() || !store.canShowTour || result.status === 'stale') return;
    if (result.status !== 'ready') {
      notifications.notify({ type: 'error', prefix: 'Не удалось подготовить обучение. Попробуй ещё раз.' });
    } else selectedTour.value = key;
  } finally { if (mounted && generation === dialogGeneration) preparing.value = false; }
}

async function beginTour({ fromStepId, resume = false } = {}) {
  const key = selectedTour.value;
  if (!key || preparing.value) return;
  preparing.value = true;
  const generation = dialogGeneration;
  const identity = store.contextIdentity();
  try {
    const result = await store.prepareTour(key);
    if (!mounted || generation !== dialogGeneration || selectedTour.value !== key || identity !== store.contextIdentity() || !store.canShowTour || result.status === 'stale') return;
    if (result.status !== 'ready') throw new Error('Learning context not ready');
    selectedTour.value = null;
    await nextTick();
    if (!mounted || generation !== dialogGeneration || identity !== store.contextIdentity() || !store.canShowTour) return;
    if (!store.start({ tour: key, manual: true, restart: !resume, ...(fromStepId ? { fromStepId } : {}) })) {
      selectedTour.value = key;
      throw new Error('Learning entry no longer available');
    }
  } catch {
    if (mounted && generation === dialogGeneration && identity === store.contextIdentity()) notifications.notify({ type: 'error', prefix: 'Не удалось подготовить обучение. Попробуй ещё раз.' });
  } finally { if (mounted && generation === dialogGeneration) preparing.value = false; }
}

/**
 * Единственный доступный тур запускаем прямо с кнопки; иначе открываем список.
 *
 * @param {() => void} toggle тоггл меню от BaseDropdown
 */
async function onTriggerClick(toggle) {
  if (preparing.value) return;
  const identity = store.contextIdentity();
  preparing.value = true;
  const selection = await store.ensureSelectionContext();
  preparing.value = false;
  if (!mounted || identity !== store.contextIdentity() || selection.status === 'stale') return;
  if (selection.status !== 'ready') {
    notifications.notify({ type: 'error', prefix: 'Не удалось подготовить обучение. Попробуй ещё раз.' });
    return;
  }
  if (isSingle.value) {
    startTour(tours.value[0].key);
    return;
  }
  toggle();
}

/**
 * Метка состояния тура: пройден целиком либо пройден в прежней версии и с тех пор
 * дополнен. Непройденный тур метки не несёт - она означала бы «новый», а туры
 * новые все, пока их не прошли.
 *
 * @param {string} key
 * @returns {string} текст бейджа или пустая строка
 */
function badgeFor(key) {
  // Именно hasFinished, а не hasCompleted: закрытый на середине тур запись
  // прогресса тоже создаёт (она гасит автозапуск), и по ней бейдж утверждал бы,
  // что человек всё посмотрел.
  if (store.hasFinished(key)) return 'Пройден';
  if (store.isOutdated(key)) return 'Обновлён';
  // Прервался на середине - меню обещает продолжить с той же главы, а не начать
  // семь минут заново.
  if (store.hasProgress(key)) return 'Продолжить';
  return '';
}

// Права/роли определяют состав списка, пройденные версии - бейджи. Тянем при
// монтировании; все запросы идемпотентны (кэш и in-flight-гарды в сторах).
onMounted(() => {
  store.ensureSelectionContext();
  if (!store.statusLoaded) store.loadStatus();
});
</script>

<template>
  <BaseDropdown
    v-if="canShow"
    class="ob-menu"
    :model-value="null"
    :options="tours"
    label-key="title"
    value-key="key"
    teleport
    :menu-min-width="300"
    :menu-max-height="520"
    @update:model-value="startTour"
  >
    <template #trigger="{ toggle }">
      <button
        type="button"
        class="lk-button lk-button--secondary ob-start-button"
        data-testid="ob-start-button"
        :disabled="preparing"
        :aria-haspopup="isSingle ? undefined : 'menu'"
        @click="onTriggerClick(toggle)"
      >
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          aria-hidden="true"
        >
          <circle
            cx="12"
            cy="12"
            r="9"
            stroke="currentColor"
            stroke-width="1.8"
          />
          <path
            d="M9.5 9.2a2.6 2.6 0 1 1 3.6 2.4c-.7.3-1.1.7-1.1 1.5v.4"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
          />
          <circle
            cx="12"
            cy="16.6"
            r="1"
            fill="currentColor"
          />
        </svg>
        <span>Обучение</span>
      </button>
    </template>

    <template #option="{ option }">
      <span
        class="ob-menu__item"
        :data-testid="`ob-tour-${option.key}`"
      >
        <span class="ob-menu__head">
          <span class="ob-menu__title">{{ option.title }}</span>
          <span
            v-if="badgeFor(option.key)"
            class="ob-menu__badge"
            :class="{
              'ob-menu__badge--updated': badgeFor(option.key) === 'Обновлён',
              'ob-menu__badge--resume': badgeFor(option.key) === 'Продолжить',
            }"
          >{{ badgeFor(option.key) }}</span>
        </span>
        <span class="ob-menu__description">{{ option.description }}</span>
      </span>
    </template>
  </BaseDropdown>
  <OnboardingStartDialog
    :show="Boolean(selectedTour)"
    :busy="preparing"
    :chapters="chapters"
    :can-resume="Boolean(selectedTour && store.hasProgress(selectedTour) && !store.hasFinished(selectedTour))"
    @close="closeStartDialog"
    @start="beginTour"
    @resume="beginTour({ resume: true })"
  />
</template>

<style scoped>
/* Триггер - кнопка шапки «Обзора», а не поле ввода: гасим геометрию,
   которую BaseDropdown задаёт своей штатной кнопке. */
.ob-menu {
  display: inline-block;
  width: auto;
}

.ob-start-button {
  height: 25px;
  padding: 0 14px;
  font-size: 13px;
}

.ob-menu__item {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.ob-menu__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ob-menu__title {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

.ob-menu__description {
  font-size: 12px;
  line-height: 1.35;
  color: var(--text-muted);
  white-space: normal;
}

.ob-menu__badge {
  flex-shrink: 0;
  padding: 1px 8px;
  border-radius: 50px;
  font-size: 11px;
  font-weight: 500;
  background: var(--accent-tint);
  color: var(--accent-text);
}

/* «Обновлён» - предупреждение, а не факт: тур проходили, но он с тех пор дополнен. */
.ob-menu__badge--updated {
  background: var(--warning-bg);
  color: var(--warning-text);
}

/* «Продолжить» - приглашение вернуться, а не предупреждение: тон акцента. */
.ob-menu__badge--resume {
  background: var(--accent-tint);
  color: var(--accent);
}

/* Пункт с описанием - блочный: штатный item рассчитан на одну строку и разводит
   содержимое по краям (space-between). */
.ob-menu :deep(.base-dropdown__item) {
  justify-content: flex-start;
  padding: 10px 15px;
}
</style>
