<template>
  <Teleport to="body">
    <transition name="pdc-fade">
      <div
        v-if="active"
        class="pdc-overlay"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="pdc-title"
      >
        <div class="pdc-modal">
          <div class="pdc-modal__top">
            <h2
              id="pdc-title"
              class="pdc-modal__title"
            >
              Согласие на обработку персональных данных
            </h2>
            <p
              v-if="version"
              class="pdc-modal__version"
            >
              {{ versionLabel }}
            </p>
          </div>

          <div
            ref="columns"
            class="pdc-modal__columns"
            @scroll="updateProgress"
          >
            <div
              ref="scroller"
              class="pdc-modal__doc"
              tabindex="0"
              role="document"
              aria-label="Текст согласия на обработку персональных данных"
              @scroll="updateProgress"
            >
              <!-- Только sanitizeHtml: в system_settings текст лежит сырым. -->
              <div
                v-if="hasText"
                class="pdc-doc"
                v-html="safeHtml"
              />
              <p
                v-else
                class="pdc-doc__missing"
              >
                Текст согласия недоступен. Обновите страницу, а если сообщение осталось - обратитесь
                к администратору системы.
              </p>
              <div
                ref="sentinel"
                class="pdc-doc__end"
                aria-hidden="true"
              />
            </div>

            <!-- Пояснение своими словами: сам документ юридический, и по нему трудно
               понять, что именно происходит с данными конкретного человека. -->
            <aside class="pdc-modal__aside">
              <h3 class="pdc-aside__title">
                Коротко о главном
              </h3>
              <p class="pdc-aside__text">
                Система обрабатывает данные вашей учётной записи: фамилию, имя и отчество,
                организацию и должность, рабочие контакты, а также сведения о входах и
                действиях в системе.
              </p>
              <p class="pdc-aside__text">
                Эти данные нужны, чтобы оформлять и согласовывать заявки на проход, показывать
                вас коллегам как участника заявки и вести учёт доступа на территорию.
              </p>
              <p class="pdc-aside__note">
                Отозвать согласие можно в личном кабинете или через бюро пропусков. После
                отзыва система снова покажет это окно, и до нового подтверждения работать в
                ней не получится.
              </p>
            </aside>
          </div>

          <div class="pdc-modal__foot">
            <div
              class="pdc-progress"
              role="progressbar"
              aria-label="Прочитано"
              aria-valuemin="0"
              aria-valuemax="100"
              :aria-valuenow="progressPercent"
            >
              <div
                class="pdc-progress__fill"
                :style="{ transform: `scaleX(${progress})` }"
              />
            </div>
            <p class="pdc-hint">
              {{ hint }}
            </p>

            <label
              class="pdc-agree"
              :class="{ 'pdc-agree--locked': !canAgree }"
            >
              <input
                v-model="agreed"
                type="checkbox"
                class="pdc-agree__box"
                :disabled="!canAgree"
                data-testid="pdc-agree"
              >
              <span>Я прочитал документ и даю согласие на обработку персональных данных</span>
            </label>

            <div class="pdc-actions">
              <button
                type="button"
                class="pdc-btn pdc-btn--primary"
                :disabled="!canAccept"
                data-testid="pdc-accept"
                @click="submit"
              >
                {{ busy ? 'Подтверждаем...' : 'Подтверждаю согласие' }}
              </button>
              <button
                v-if="hasDocument"
                type="button"
                class="pdc-btn"
                :disabled="downloading"
                data-testid="pdc-download"
                @click="download"
              >
                {{ downloading ? 'Загружаем...' : 'Скачать документ' }}
              </button>
              <button
                type="button"
                class="pdc-btn pdc-btn--ghost"
                data-testid="pdc-logout"
                @click="emit('logout')"
              >
                Выйти
              </button>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { usePDConsentStore } from '@/stores/pdConsent';
import { useDeletionsStore } from '@/stores/deletions';
import { sanitizeHtml } from '@/utils/sanitize';
import { downloadDataProcessingDoc } from '@/api/dataProcessing';
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { formatMomentDate } from '@/utils/datetime';

/**
 * Неснимаемое окно согласия на обработку персональных данных (#1567). Написано
 * своей вёрсткой, а не на BaseModal: тот пропускает свайп-закрытие даже при
 * `closable: false` (гвард смотрит только на `sheetSwipe`), а согласие смахнуть
 * пальцем нельзя. Из окна есть два честных выхода - подтвердить или выйти из
 * системы; закрытия по оверлею, крестику и Escape нет намеренно.
 *
 * Показом управляет App.vue (`consentBlocking`): он знает и про маршрут, и про
 * бан, и про супер-администратора. Компонент остаётся смонтированным всегда,
 * поэтому у окна отыгрывают обе анимации - и появления, и скрытия.
 */
