<template>
  <teleport to="body">
    <transition name="gsp-slide">
      <aside
        v-if="open"
        ref="panel"
        class="gsp"
        :class="{ 'gsp--collapsed': collapsed }"
        data-testid="global-search-panel"
        role="complementary"
        aria-label="Результаты поиска"
      >
        <button
          v-if="collapsed"
          class="gsp__strip"
          type="button"
          title="Развернуть результаты поиска"
          aria-label="Развернуть результаты поиска"
          @click="collapsed = false"
        >
          <NavIcon
            name="search"
            :size="18"
          />
          <span
            v-if="totalFound"
            class="gsp__strip-count"
          >{{ totalFound }}</span>
        </button>

        <header
          v-if="!collapsed"
          class="gsp__head"
        >
          <NavIcon
            name="search"
            :size="18"
            class="gsp__head-icon"
          />
          <input
            ref="input"
            :value="query"
            class="gsp__input"
            type="text"
            placeholder="Поиск по системе"
            aria-label="Поиск по системе"
            autocomplete="off"
            @input="$emit('update:query', $event.target.value)"
            @keydown.down.prevent="move(1)"
            @keydown.up.prevent="move(-1)"
            @keydown.enter.prevent="openActive"
          >
          <button
            class="gsp__act gsp__act--pin"
            :class="{ 'is-pinned': pinned }"
            type="button"
            :title="pinned ? 'Открепить панель' : 'Закрепить раскрытой'"
            :aria-label="pinned ? 'Открепить панель' : 'Закрепить панель'"
            :aria-pressed="pinned"
            @click="togglePinned"
          >
            <!-- Тот же глиф и то же состояние, что у закрепления рельса в навигации:
                 одна кнопка в двух местах интерфейса должна выглядеть одинаково. -->
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.7"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <path d="M9 4h6l-1 7 3 3v2H7v-2l3-3z" />
              <line
                x1="12"
                y1="16"
                x2="12"
                y2="21"
              />
            </svg>
          </button>
          <button
            class="gsp__act"
            type="button"
            title="Свернуть в столбик"
            aria-label="Свернуть в столбик"
            @click="collapsed = true"
          >
            <NavIcon
              name="collapse-right"
              :size="17"
            />
          </button>
          <button
            class="gsp__act"
            type="button"
            title="Закрыть поиск и очистить запрос"
            aria-label="Закрыть результаты"
            @click="close"
          >
            <NavIcon
              name="close"
              :size="17"
            />
          </button>
        </header>

        <div
          v-if="!collapsed"
          class="gsp__body"
        >
          <p
            v-if="degradedLabels.length"
            class="gsp__degraded"
          >
            Не удалось опросить: {{ degradedLabels.join(', ') }}. Показано остальное.
          </p>

          <div
            v-for="group in visibleGroups"
            :key="group.type"
            class="gsp__group"
          >
            <div class="gsp__group-title">
              {{ group.title }}
              <span
                v-if="group.total > 1"
                class="gsp__group-count"
              >{{ group.total }}</span>
            </div>
            <button
              v-for="item in group.items"
              :key="group.type + '-' + item.key"
              type="button"
              class="gsp__row"
              :class="{ 'gsp__row--active': item.index === activeIndex }"
              @click="openItem(item)"
              @mousemove="activeIndex = item.index"
            >
              <NavIcon
                :name="item.icon"
                :size="18"
                class="gsp__row-icon"
              />
              <span class="gsp__row-text">
                <span class="gsp__row-title">{{ item.title }}</span>
                <span
                  v-if="item.subtitle"
                  class="gsp__row-subtitle"
                >{{ item.subtitle }}</span>
              </span>
            </button>
            <button
              v-if="group.hidden > 0"
              type="button"
              class="gsp__more"
              data-testid="global-search-expand"
              @click="expanded[group.type] = true"
            >
              Показать ещё {{ group.hidden }}
            </button>
          </div>

          <div
            v-if="loading"
            class="gsp__group"
          >
            <SkeletonLine
              v-for="n in 3"
              :key="n"
              class="gsp__skeleton"
            />
          </div>

          <p
            v-else-if="failed"
            class="gsp__hint"
          >
            Поиск сейчас недоступен. Попробуйте ещё раз.
          </p>
          <p
            v-else-if="!query.trim()"
            class="gsp__hint"
          >
            Человек, машина, заявка, раздел или действие -- начните вводить
          </p>
          <p
            v-else-if="tooShort"
            class="gsp__hint"
          >
            Введите хотя бы {{ MIN_QUERY_LENGTH }} символа
          </p>
          <p
            v-else-if="nothingFound"
            class="gsp__hint"
          >
            Ничего не найдено по запросу «{{ query.trim() }}»
          </p>
        </div>
      </aside>
    </transition>
  </teleport>
