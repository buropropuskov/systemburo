<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modes-overlay"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        data-testid="work-modes-modal"
        @click.self="close"
      >
        <div class="modes">
          <header class="modes__header">
            <div>
              <h2
                :id="titleId"
                class="modes__title"
              >
                Режимы работы
              </h2>
              <p class="modes__sub">
                Время работы Бюро, мест разгрузки и мест прохода
              </p>
            </div>
            <button
              type="button"
              class="modes__close"
              aria-label="Закрыть"
              @click="close"
            >
              ×
            </button>
          </header>

          <div
            v-if="loading"
            class="modes__state"
          >
            <LoaderSpinner label="Загрузка режимов работы…" />
          </div>

          <div
            v-else-if="error"
            class="modes__state"
          >
            Не удалось загрузить режимы работы
          </div>

          <template v-else>
            <div
              class="cats"
              role="tablist"
              aria-label="Категории объектов"
            >
              <button
                v-for="cat in categories"
                :key="cat.key"
                type="button"
                role="tab"
                class="cat-pill"
                :class="{ 'cat-pill--active': cat.key === activeCat }"
                :aria-selected="cat.key === activeCat"
                :data-testid="`work-modes-cat-${cat.key}`"
                @click="activeCat = cat.key"
              >
                <span class="cat-pill__icon">
                  <component :is="iconFor(cat.key)" />
                </span>
                {{ cat.label }}
                <span
                  v-if="cat.showCount"
                  class="cat-pill__cnt"
                >{{ cat.items.length }}</span>
              </button>
            </div>

            <div class="modes__body">
              <div
                v-if="activeCat !== 'bureau'"
                class="modes__search"
              >
                <input
                  v-model="searchQuery"
                  type="text"
                  class="lk-input"
                  placeholder="Поиск по названию..."
                >
              </div>
              <div
                v-if="!visibleItems.length"
                class="modes__empty"
              >
                {{ searchQuery ? 'Ничего не найдено' : 'Нет объектов в этой категории' }}
              </div>
              <div
                v-for="item in visibleItems"
                v-else
                :key="`${item.kind}:${item.id}`"
                class="obj"
                :class="{ 'obj--open': isOpen(item) }"
                data-testid="work-modes-card"
              >
                <div
                  class="obj__head"
                  @click="toggle(item)"
                >
                  <span class="obj__icon">
                    <component :is="iconFor(item.kind)" />
                  </span>
                  <div class="obj__main">
                    <div class="obj__name">
                      {{ item.name }}
                    </div>
                    <div class="obj__today">
                      <span>Сегодня</span> · <b>{{ todayText(item) }}</b>
                    </div>
                  </div>
                  <span
                    class="status"
                    :class="`status--${statusClass(item)}`"
                    data-testid="work-modes-status"
                  >
                    {{ statusText(item) }}
                  </span>
                  <svg
                    class="obj__chev"
                    width="16"
                    height="16"
                    viewBox="0 0 16 16"
                    fill="none"
                  >
                    <path
                      d="M4 6l4 4 4-4"
                      stroke="currentColor"
                      stroke-width="1.8"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    />
                  </svg>
                </div>
                <div class="obj__week-wrap">
                  <div class="obj__week-inner">
                    <div class="week">
                      <div
                        v-for="(day, i) in DAYS"
                        :key="i"
                        class="week__row"
                        :class="{ 'week__row--today': i === todayIdx }"
                      >
                        <span class="week__day">{{ day }}</span>
                        <span
                          class="week__time"
                          :class="dayText(item, i).cls"
                        >{{ dayText(item, i).txt }}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </template>

          <footer class="modes__footer">
            <div class="modes__legend">
              <span><i style="background: #43a047" />Открыто</span>
              <span><i style="background: #ef6c00" />Закрыто</span>
              <span><i style="background: #c62828" />Неактивно</span>
            </div>
            <button
              type="button"
              class="modes__done"
              @click="close"
            >
              Готово
            </button>
          </footer>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { h } from 'vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import { getWorkModes } from '@/api/work-modes';

let uid = 0;

// 0 = Понедельник ... 6 = Воскресенье (как day_of_week на бэке).
const DAYS = ['Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота', 'Воскресенье'];