const props = defineProps({
  active: { type: Boolean, default: false },
});

const emit = defineEmits(['logout']);

const store = usePDConsentStore();

const scroller = ref(null);
// На узком экране прокручивается уже столбец целиком, а не область текста:
// прогресс обязан считаться по тому, что реально скроллится, иначе полоса
// показывает 100% с первого кадра.
const columns = ref(null);
const sentinel = ref(null);
// Документ дочитан. Значение монотонное: один раз доскроллив до конца,
// пользователь не теряет право согласиться, случайно прокрутив текст вверх.
const atEnd = ref(false);
const agreed = ref(false);
const progress = ref(0);
const busy = ref(false);
const downloading = ref(false);

let io = null;
let ro = null;
// Ключ владельца общей блокировки прокрутки фона (utils/bodyScrollLock): окна
// живут стопкой, и каждое снимает блокировку только за себя.
const lockOwner = {};

const version = computed(() => store.version);
/**
 * Подпись редакции. Номер сам по себе человеку ничего не говорит - дата говорит,
 * с какого числа действует то, что ему показывают. Без даты (настройки заведены
 * до появления поля) остаётся один номер.
 */
const versionLabel = computed(() => {
  const at = store.versionAt ? new Date(store.versionAt) : null;
  if (!at || Number.isNaN(at.getTime())) return `Редакция ${store.version}`;
  return `Редакция ${store.version} от ${formatMomentDate(at)}`;
});
const hasText = computed(() => Boolean(store.html));
const hasDocument = computed(() => Boolean(store.docMeta?.stored_name));
// В system_settings HTML лежит сырым - это единственная точка его рендера, и она
// обязана быть санитизированной, иначе редактор текста превращается в stored XSS
// на каждого пользователя при каждом входе.
const safeHtml = computed(() => sanitizeHtml(store.html));

const progressPercent = computed(() => Math.round(progress.value * 100));
const canAgree = computed(() => hasText.value && atEnd.value);
const canAccept = computed(() => canAgree.value && agreed.value && !busy.value);

const hint = computed(() => {
  if (!hasText.value) return 'Текст согласия не загружен';
  if (!atEnd.value) return 'Прокрутите документ до конца';
  if (!agreed.value) return 'Отметьте согласие с условиями';
  return 'Можно подтверждать';
});

function activeScroller() {
  const doc = scroller.value;
  if (doc && doc.scrollHeight > doc.clientHeight + 1) return doc;
  return columns.value || doc;
}

function updateProgress() {
  const el = activeScroller();
  if (!el) return;
  const max = el.scrollHeight - el.clientHeight;
  if (max <= 1) {
    progress.value = 1;
    return;
  }
  progress.value = Math.min(1, Math.max(0, el.scrollTop / max));
}

function observe() {
  const end = sentinel.value;
  if (!end) return;

  if (typeof IntersectionObserver === 'undefined') {
    // Наблюдателя нет (старый браузер) - не запираем пользователя в окне без
    // возможности согласиться. Галочка при этом остаётся обязательной.
    atEnd.value = true;
  } else {
    // root: null - пересечение считаем с вьюпортом, а не арифметикой
    // scrollTop + clientHeight: скроллер разный на десктопе и мобилке, а
    // корневой zoom (utils/viewportScale.js) разводит единицы rect и innerHeight.
    io = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) atEnd.value = true;
      },
      { root: null, threshold: 0.01 },
    );
    io.observe(end);
  }

  // Картинки и подгружаемые шрифты меняют высоту документа уже после первого
  // кадра: без пересчёта полоса прочтения врёт (сначала 100%, потом откат).
  if (typeof ResizeObserver !== 'undefined' && scroller.value) {
    ro = new ResizeObserver(updateProgress);
    ro.observe(scroller.value);
  }
  if (typeof document !== 'undefined' && document.fonts?.ready) {
    document.fonts.ready.then(updateProgress).catch(() => {});
  }
  updateProgress();
}

function unobserve() {
  io?.disconnect();
  io = null;
  ro?.disconnect();
  ro = null;
}

watch(
  () => props.active,
  async (open) => {
    setBodyScrollLock(lockOwner, open);
    if (!open) {
      unobserve();
      atEnd.value = false;
      agreed.value = false;
      progress.value = 0;
      return;
    }
    await nextTick();
    observe();
    // Фокус на текст: с клавиатуры документ должен листаться сразу, без Tab.
    scroller.value?.focus?.();
  },
  { immediate: true },
);