</template>

<script>
import NavIcon from '@/components/icons/NavIcon.vue';
import SkeletonLine from '@/components/ui/SkeletonLine.vue';
import { usePermission } from '@/composables/usePermission';
import { useGlobalSearch, MIN_QUERY_LENGTH } from '@/composables/useGlobalSearch';
import { ADMIN_GROUPS, MAIN_SECTIONS } from '@/constants/navSections';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import { SEARCH_TARGETS } from '@/constants/searchTargets';

/** Сколько строк раздела видно до раскрытия. Пять - столько, чтобы читалось разом. */
const GROUP_PREVIEW = 5;
import { SEARCH_ACTIONS } from '@/constants/searchActions';
import { useOnboardingStore } from '@/stores/onboarding';

/** Сколько разделов меню показывать: список длинный, а нужен обычно первый же. */
const SECTIONS_LIMIT = 5;
const PIN_STORAGE_KEY = 'globalSearch.pinned';

export default {
  name: 'GlobalSearchPanel',
  components: { NavIcon, SkeletonLine },
  props: {
    /** Открыта ли панель. Открывается кнопкой в шапке, до всякого ввода. */
    show: { type: Boolean, default: false },
    /** Строка поиска. Живёт снаружи, чтобы переживать сворачивание панели. */
    query: { type: String, default: '' },
  },
  emits: ['close', 'update:query'],
  setup() {
    const { can } = usePermission();
    const search = useGlobalSearch();
    return { can, ...search, MIN_QUERY_LENGTH };
  },
  data() {
    return {
      // Какие разделы раскрыты целиком; новый запрос сворачивает всё обратно.
      expanded: {},
      activeIndex: 0,
      // Закрепление переживает перезагрузку: это привычка работы, а не состояние
      // одного захода -- каждый раз закреплять заново раздражало бы.
      pinned: localStorage.getItem(PIN_STORAGE_KEY) === '1',
      collapsed: false,
    };
  },
  computed: {
    /**
     * Панель открыта по явному признаку, а не по наличию текста: её открывают кнопкой
     * в шапке и сразу видят, что можно сделать, ещё не набрав ни буквы.
     */
    open() {
      return this.show;
    },
    /**
     * Быстрые действия -- первая группа: «подать» или «отправить» это намерение, и
     * предложить его надо раньше, чем список мест, где слово встречается. Ищем и по
     * названию, и по обиходным словам, до которых человек доходит раньше официальных.
     */
    actionItems() {
      const raw = this.query.trim();
      if (!raw) return [];
      const variants = buildSearchVariants(raw);

      return SEARCH_ACTIONS
        .filter((a) => !a.permission || this.can(a.permission))
        .filter((a) => matchesSearch(a.label, variants)
          || a.keywords.some((k) => matchesSearch(k, variants)))
        .map((a) => ({
          key: a.key,
          title: a.label,
          subtitle: a.hint,
          icon: a.icon,
          to: a.to,
        }));
    },
    /**
     * Разделы меню -- вторая группа. Ради неё поиск и затевался: чаще всего человек не
     * помнит, в каком разделе искать.
     */
    sectionItems() {
      const raw = this.query.trim();
      if (!raw) return [];
      const variants = buildSearchVariants(raw);

      const all = [...MAIN_SECTIONS, ...ADMIN_GROUPS.flatMap((g) => g.items)];
      return all
        .filter((s) => (!s.permission || this.can(s.permission))
          && (matchesSearch(s.label, variants)
            || (s.keywords || []).some((k) => matchesSearch(k, variants))))
        .slice(0, SECTIONS_LIMIT)
        .map((s) => ({
          // Вкладка внутри страницы делит путь с родителем, поэтому ключ строки
          // собирается вместе с параметрами - иначе два пункта дали бы один key.
          key: s.query ? `${s.path}?${new URLSearchParams(s.query)}` : s.path,
          title: s.label,
          subtitle: '',
          icon: s.icon,
          to: s.query ? { path: s.path, query: s.query } : { path: s.path },
        }));
    },
    /**
     * Группы со сквозной нумерацией строк: стрелки ходят сквозь заголовки, поэтому
     * индекс проставляется один раз здесь, а не пересчитывается на каждое нажатие.
     */
    visibleGroups() {
      const groups = [];
      let index = 0;

      // Раздел показывается свёрнутым до GROUP_PREVIEW строк: без этого выдача из
      // нескольких разделов превращается в простыню, где ничего не найти. Остальное
      // раскрывается на месте - уходить со страницы за своими же результатами незачем.
      const push = (type, title, all) => {
        if (!all.length) return;
        const open = this.expanded[type];
        const shown = open ? all : all.slice(0, GROUP_PREVIEW);
        groups.push({
          type,
          title,
          total: all.length,
          hidden: all.length - shown.length,
          items: shown.map((it) => ({ ...it, index: index++ })),
        });
      };

      push('actions', 'Действия', this.actionItems);
      push('sections', 'Разделы', this.sectionItems);
      for (const g of this.groups) {
        const items = (g.items || []).map((it) => ({
          key: `${it.type}-${it.id}`,
          title: it.title,
          subtitle: it.subtitle,
          icon: SEARCH_TARGETS[it.target?.entity]?.icon || 'search',
          to: this.routeFor(it),
        }));
        push(g.type, g.title, items);
      }
      return groups;
    },
    flatItems() {
      return this.visibleGroups.flatMap((g) => g.items);
    },
    /** Число найденного для свёрнутого столбика: иначе он не сообщает ничего. */
    totalFound() {
      return this.flatItems.length;
    },
    tooShort() {
      const len = this.query.trim().length;
      return len > 0 && len < MIN_QUERY_LENGTH && this.sectionItems.length === 0;
    },
    nothingFound() {
      return this.query.trim().length >= MIN_QUERY_LENGTH && this.flatItems.length === 0;
    },
    /** Разделы, которые не ответили: молчать нельзя -- пусто и «нет данных» читаются одинаково. */
    degradedLabels() {
      return (this.degraded || []).map((t) => SEARCH_TARGETS.groupTitles?.[t] || t);
    },
  },
  watch: {
    show(val) {
      if (val) {
        this.collapsed = false;
        this.$nextTick(() => this.$refs.input?.focus());
      }
    },
    collapsed(val) {
      if (!val) this.$nextTick(() => this.$refs.input?.focus());
    },
    query: {
      immediate: true,
      handler(val) {
        this.activeIndex = 0;
        // Новый запрос - новые разделы: раскрытые сворачиваем обратно.
        this.expanded = {};
        // Новый запрос разворачивает столбик: искать со свёрнутой панелью бессмысленно.
        if (val.trim()) this.collapsed = false;
        this.search(val);
      },
    },
  },
  mounted() {
    // Capture на document: меню слушает Escape на window, и без перехвата он закрыл бы
    // заодно выдвижное меню под панелью.
    document.addEventListener('keydown', this.onKeydownCapture, true);
    document.addEventListener('mousedown', this.onDocumentMousedown);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeydownCapture, true);
    document.removeEventListener('mousedown', this.onDocumentMousedown);
  },
  methods: {
    onKeydownCapture(e) {
      if (!this.open) return;
      if (e.key === 'Escape') {
        e.stopPropagation();
        e.preventDefault();
        // Escape сворачивает, а не закрывает: результаты и запрос остаются, и вернуться
        // к ним можно одним нажатием. Совсем убрать панель -- крестиком.
        if (!this.collapsed) this.collapsed = true;
        return;
      }

    },
    /**
     * Клик мимо панели и мимо поля поиска сворачивает её в столбик. Закреплённую не
     * трогаем: её для того и закрепляют, чтобы работать со страницей рядом.
     *
     * Именно сворачивает, а не закрывает: запрос и найденное сохраняются, вернуться к
     * ним можно одним нажатием. Закрытие -- только явное, крестиком.
     */
    onDocumentMousedown(e) {
      if (!this.open || this.pinned || this.collapsed) return;
      // Панель раскрыл онбординг-тур: шаг рассказывает именно про открытый поиск,
      // а окно шага лежит вне панели. Без этой проверки клик по окну сворачивал
      // панель в столбик, подсветка слетала, а вырез в затемнении оставался
      // висеть на опустевшем месте - шаг превращался в мёртвый.
      if (useOnboardingStore().revealOpen === 'search-panel') return;
      if (this.$refs.panel?.contains(e.target)) return;
      if (e.target.closest?.('.search-btn')) return;
      this.collapsed = true;
    },
    move(delta) {
      const total = this.flatItems.length;
      if (!total) return;
      // Без зацикливания: в длинном списке прыжок с конца в начало дезориентирует.
      this.activeIndex = Math.min(Math.max(this.activeIndex + delta, 0), total - 1);
      this.$nextTick(() => {
        this.$refs.panel?.querySelector('.gsp__row--active')?.scrollIntoView({ block: 'nearest' });
      });
    },
    openActive() {
      const item = this.flatItems[this.activeIndex];
      if (item) this.openItem(item);
    },
    /** Куда ведёт результат: маршруты знает фронт, сервер отдаёт сущность и её номер. */
    routeFor(item) {
      const target = SEARCH_TARGETS[item.target?.entity];
      if (!target) return null;
      return target.route(item.target.id, this.query.trim());
    },
    openItem(item) {
      if (!item.to) return;
      // Закреплённая панель остаётся развёрнутой: по одному запросу часто открывают
      // несколько находок подряд. Незакреплённая сворачивается в столбик -- уступает
      // место странице, но помнит запрос и найденное.
      if (!this.pinned) this.collapsed = true;
      this.$nextTick(() => this.$router.push(item.to));
    },
    togglePinned() {
      this.pinned = !this.pinned;
      localStorage.setItem(PIN_STORAGE_KEY, this.pinned ? '1' : '0');
    },
    close() {
      this.cancel();
      this.$emit('close');
    },
  },
};
</script>