const BureauIcon = {
  render() {
    return h('svg', { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none' }, [
      h('path', { d: 'M4 21V8l8-5 8 5v13', stroke: 'currentColor', 'stroke-width': 1.7, 'stroke-linejoin': 'round' }),
      h('path', { d: 'M9 21v-6h6v6M3 21h18', stroke: 'currentColor', 'stroke-width': 1.7, 'stroke-linecap': 'round', 'stroke-linejoin': 'round' }),
    ]);
  },
};
const UnloadIcon = {
  render() {
    return h('svg', { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none' }, [
      h('path', { d: 'M3 7h11v8H3zM14 10h4l3 3v2h-7z', stroke: 'currentColor', 'stroke-width': 1.7, 'stroke-linejoin': 'round' }),
      h('circle', { cx: 7, cy: 18, r: 1.8, stroke: 'currentColor', 'stroke-width': 1.7 }),
      h('circle', { cx: 17, cy: 18, r: 1.8, stroke: 'currentColor', 'stroke-width': 1.7 }),
    ]);
  },
};
const CheckpointIcon = {
  render() {
    return h('svg', { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none' }, [
      h('path', { d: 'M5 21V5a2 2 0 0 1 2-2h6v18M13 9h6v12M3 21h18', stroke: 'currentColor', 'stroke-width': 1.7, 'stroke-linecap': 'round', 'stroke-linejoin': 'round' }),
    ]);
  },
};

export default {
  name: 'WorkModesModal',
  components: { LoaderSpinner },
  props: {
    show: { type: Boolean, default: false },
  },
  emits: ['close'],
  data() {
    uid += 1;
    return {
      DAYS,
      data: null,
      loading: false,
      error: false,
      loadSeq: 0,
      activeCat: 'bureau',
      searchQuery: '',
      openKeys: [],
      titleId: `work-modes-title-${uid}`,
    };
  },
  computed: {
    /** Сегодня в формате day_of_week: Пн(0)..Вс(6). */
    todayIdx() {
      return (new Date().getDay() + 6) % 7;
    },
    categories() {
      const d = this.data;
      return [
        { key: 'bureau', label: 'Бюро пропусков', items: d?.bureau ? [d.bureau] : [], showCount: false },
        { key: 'unload', label: 'Места разгрузки', items: d?.unload_places || [], showCount: true },
        { key: 'pass', label: 'Места прохода', items: d?.checkpoints || [], showCount: true },
      ];
    },
    currentItems() {
      return this.categories.find((c) => c.key === this.activeCat)?.items || [];
    },
    // Список с учётом поиска (поиск показывается для мест разгрузки/прохода).
    visibleItems() {
      const q = this.searchQuery.trim().toLowerCase();
      if (!q) return this.currentItems;
      return this.currentItems.filter((it) => (it.name || '').toLowerCase().includes(q));
    },
  },
  watch: {
    // Модалка всегда смонтирована (leave-анимация): блокировку скролла и загрузку
    // вешаем на открытие. Перезапрашиваем каждый раз - current_status зависит от времени.
    show(visible) {
      document.body.style.overflow = visible ? 'hidden' : '';
      if (visible) this.load();
    },
    // Сброс поиска при переключении категории.
    activeCat() {
      this.searchQuery = '';
    },
  },
  mounted() {
    document.addEventListener('keydown', this.onKey);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKey);
    document.body.style.overflow = '';
  },
  methods: {
    async load() {
      const seq = (this.loadSeq += 1);
      this.loading = true;
      this.error = false;
      try {
        const data = await getWorkModes();
        if (seq !== this.loadSeq) return;
        this.data = data;
        this.activeCat = 'bureau';
        this.initExpanded();
      } catch {
        if (seq !== this.loadSeq) return;
        this.error = true;
      } finally {
        if (seq === this.loadSeq) this.loading = false;
      }
    },
    /** Бюро и категории с единственным объектом раскрыты по умолчанию. */
    initExpanded() {
      const keys = [];
      this.categories.forEach((cat) => {
        if (cat.key === 'bureau') {
          cat.items.forEach((it) => keys.push(this.keyOf(it)));
        } else if (cat.items.length === 1) {
          keys.push(this.keyOf(cat.items[0]));
        }
      });
      this.openKeys = keys;
    },
    keyOf(item) {
      return `${item.kind}:${item.id}`;
    },
    isOpen(item) {
      return this.openKeys.includes(this.keyOf(item));
    },
    toggle(item) {
      const key = this.keyOf(item);
      this.openKeys = this.openKeys.includes(key)
        ? this.openKeys.filter((k) => k !== key)
        : [...this.openKeys, key];
    },
    iconFor(kind) {
      if (kind === 'bureau') return BureauIcon;
      if (kind === 'unload' || kind === 'unload_place') return UnloadIcon;
      return CheckpointIcon;
    },
    statusClass(item) {
      if (item.status !== 'active') return 'inactive';
      return item.current_status === 'open' ? 'open' : 'closed';
    },
    statusText(item) {
      if (item.status !== 'active') {
        return item.status === 'maintenance' ? 'На обслуживании' : 'Неактивно';
      }
      return item.current_status === 'open' ? 'Открыто сейчас' : 'Закрыто сейчас';
    },
    /** Активные слоты дня (0=Пн..6=Вс). */
    activeSlots(item, dayIdx) {
      return (item.time_slots || []).filter((s) => s.day_of_week === dayIdx && s.is_active);
    },
    fmtTime(t) {
      return (t || '').slice(0, 5);
    },
    isRoundTheClock(slot) {
      return this.fmtTime(slot.open_time) === '00:00' && this.fmtTime(slot.close_time) === '23:59' && !slot.is_next_day;
    },
    /** Текст и класс ячейки дня: выходной / круглосуточно / окна времени. */
    dayText(item, dayIdx) {
      const slots = this.activeSlots(item, dayIdx);
      if (!slots.length) return { txt: 'выходной', cls: 'is-off' };
      if (slots.some((s) => this.isRoundTheClock(s))) return { txt: 'круглосуточно', cls: 'is-24' };
      const txt = slots.map((s) => `${this.fmtTime(s.open_time)} – ${this.fmtTime(s.close_time)}`).join(', ');
      return { txt, cls: '' };
    },
    todayText(item) {
      return this.dayText(item, this.todayIdx).txt;
    },
    onKey(e) {
      if (this.show && e.key === 'Escape') this.close();
    },
    close() {
      this.$emit('close');
    },
  },
};
</script>

