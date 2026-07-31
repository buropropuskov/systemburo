<template>
  <teleport to="body">
    <transition name="gsp-slide">
      <aside
        v-if="open"
        ref="panel"
        class="gsp"
        role="complementary"
        aria-label="Результаты поиска"
      >
        <header class="gsp__head">
          <span class="gsp__head-title">Результаты поиска</span>
          <button
            class="gsp__close"
            type="button"
            aria-label="Закрыть результаты"
            @click="close"
          >
            &times;
          </button>
        </header>

        <div class="gsp__body">
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

/** Сколько разделов меню показывать: список длинный, а нужен обычно первый же. */
const SECTIONS_LIMIT = 5;

export default {
  name: 'GlobalSearchPanel',
  components: { NavIcon, SkeletonLine },
  props: {
    /** Строка из поля поиска в меню. Ввод идёт там, панель только показывает найденное. */
    query: { type: String, default: '' },
  },
  emits: ['close'],
  setup() {
    const { can } = usePermission();
    const search = useGlobalSearch();
    return { can, ...search, MIN_QUERY_LENGTH };
  },
  data() {
    return {
      activeIndex: 0,
    };
  },
  computed: {
    /** Панель открыта, пока в поле что-то есть: пустой запрос закрывает её сам собой. */
    open() {
      return this.query.trim().length > 0;
    },
    /**
     * Разделы меню -- первая группа и единственная, что считается на месте. Ради неё
     * поиск и затевался: чаще всего человек не помнит, в каком разделе искать.
     */
    sectionItems() {
      const raw = this.query.trim();
      if (!raw) return [];
      const variants = buildSearchVariants(raw);

      const all = [...MAIN_SECTIONS, ...ADMIN_GROUPS.flatMap((g) => g.items)];
      return all
        .filter((s) => (!s.permission || this.can(s.permission)) && matchesSearch(s.label, variants))
        .slice(0, SECTIONS_LIMIT)
        .map((s) => ({
          key: s.path,
          title: s.label,
          subtitle: '',
          icon: s.icon,
          to: { path: s.path },
        }));
    },
    /**
     * Группы со сквозной нумерацией строк: стрелки ходят сквозь заголовки, поэтому
     * индекс проставляется один раз здесь, а не пересчитывается на каждое нажатие.
     */
    visibleGroups() {
      const groups = [];
      let index = 0;

      const push = (type, title, items) => {
        if (!items.length) return;
        groups.push({ type, title, items: items.map((it) => ({ ...it, index: index++ })) });
      };

      push('sections', 'Разделы', this.sectionItems);
      for (const g of this.groups) {
        push(g.type, g.title, (g.items || []).map((it) => ({
          key: `${it.type}-${it.id}`,
          title: it.title,
          subtitle: it.subtitle,
          icon: SEARCH_TARGETS[it.target?.entity]?.icon || 'search',
          to: this.routeFor(it),
        })));
      }
      return groups;
    },
    flatItems() {
      return this.visibleGroups.flatMap((g) => g.items);
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
    query: {
      immediate: true,
      handler(val) {
        this.activeIndex = 0;
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
        this.close();
        return;
      }
      // Стрелки и ввод работают, пока курсор в поле поиска: список живёт в панели, а
      // фокус остаётся в меню, иначе после каждой буквы пришлось бы возвращать его руками.
      if (e.key === 'ArrowDown') { e.preventDefault(); this.move(1); }
      if (e.key === 'ArrowUp') { e.preventDefault(); this.move(-1); }
      if (e.key === 'Enter') this.openActive();
    },
    /** Клик мимо панели и мимо поля поиска закрывает результаты. */
    onDocumentMousedown(e) {
      if (!this.open) return;
      if (this.$refs.panel?.contains(e.target)) return;
      if (e.target.closest?.('.nav-search-row')) return;
      this.close();
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
      // Сначала закрываем панель, потом переходим: подтверждение о несохранённой форме
      // роутер поднимает поверх страницы, и панель не должна его перекрывать.
      this.close();
      this.$nextTick(() => this.$router.push(item.to));
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
  /* Панель начинается под шапкой: накрывая её, она обрезала кнопки и выглядела
     съехавшей поверх интерфейса, а не рядом с ним. */
  top: 60px;
  right: 0;
  bottom: 0;
  width: 420px;
  max-width: 100vw;
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border-left: 1px solid var(--border);
  box-shadow: -8px 0 24px rgb(0 0 0 / 12%);
  /* Выше выдвижного меню и карточек, ниже блокирующих окон: подтверждение о
     несохранённой форме и сообщение о блокировке должны перекрывать результаты. */
  z-index: 15000;
}

.gsp__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 14px 8px 14px 18px;
  border-bottom: 1px solid var(--border);
}

.gsp__head-title {
  font-size: 13px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-text-muted);
}

.gsp__close {
  width: 44px;
  height: 44px;
  border: none;
  background: transparent;
  font-size: 26px;
  line-height: 1;
  cursor: pointer;
  color: var(--color-text-muted);
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
    top: var(--mobile-header-height, 55px);
    bottom: 0;
  }
}
</style>