<style scoped>
.gsp {
  position: fixed;
  /* Во всю высоту окна. Шапка приложения прокручивается вместе со страницей, поэтому
     панель, начинавшаяся под ней, при прокрутке оставляла сверху пустую полосу.
     Кнопки шапки под панелью не теряются: она открыта, только пока идёт поиск. */
  top: 0;
  right: 0;
  bottom: 0;
  width: 420px;
  max-width: 100vw;
  display: flex;
  flex-direction: column;
  /* Полупрозрачная: страница под панелью остаётся видна, поэтому переход по находке
     не ощущается как уход «вслепую». */
  background: color-mix(in srgb, var(--surface) 85%, transparent);
  border-left: 1px solid var(--border);
  box-shadow: -8px 0 24px rgb(0 0 0 / 12%);
  /* Выше выдвижного меню и карточек, ниже блокирующих окон: подтверждение о
     несохранённой форме и сообщение о блокировке должны перекрывать результаты. */
  z-index: 15000;
  transition: width 0.2s ease-out;
}

/* Лёгкое размытие подложки -- только на десктопе. На мобильных backdrop-filter форсит
   слой компоновки и рвёт кадры при выезде панели (запрет из правил адаптивности,
   #1201), поэтому там остаётся чистая прозрачность без размытия. */