// Смена редакции при открытом окне (текст подняли в другой вкладке) - читать
// заново: прежний доскролл относился к другому документу.
watch(
  () => store.version,
  () => {
    if (!props.active) return;
    atEnd.value = false;
    agreed.value = false;
    progress.value = 0;
    unobserve();
    nextTick(observe);
  },
);

onBeforeUnmount(() => {
  unobserve();
  releaseBodyScrollLock(lockOwner);
});

async function submit() {
  if (!canAccept.value) return;
  busy.value = true;
  try {
    await store.accept();
  } catch (e) {
    useDeletionsStore().notify({
      type: 'error',
      prefix: 'Не удалось подтвердить согласие: ',
      bold: e?.message || 'ошибка сети',
    });
  } finally {
    busy.value = false;
  }
}

async function download() {
  downloading.value = true;
  try {
    await downloadDataProcessingDoc(store.docMeta?.file_name);
  } catch (e) {
    useDeletionsStore().notify({
      type: 'error',
      prefix: 'Не удалось скачать документ: ',
      bold: e?.message || 'ошибка сети',
    });
  } finally {
    downloading.value = false;
  }
}
</script>

<style scoped>
.pdc-overlay {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  /* Между сессией (25000) и баном (26000): забаненному показываем блокировку,
     а не согласие - согласиться он всё равно не сможет, серверный гейт бана
     стоит раньше гейта согласия. Ниже тоста (29000). */
  z-index: 25500;
  background: rgba(20, 24, 33, 0.62);
  backdrop-filter: blur(3px);
  -webkit-backdrop-filter: blur(3px);
}

.pdc-modal {
  display: flex;
  flex-direction: column;
  width: 880px;
  max-width: 94vw;
  /* --app-vh, а не vh: под корневым zoom и с мобильной адресной строкой
     единицы vh врут и окно уезжает за экран (#1359). */
  max-height: calc(var(--app-vh, 1vh) * 88);
  background: var(--surface);
  border-radius: 30px;
  overflow: hidden;
  box-shadow: 0 30px 80px rgba(10, 14, 24, 0.45);
}

/* Шапка пастельная, а не заливка акцентом: окно видит каждый работник при первом
   входе, и ярко-синяя плашка во весь верх читается тревожно. Оттенок берётся от
   акцента темы, поэтому шапка остаётся согласованной с остальным интерфейсом. */
.pdc-modal__top {
  padding: 24px 28px 20px;
  color: var(--color-text, var(--text));
  background: color-mix(in srgb, var(--accent) 12%, var(--surface));
  border-bottom: 1px solid color-mix(in srgb, var(--accent) 22%, var(--border));
}

.pdc-modal__title {
  font-size: 20px;
  font-weight: 700;
}

.pdc-modal__subtitle {
  margin-top: 8px;
  font-size: 13px;
  font-weight: 500;
  line-height: 1.5;
  opacity: 0.92;
}

.pdc-modal__version {
  margin-top: 10px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--color-text-muted, var(--text-muted));
}

/* Две колонки: слева документ, справа пояснение своими словами. На узком экране
   пояснение уходит вниз и не отбирает место у текста, который надо прочитать. */
.pdc-modal__columns {
  display: flex;
  flex: 1;
  min-height: 0;
  gap: 0;
}

.pdc-modal__aside {
  flex: 0 0 272px;
  min-width: 0;
  padding: 22px 26px;
  border-left: 1px solid var(--border);
  background: color-mix(in srgb, var(--accent) 5%, var(--surface));
  overflow-y: auto;
}

.pdc-aside__title {
  margin: 0 0 10px;
  font-size: 14px;
  font-weight: 700;
  color: var(--color-text, var(--text));
}

.pdc-aside__text {
  margin: 0 0 10px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-muted, var(--text-muted));
}

.pdc-aside__note {
  margin: 14px 0 0;
  padding-top: 12px;
  border-top: 1px solid var(--border);
  font-size: 12px;
  line-height: 1.5;
  color: var(--color-text-muted, var(--text-muted));
}

.pdc-modal__doc {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  /* Прокрутка не уходит на фон, когда текст дочитан до края. */
  overscroll-behavior: contain;
  padding: 22px 30px;
  scrollbar-gutter: stable;
}

.pdc-modal__doc:focus-visible {
  outline: 2px solid var(--accent-text);
  outline-offset: -4px;
}

.pdc-doc {
  font-size: 14px;
  line-height: 1.65;
  color: var(--text);
  overflow-wrap: anywhere;
}

.pdc-doc :deep(p) {
  margin: 0 0 10px;
}

.pdc-doc :deep(h1),
.pdc-doc :deep(h2),
.pdc-doc :deep(h3) {
  margin: 18px 0 10px;
  line-height: 1.35;
}

