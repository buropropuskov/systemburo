<template>
  <BaseModal
    :show="show"
    :title="title"
    width="560px"
    content-class="application-assign-modal"
    radius="30px"
    :z-index="10006"
    @close="$emit('close')"
  >
    <div
      class="assign"
      data-testid="application-assign-modal"
    >
      <p class="assign__summary">
        {{ summary }}
      </p>

      <div
        class="assign__content"
        :style="{ minHeight: contentMinHeight }"
      >
        <transition
          name="assign-fade"
          mode="out-in"
        >
          <div
            v-if="loading"
            key="loading"
            class="assign__grid"
            data-testid="application-assign-loading"
          >
            <SkeletonBlock
              v-for="n in skeletonCount"
              :key="n"
              :width="skeletonWidth(n)"
              height="35px"
            />
          </div>

          <div
            v-else-if="!options.length"
            key="empty"
            class="assign__state"
            data-testid="application-assign-empty"
          >
            {{ kind === 'tables' ? 'Нет доступных постов' : 'Нет доступных мест разгрузки' }}
          </div>

          <TargetTablesGrid
            v-else-if="kind === 'tables'"
            key="tables"
            v-model="selected"
            :tables="options"
            multiple
          />

          <div
            v-else
            key="places"
            class="assign__grid"
          >
            <div
              v-for="place in options"
              :key="place.id"
              class="assign__item"
              :class="{
                'assign__item--active': selected.includes(place.id) && place.status === 'active',
                'assign__item--inactive': place.status !== 'active'
              }"
              data-testid="application-assign-place"
              @click="togglePlace(place)"
            >
              {{ place.name }}
            </div>
          </div>
        </transition>
      </div>

      <p
        v-if="willClearAll"
        class="assign__warning"
        data-testid="application-assign-warning"
      >
        Ничего не выбрано: {{ kind === 'tables' ? 'элемент не попадёт ни в одну таблицу проходной' : 'у машины не останется мест разгрузки' }}.
      </p>
    </div>

    <template #actions>
      <button
        type="button"
        class="lk-button lk-button--ghost"
        data-testid="application-assign-cancel"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <button
        type="button"
        class="lk-button lk-button--primary"
        data-testid="application-assign-apply"
        :disabled="submitting"
        @click="$emit('apply', [...selected])"
      >
        {{ submitting ? 'Сохранение...' : 'Сохранить' }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import SkeletonBlock from '@/components/ui/SkeletonBlock.vue';
import TargetTablesGrid from '@/components/CreateApplication/TargetTablesGrid.vue';
import { apiRequest } from '@/api/client';

/**
 * Выбор постов или мест разгрузки при доназначении элементам заявки (#1393).
 *
 * Показывает текущий набор выбранным и отдаёт родителю итоговый список - тот шлёт
 * его режимом replace, поэтому одно окно закрывает и добавление, и снятие.
 * Для постов переиспользует TargetTablesGrid (тот же грид, что в форме заявки и в
 * групповых операциях таблиц проходной), для мест разгрузки - плитки по образцу
 * блока «Места разгрузки» VehicleForm.
 */
export default {
  name: 'ApplicationAssignModal',
  components: { BaseModal, SkeletonBlock, TargetTablesGrid },
  props: {
    show: { type: Boolean, default: false },
    // 'tables' - посты проезда/прохода, 'places' - места разгрузки
    kind: {
      type: String,
      required: true,
      validator: (v) => ['tables', 'places'].includes(v),
    },
    // 'cars' | 'people' - от типа зависят и посты, и формулировки
    elementType: {
      type: String,
      required: true,
      validator: (v) => ['cars', 'people'].includes(v),
    },
    // Что уже назначено: набор показывается выбранным
    currentIds: { type: Array, default: () => [] },
    // Сколько элементов затронет операция (1 - одна строка, N - «назначить всем»)
    targetCount: { type: Number, default: 1 },
    submitting: { type: Boolean, default: false },
  },
  emits: ['close', 'apply'],
  data() {
    return {
      selected: [],
      allTables: [],
      allPlaces: [],
      loading: false,
    };
  },
  computed: {
    title() {
      if (this.kind === 'places') return 'Места разгрузки';
      return this.elementType === 'cars' ? 'Посты проезда' : 'Места прохода';
    },

    summary() {
      if (this.targetCount > 1) {
        const word = this.elementType === 'cars' ? 'машинам' : 'сотрудникам';
        return `Выбор применится к ${this.targetCount} ${word}.`;
      }
      return 'Отметьте нужное: снятые отметки уберут привязку.';
    },

    /**
     * /system-tables отдаёт { table: {...}, fields, ... } - разворачиваем t.table,
     * иначе table_type undefined и фильтр всегда пуст.
     */
    options() {
      if (this.kind === 'places') {
        // отключённое место, уже назначенное машине, показываем - иначе оно
        // молча уедет в запрос и там будет непонятно, что происходит
        return this.allPlaces.filter(
          (p) => p.is_active !== false || this.currentIds.includes(p.id),
        );
      }
      return this.allTables
        .map((t) => t.table || t)
        .filter((tbl) => tbl.table_type === this.elementType)
        .filter((tbl) => tbl.is_active !== false || this.currentIds.includes(tbl.id))
        .map((tbl) => ({
          table: {
            id: tbl.id,
            name: tbl.name,
            display_name: tbl.display_name,
            table_type: tbl.table_type,
            status: tbl.status || 'active',
            status_comment: tbl.status_comment,
          },
        }));
    },

    /**
     * Заглушек столько, чтобы они заняли примерно те же ряды, что и данные:
     * мест обычно много (три-четыре ряда), постов у машины два-четыре (один ряд).
     */
    skeletonCount() {
      return this.kind === 'places' ? 12 : 3;
    },

    /**
     * Резерв высоты области: пока справочник грузится, окно уже занимает
     * примерно столько же, сколько займёт с данными.
     */
    contentMinHeight() {
      return this.kind === 'places' ? '190px' : '55px';
    },

    willClearAll() {
      return this.selected.length === 0 && this.currentIds.length > 0;
    },
  },
  watch: {
    show: {
      immediate: true,
      handler(val) {
        if (!val) return;
        this.selected = [...this.currentIds];
        this.load();
      },
    },
  },
  methods: {
    async load() {
      this.loading = true;
      try {
        const path = this.kind === 'places' ? '/unload-places' : '/system-tables';
        const res = await apiRequest(path);
        if (!res?.ok) return;

        const data = await res.json();
        const list = Array.isArray(data) ? data : [];
        if (this.kind === 'places') this.allPlaces = list;
        else this.allTables = list;
      } catch (error) {
        // Справочник не загрузился - окно покажет «нет доступных», а не упадёт
        // необработанным промисом.
        console.error('Не удалось загрузить справочник для назначения:', error);
      } finally {
        this.loading = false;
      }
    },

    /** Ширины заглушек чуть разные - ровный ряд одинаковых плиток выглядит искусственно. */
    skeletonWidth(index) {
      const widths = ['132px', '108px', '156px', '120px', '144px', '112px', '128px', '150px'];
      return widths[(index - 1) % widths.length];
    },

    togglePlace(place) {
      if (place.status !== 'active') return;
      const index = this.selected.indexOf(place.id);
      if (index === -1) this.selected.push(place.id);
      else this.selected.splice(index, 1);
    },
  },
};
</script>

<style scoped>
/* Цвета берём из переменных темы (#1415): захардкоженный белый фон и тёмный
   текст делали плитки нечитаемыми в тёмной теме. */
.assign {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.assign__summary {
  margin: 0;
  font-size: 14px;
  color: var(--text-muted, var(--text-muted));
}

.assign__state {
  padding: 24px 0;
  text-align: center;
  color: var(--text-muted, var(--text-muted));
  font-size: 14px;
}

/* Область контента резервирует высоту (значение приходит из contentMinHeight):
   без неё окно прыгало, когда заглушки сменялись плитками. */
.assign__content {
  display: flex;
  flex-direction: column;
  justify-content: flex-start;
}

/* Смена заглушек на данные - мягкая, без рывка */
.assign-fade-enter-active,
.assign-fade-leave-active {
  transition: opacity 0.18s ease;
}

.assign-fade-enter-from,
.assign-fade-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .assign-fade-enter-active,
  .assign-fade-leave-active {
    transition: none;
  }
}

/* Плитки мест - по образцу блока «Места разгрузки» формы машины */
.assign__grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  max-height: 320px;
  overflow-y: auto;
}

.assign__item {
  padding: 8px 14px;
  border: 1px solid var(--border, var(--border));
  border-radius: var(--radius-md);
  background: var(--surface-2, var(--surface-2));
  color: var(--text, var(--text));
  font-size: 13.5px;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.assign__item:not(.assign__item--active):not(.assign__item--inactive):hover {
  border-color: var(--accent);
  color: var(--accent-text);
}

.assign__item--active {
  border-color: var(--accent);
  background: var(--color-primary);
  color: var(--accent-contrast);
  font-weight: 600;
}

.assign__item--inactive {
  opacity: 0.45;
  cursor: not-allowed;
}

.assign__warning {
  margin: 0;
  padding: 9px 12px;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--warning) 16%, var(--surface));
  color: var(--warning-text);
  font-size: 13px;
}
</style>