@media (min-width: 769px) {
  .gsp {
    backdrop-filter: blur(2px);
  }
}

/* На телефоне панель занимает весь экран, и просвечивать ей нечего: страницы за ней не
   видно всё равно, а размытия, которое отделяло бы находки от текста под ними, здесь
   нет. Оставшиеся 15% прозрачности читались как грязь на списке результатов, поэтому
   на мобилке подложка почти глухая. */
@media (max-width: 768px) {
  .gsp {
    background: color-mix(in srgb, var(--surface) 96%, transparent);
  }
}

/* Свёрнутая -- узкий столбик с иконкой и числом найденного: панель не мешает работать
   со страницей, но остаётся под рукой и помнит запрос. */
.gsp--collapsed {
  width: 52px;
}

.gsp__strip {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  width: 100%;
  height: 100%;
  padding: 14px 0;
  border: none;
  background: transparent;
  cursor: pointer;
  color: var(--color-text-muted);
}

.gsp__strip-count {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text);
}

.gsp__head-icon {
  flex-shrink: 0;
  opacity: 0.6;
}

.gsp__act {
  flex-shrink: 0;
  width: 34px;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: var(--radius-md, 15px);
  background: transparent;
  cursor: pointer;
  color: var(--color-text-muted);
}

.gsp__act:hover {
  background: var(--surface-2);
}

