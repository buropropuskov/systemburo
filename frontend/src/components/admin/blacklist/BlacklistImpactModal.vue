<template>
  <BaseModal
    :show="show"
    title="Где эта запись сейчас есть"
    width="560px"
    :z-index="10006"
    content-class="bl-impact-modal"
    @close="$emit('close')"
  >
    <div class="impact-body">
      <p class="impact-lead">
        <strong>{{ subject }}</strong> перестанет действовать: строки уйдут из таблиц
        постов, пропуск работать не будет. В заявках запись останется - там она будет
        помечена как внесённая в чёрный список.
      </p>

      <div class="impact-counter" data-testid="impact-count">
        Перестанет действовать строк: <strong>{{ impact.matches }}</strong>
      </div>

      <div
        v-if="impact.tables && impact.tables.length"
        class="impact-block"
      >
        <div class="impact-block__title">
          Уйдёт из таблиц постов
        </div>
        <div class="impact-chips" data-testid="impact-tables">
          <span
            v-for="table in impact.tables"
            :key="table"
            class="impact-chip"
          >
            {{ table }}
          </span>
        </div>
      </div>

      <div
        v-if="rowsWithApplications.length"
        class="impact-block"
      >
        <div class="impact-block__title">
          Фигурирует в заявках
        </div>
        <ul class="impact-list" data-testid="impact-applications">
          <li
            v-for="(row, index) in rowsWithApplications"
            :key="index"
          >
            {{ row.label }}<template v-if="row.organization"> ({{ row.organization }})</template>:
            {{ row.applications.join(', ') }}
          </li>
        </ul>
      </div>

      <p
        v-if="!impact.matches"
        class="impact-empty"
        data-testid="impact-empty"
      >
        Действующих строк нет - внесение никого не затронет.
      </p>
    </div>

    <template #actions>
      <button
        type="button"
        class="lk-button lk-button--ghost"
        :disabled="submitting"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <button
        type="button"
        class="lk-button lk-button--danger"
        :disabled="submitting"
        data-testid="impact-confirm"
        @click="$emit('confirm')"
      >
        {{ confirmLabel }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'

export default {
    name: 'BlacklistImpactModal',
    components: { BaseModal },
    props: {
        show: {
            type: Boolean,
            required: true
        },
        /** Кого вносят: ФИО либо номер с маркой - для заголовка окна. */
        subject: {
            type: String,
            default: ''
        },
        impact: {
            type: Object,
            default: () => ({ matches: 0, tables: [], rows: [] })
        },
        submitting: {
            type: Boolean,
            default: false
        },
        /** Надпись кнопки: внесение и возврат из архива приводят к одному итогу,
            но называются по-разному. */
        confirmLabel: {
            type: String,
            default: 'Внести в чёрный список'
        }
    },
    emits: ['confirm', 'close'],
    computed: {
        rowsWithApplications() {
            const rows = (this.impact && this.impact.rows) || []
            return rows.filter((row) => row.applications && row.applications.length)
        }
    }
}
</script>

<style scoped>
.impact-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 20px;
}

.impact-lead {
  margin: 0;
  font-size: 13px;
  line-height: 1.45;
  color: var(--text-muted);
}

.impact-counter {
  padding: 10px 14px;
  border-radius: var(--radius-md);
  background: var(--danger-bg);
  color: var(--danger-text);
  font-size: 14px;
}

.impact-block__title {
  margin-bottom: 6px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.impact-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.impact-chip {
  padding: 4px 10px;
  border-radius: 999px;
  background: var(--accent-tint);
  font-size: 12px;
}

.impact-list {
  margin: 0;
  padding-left: 18px;
  font-size: 13px;
  line-height: 1.5;
}

.impact-empty {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
}
</style>

<!-- не scoped: контент BaseModal телепортится в body и несёт data-v самого BaseModal,
     поэтому радиус задаём глобально двойным классом - как в BlacklistOverrideModal. -->
<style>
.base-modal.bl-impact-modal {
  border-radius: 30px;
}
</style>