.pdc-doc :deep(ul),
.pdc-doc :deep(ol) {
  margin: 0 0 10px;
  padding-left: 22px;
}

.pdc-doc :deep(img) {
  max-width: 100%;
  height: auto;
}

.pdc-doc :deep(table) {
  width: 100%;
  border-collapse: collapse;
}

.pdc-doc :deep(td),
.pdc-doc :deep(th) {
  border: 1px solid var(--border);
  padding: 6px 8px;
}

.pdc-doc__missing {
  font-size: 14px;
  line-height: 1.6;
  color: var(--danger-text);
}

/* Метка конца текста для наблюдателя. Высота ненулевая: элемент нулевой высоты
   IntersectionObserver в части браузеров не считает пересекающимся. */
.pdc-doc__end {
  height: 1px;
}

.pdc-modal__foot {
  padding: 18px 30px 24px;
  border-top: 1px solid var(--border);
  background: var(--surface);
}

.pdc-progress {
  height: 4px;
  border-radius: 999px;
  overflow: hidden;
  /* Приглушённая дорожка: на акцентном оттенке пустая полоса читалась как
     заполненная, и «ничего не прочитано» выглядело как «прочитано всё». */
  background: color-mix(in srgb, var(--color-text, var(--text)) 14%, transparent);
}

.pdc-progress__fill {
  height: 100%;
  background: var(--accent);
  transform: scaleX(0);
  transform-origin: left center;
  transition: transform 0.15s linear;
}

.pdc-hint {
  margin-top: 10px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
}

.pdc-agree {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-top: 14px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--text);
  cursor: pointer;
}

.pdc-agree--locked {
  cursor: default;
  color: var(--text-muted);
}

.pdc-agree__box {
  width: 18px;
  height: 18px;
  margin-top: 1px;
  flex: 0 0 auto;
  accent-color: var(--accent);
  cursor: inherit;
}

.pdc-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 18px;
}

.pdc-btn {
  padding: 10px 20px;
  border: 1px solid var(--border);
  border-radius: var(--radius-pill, 999px);
  background: transparent;
  color: var(--text);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s ease, border-color 0.2s ease, opacity 0.2s ease;
}

.pdc-btn:hover:not(:disabled) {
  background: var(--accent-tint);
}

.pdc-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.pdc-btn--primary {
  flex: 1;
  min-width: 220px;
  border-color: transparent;
  background: var(--accent);
  color: var(--accent-contrast);
  font-weight: 600;
}

.pdc-btn--primary:hover:not(:disabled) {
  background: color-mix(in srgb, var(--accent) 88%, var(--text));
}

.pdc-btn--ghost {
  color: var(--text-muted);
}

/* Анимация: только transform/opacity (rules/web). */
.pdc-fade-enter-active {
  transition: opacity 0.25s ease-out;
}
.pdc-fade-leave-active {
  transition: opacity 0.2s ease-in;
}
.pdc-fade-enter-from,
.pdc-fade-leave-to {
  opacity: 0;
}
.pdc-fade-enter-active .pdc-modal {
  transition: transform 0.25s ease-out;
}
.pdc-fade-enter-from .pdc-modal {
  transform: translateY(16px);
}

/* Мобильный лист по эталону (.claude/ui-etalon, 3.2), с двумя отличиями:
   ползунка свайпа нет - окно намеренно не смахивается, и backdrop-filter снят
   (форсит compositing-слой и роняет кадры при слайде листа). */
@media (max-width: 768px) {
  .pdc-overlay {
    padding: 0;
    align-items: flex-end;
    top: 0;
    height: 100dvh;
    bottom: auto;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  .pdc-modal {
    width: 100%;
    max-width: 100%;
    /* 90dvh, а не app-vh*100: композитор держит высоту по видимой области, и
       лист не выезжает за кромку экрана на 3-4px (замер на 390x844). */
    max-height: 90dvh;
    border-radius: 20px 20px 0 0;
  }

  .pdc-modal__top {
    padding: 22px 20px 18px;
  }

  .pdc-modal__columns {
    flex-direction: column;
    overflow-y: auto;
  }

  .pdc-modal__doc {
    padding: 18px 20px;
    /* В колонке прокручивается уже сам столбец - иначе получаются две вложенные
       прокрутки и текст ловится пальцем через раз. */
    flex: none;
    overflow: visible;
  }

  .pdc-modal__aside {
    flex: none;
    border-left: none;
    border-top: 1px solid var(--border);
    padding: 18px 20px;
    overflow: visible;
  }

  .pdc-modal__foot {
    padding: 16px 20px 20px;
  }

  .pdc-btn {
    width: 100%;
  }

  .pdc-btn--primary {
    min-width: 0;
  }
}
</style>