/* Закреплённое состояние -- как у закрепления рельса: цвет темы и мягкая подложка. */
.gsp__act--pin.is-pinned {
  color: var(--color-primary, #4f5bdf);
  background: var(--color-primary-tint);
}

.gsp__head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 8px 12px 16px;
  border-bottom: 1px solid var(--border);
  /* Шапка панели непрозрачная, в отличие от тела: она приходится на ту же полосу, что
     шапка приложения, и сквозь прозрачность её кнопки просвечивали под заголовком
     панели -- получалась каша из двух наложенных строк. */
  background: var(--surface);
}

.gsp__input {
  flex: 1;
  min-width: 0;
  border: none;
  background: transparent;
  color: var(--color-text);
  /* 16px, иначе iOS зазумит страницу при фокусе. */
  font-size: 16px;
  padding: 6px 0;
}

.gsp__input:focus {
  outline: none;
}

.gsp__body {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
  min-height: 132px;
}

.gsp__group + .gsp__group {
  margin-top: 14px;
}

.gsp__group-title {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-text-muted);
  padding: 0 4px 6px;
}

.gsp__row {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 48px;
  padding: 6px 10px;
  border: none;
  border-radius: var(--radius-md, 15px);
  background: transparent;
  text-align: left;
  cursor: pointer;
  color: inherit;
}

/* Подложка строки -- --surface-2: слой «внутри карточки», единственный, который ведёт
   себя правильно в обеих темах. */
.gsp__row--active {
  background: var(--surface-2);
}

.gsp__row-icon {
  flex-shrink: 0;
  opacity: 0.7;
}

.gsp__row-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.gsp__row-title {
  font-size: 14px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Счётчик рядом с названием раздела: сколько всего нашлось, а не сколько видно. */
.gsp__group-count {
  margin-left: 6px;
  padding: 0 6px;
  border-radius: 8px;
  background: var(--accent-tint);
  color: var(--accent-text);
  font-size: 11px;
  font-weight: 600;
}

/* Строка остатка: приглушённее записей - это не результат, а путь к остальным. */
.gsp__more {
  width: 100%;
  padding: 6px 10px;
  border: 0;
  border-radius: 10px;
  background: none;
  font-size: 12px;
  text-align: left;
  color: var(--accent-text);
  cursor: pointer;
}

.gsp__more:hover {
  background: var(--accent-tint);
}

.gsp__row-subtitle {
  font-size: 12px;
  color: var(--color-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.gsp__hint,
.gsp__degraded {
  padding: 18px 8px;
  margin: 0;
  font-size: 13px;
  color: var(--color-text-muted);
  text-align: center;
}

.gsp__degraded {
  padding: 6px 8px;
  text-align: left;
}

.gsp__skeleton {
  height: 40px;
  margin-bottom: 8px;
}

/* Выезд только по transform: положение и ширина не анимируются, чтобы не пересчитывать
   раскладку страницы на каждый кадр. */
.gsp-slide-enter-active,
.gsp-slide-leave-active {
  transition: transform 0.24s ease-out;
}

.gsp-slide-enter-from,
.gsp-slide-leave-to {
  transform: translateX(100%);
}

@media (max-width: 768px) {
  .gsp {
    width: 100vw;
    top: 0;
    bottom: 0;
  }

  /* На узком экране столбик занимал бы полосу поперёк единственной колонки контента,
     поэтому свёрнутая панель просто уже, но остаётся у края. */
  .gsp--collapsed {
    width: 52px;
  }
}
</style>