<style scoped>
.modes-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
  padding: 24px;
}

.modes {
  width: min(720px, 100%);
  /* Фиксированная высота: список едет скроллом в .modes__body, модалка не прыгает
     при смене вкладок Бюро пропусков/Места разгрузки/Места прохода. */
  height: min(620px, 88vh);
  background: #fff;
  border-radius: 30px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 24px 70px rgba(20, 24, 60, 0.28);
  animation: modes-pop 0.22s ease;
}

@keyframes modes-pop {
  from { opacity: 0; transform: translateY(12px) scale(0.985); }
  to { opacity: 1; transform: none; }
}

.modes__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 22px 26px 18px;
}

.modes__title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: #1a1a1a;
}

.modes__sub {
  margin: 3px 0 0;
  font-size: 12.5px;
  color: #9a9aae;
  font-weight: 500;
}

.modes__close {
  width: 36px;
  height: 36px;
  border: none;
  background: #f3f4fa;
  border-radius: 50%;
  font-size: 22px;
  line-height: 1;
  color: #6a6a7d;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
  flex-shrink: 0;
}

.modes__close:hover {
  background: #e9eaf4;
  color: #1a1a1a;
}

.modes__state {
  padding: 40px 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #9a9aae;
  font-size: 14px;
  flex: 1 1 auto;
}

.cats {
  display: flex;
  gap: 8px;
  padding: 0 26px 4px;
}

.cat-pill {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 42px;
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: var(--radius-pill);
  font-family: inherit;
  font-size: 13.5px;
  font-weight: 600;
  color: #6a6a7d;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.cat-pill:hover:not(.cat-pill--active) {
  border-color: #cfd4ff;
  color: var(--color-primary);
}

.cat-pill--active {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: #fff;
}

