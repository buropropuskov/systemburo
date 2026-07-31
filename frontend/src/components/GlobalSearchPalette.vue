<template>
  <BaseModal
    :show="show"
    :width="'640px'"
    :radius="'30px'"
    :closable="false"
    content-class="gsp"
    :z-index="15000"
    @close="close"
  >
    <template #header>
      <div class="gsp__search">
        <NavIcon
          name="search"
          :size="18"
          class="gsp__search-icon"
        />
        <input
          ref="input"
          v-model="query"
          class="lk-input gsp__input"
          type="text"
          placeholder="Поиск по системе: человек, машина, заявка, раздел"
          aria-label="Поиск по системе"
          autocomplete="off"
          @keydown.down.prevent="move(1)"
          @keydown.up.prevent="move(-1)"
          @keydown.enter.prevent="openActive"
        >
        <button
          class="gsp__close"
          type="button"
          aria-label="Закрыть"
          @click="close"
        >
          &times;
        </button>
      </div>
    </template>

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
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
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
  name: 'GlobalSearchPalette',
  components: { BaseModal, NavIcon, SkeletonLine },
  props: {
    show: { type: Boolean, required: true },
  },
  emits: ['close'],
  setup() {
    const { can } = usePermission();
    const search = useGlobalSearch();
    return { can, ...search, MIN_QUERY_LENGTH };
  },
  data() {
    return {
      query: '',
      activeIndex: 0,
    };
  },
  computed: {
    /**
     * Разделы меню -- первая группа выдачи и единственная, что считается на месте.
     * Ради неё поиск и затевался: чаще всего человек не помнит, в каком разделе искать.
     */
    sectionItems() {
      const raw = this.query.trim();
      if (!raw) return [];
      const variants = buildSearchVariants(raw);

      const all = [
        ...MAIN_SECTIONS,
        ...ADMIN_GROUPS.flatMap((g) => g.items),
      ];
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
     * Группы выдачи с плоской нумерацией строк: стрелки ходят сквозь заголовки, поэтому
     * индекс проставляется один раз здесь, а не пересчитывается на каждое нажатие.
     */
    visibleGroups() {
      const groups = [];
      let index = 0;

      const push = (type, title, items) => {
        if (!items.length) return;
        groups.push({
          type,
          title,
          items: items.map((it) => ({ ...it, index: index++ })),
        });
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
    /** Заголовки разделов, которые не ответили: молчать о них нельзя -- пусто и «нет данных» читаются одинаково. */
    degradedLabels() {
      return (this.degraded || []).map((t) => SEARCH_TARGETS.groupTitles?.[t] || t);
    },
  },
  watch: {
    query(val) {
      this.activeIndex = 0;
      this.search(val);
    },
    show(val) {
      if (val) {
        this.query = '';
        this.activeIndex = 0;
        // Фокус после отрисовки окна: до неё поля ещё нет в документе.
        this.$nextTick(() => this.$refs.input?.focus());
      } else {
        this.cancel();
      }
    },
  },
  mounted() {
    // Capture на document: NavMenu слушает Escape на window, а BaseModal -- на document
    // в фазе всплытия. Оба сработали бы раньше, чем мы успеем остановить событие, и
    // Escape закрыл бы заодно выдвижное меню под палитрой.
    document.addEventListener('keydown', this.onKeydownCapture, true);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeydownCapture, true);
  },
  methods: {
    onKeydownCapture(e) {
      if (!this.show) return;
      if (e.key === 'Escape') {
        e.stopPropagation();
        e.preventDefault();
        this.close();
      }
    },
    move(delta) {
      const total = this.flatItems.length;
      if (!total) return;
      // Без зацикливания: в длинном списке прыжок с конца в начало дезориентирует.
      this.activeIndex = Math.min(Math.max(this.activeIndex + delta, 0), total - 1);
      this.$nextTick(() => {
        this.$el?.querySelector('.gsp__row--active')?.scrollIntoView({ block: 'nearest' });
      });
    },
    openActive() {
      const item = this.flatItems[this.activeIndex];
      if (item) this.openItem(item);
    },
    /** Куда ведёт результат: маршруты знает фронт, бэк отдаёт только сущность и её номер. */
    routeFor(item) {
      const target = SEARCH_TARGETS[item.target?.entity];
      if (!target) return null;
      return target.route(item.target.id, this.query.trim());
    },
    openItem(item) {
      if (!item.to) return;
      // Сначала закрываем окно, потом переходим: подтверждение о несохранённой форме
      // рисуется ниже палитры по слоям и осталось бы невидимым, а переход -- висящим.
      this.close();
      this.$nextTick(() => this.$router.push(item.to));
    },
    close() {
      this.$emit('close');
    },
  },
};
</script>

<style scoped>
.gsp__search {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
}

.gsp__search-icon {
  flex-shrink: 0;
  opacity: 0.6;
}

.gsp__input {
  flex: 1;
  min-width: 0;
  border: none;
  background: transparent;
  font-size: 16px;
  padding: 8px 0;
}

.gsp__input:focus {
  outline: none;
}

.gsp__close {
  flex-shrink: 0;
  width: 44px;
  height: 44px;
  border: none;
  background: transparent;
  font-size: 26px;
  line-height: 1;
  cursor: pointer;
  color: var(--color-text-muted, #8b8b93);
}

.gsp__body {
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
  color: var(--color-text-muted, #8b8b93);
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
}

.gsp__row--active {
  background: var(--color-surface-hover, #f1f1f4);
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
  color: var(--color-text-muted, #8b8b93);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.gsp__hint,
.gsp__degraded {
  padding: 18px 8px;
  margin: 0;
  font-size: 13px;
  color: var(--color-text-muted, #8b8b93);
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

/* Десктоп: окно читается сверху, а не по центру вьюпорта. Своим media-блоком, а не
   глобальным переопределением: иначе правило перебило бы мобильную раскладку. */
@media (min-width: 769px) {
  :global(.base-modal-overlay:has(.gsp)) {
    align-items: flex-start;
    padding-top: 10vh;
  }
}

@media (max-width: 768px) {
  /* На мобилке поле живёт в шапке окна, куда правило BaseModal о 16px не достаёт:
     меньший кегль заставил бы iOS зазумить страницу при фокусе. */
  .gsp__input {
    font-size: 16px;
  }
}
</style>