.cat-pill__icon {
  display: inline-flex;
  flex-shrink: 0;
}

.cat-pill__cnt {
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: var(--radius-pill);
  background: #eef0ff;
  color: var(--color-primary);
  font-size: 11px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.cat-pill--active .cat-pill__cnt {
  background: rgba(255, 255, 255, 0.25);
  color: #fff;
}

.modes__body {
  padding: 16px 26px 8px;
  overflow-y: auto;
  flex: 1 1 auto;
  min-height: 0;
}

.modes__search {
  margin-bottom: 12px;
}

.modes__search .lk-input {
  width: 100%;
}

.modes__body::-webkit-scrollbar {
  width: 6px;
}

.modes__body::-webkit-scrollbar-thumb {
  background: #d9e2ff;
  border-radius: 4px;
}

.obj {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: #fafbff;
  margin-bottom: 12px;
  overflow: hidden;
  transition: border-color 0.15s ease;
}

.obj:hover {
  border-color: #cfd4ff;
}

.obj__head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  cursor: pointer;
  user-select: none;
}

.obj__icon {
  width: 38px;
  height: 38px;
  flex-shrink: 0;
  border-radius: 11px;
  background: #eef0ff;
  color: var(--color-primary);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.obj__main {
  flex: 1;
  min-width: 0;
}

.obj__name {
  font-size: 14.5px;
  font-weight: 700;
  color: #1a1a2e;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.obj__today {
  margin-top: 3px;
  font-size: 12px;
  color: #8a8a9e;
  display: flex;
  align-items: center;
  gap: 6px;
}

.obj__today b {
  color: #3a3a4a;
  font-weight: 600;
}

.status {
  flex-shrink: 0;
  height: 26px;
  padding: 0 12px;
  display: inline-flex;
  align-items: center;
  border-radius: var(--radius-pill);
  font-size: 11.5px;
  font-weight: 600;
  white-space: nowrap;
}

.status--open {
  background: #e6f7e6;
  color: #2e7d32;
  border: 1px solid #a5d6a7;
}

.status--closed {
  background: #fff3e0;
  color: #ef6c00;
  border: 1px solid #ffcc80;
}

.status--inactive {
  background: #ffebee;
  color: #c62828;
  border: 1px solid #ef9a9a;
}

.obj__chev {
  flex-shrink: 0;
  color: #b3b6cf;
  transition: transform 0.2s ease;
}

.obj--open .obj__chev {
  transform: rotate(180deg);
}

.obj__week-wrap {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows 0.24s ease;
}

.obj--open .obj__week-wrap {
  grid-template-rows: 1fr;
}

.obj__week-inner {
  min-height: 0;
  overflow: hidden;
}

.week {
  padding: 4px 16px 14px;
  border-top: 1px solid #eef0f6;
}

.week__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 7px 12px;
  border-radius: 10px;
  font-size: 13px;
}

.week__row + .week__row {
  margin-top: 2px;
}

.week__row--today {
  background: #f0f4ff;
}

.week__day {
  color: #6a6a7d;
  font-weight: 600;
}

.week__row--today .week__day {
  color: var(--color-primary);
}

.week__time {
  color: #2a2a38;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.week__time.is-off {
  color: #b9b9c6;
  font-weight: 500;
}

.week__time.is-24 {
  color: #2e7d32;
}

.modes__empty {
  padding: 30px;
  text-align: center;
  color: #b9b9c6;
  font-size: 13px;
}

.modes__footer {
  padding: 14px 26px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.modes__legend {
  display: flex;
  gap: 14px;
  font-size: 11px;
  color: #9a9aae;
}

.modes__legend span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.modes__legend i {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  display: inline-block;
}

.modes__done {
  height: 42px;
  padding: 0 26px;
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: var(--radius-pill);
  font-family: inherit;
  font-size: 13.5px;
  font-weight: 600;
  color: #6a6a7d;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.modes__done:hover {
  background: #f3f4fa;
  color: #1a1a1a;
}

@media (max-width: 560px) {
  .cats {
    flex-wrap: wrap;
  }
  .cat-pill {
    flex: 1 1 auto;
  }
  .modes__legend {
    display: none;
  }
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
</style>
