<template>
  <div class="pd-section">
    <!-- Раздел свёрнут по умолчанию: про персональные данные в карточке вспоминают
         редко, а места блок занимает больше прочих полей. Раскрыт сразу, когда есть
         что показать: стоит возражение или отметка об уведомлении ещё не поставлена. -->
    <button
      type="button"
      class="pd-section__toggle"
      :aria-expanded="open ? 'true' : 'false'"
      data-testid="pd-section-toggle"
      @click="open = !open"
    >
      <span class="pd-section__title">Персональные данные</span>
      <span
        class="pd-section__badge"
        :class="{ 'pd-section__badge--alert': !!localObjectedAt }"
      >{{ summary }}</span>
      <span class="pd-section__chevron">{{ open ? '−' : '+' }}</span>
    </button>

    <div
      v-show="open"
      class="pd-section__body"
      data-testid="pd-section-body"
    >
      <!-- Уведомление субъекта (часть 3 статьи 18 152-ФЗ): данные вводит заявитель,
           он же обязан уведомить человека. У записи с отметкой показываем дату. -->
      <label class="input__label">Уведомление об обработке персональных данных</label>
      <p
        v-if="consentAt"
        class="pd-section__muted"
        data-testid="employee-consent-granted"
      >
        Уведомлён {{ formatDate(consentAt) }}
      </p>
      <label
        v-else
        class="consent-option"
      >
        <input
          :checked="consent"
          type="checkbox"
          data-testid="employee-registry-pd-consent"
          @change="$emit('update:consent', $event.target.checked)"
        >
        <span>
          Работник уведомлён об <a
            href="/data-processing"
            target="_blank"
            rel="noopener"
            class="blue"
            @click.stop
          >обработке персональных данных</a><span class="required">*</span>
        </span>
      </label>

      <div class="pd-section__objection">
        <template v-if="localObjectedAt">
          <p
            class="pd-section__note"
            data-testid="objection-note"
          >
            Работник возразил против обработки своих персональных данных
            {{ formatDate(localObjectedAt) }}. Фамилия, имя, должность и документы
            доступны только для чтения, действующие пропуска аннулированы.
          </p>
          <p
            v-if="localSource"
            class="pd-section__muted"
            data-testid="objection-source"
          >
            Основание: {{ localSource }}
          </p>
          <button
            v-if="canManageAll"
            type="button"
            class="lk-button lk-button--secondary pd-section__action"
            :disabled="busy"
            data-testid="objection-clear"
            @click="clear"
          >
            {{ busy ? 'Снимаем...' : 'Снять отметку' }}
          </button>
          <p
            v-else
            class="pd-section__muted"
          >
            Снять отметку может администратор бюро по итогам рассмотрения обращения.
          </p>
        </template>

        <template v-else>
          <label
            class="input__label"
            for="objection-source"
          >Возражение против обработки данных</label>
          <p class="pd-section__muted">
            Отметьте, если работник потребовал прекратить обработку своих данных.
            Сведения останутся в системе, но станут доступны только для чтения,
            а действующие пропуска будут аннулированы.
          </p>
          <input
            id="objection-source"
            v-model="draftSource"
            class="lk-input"
            type="text"
            placeholder="Откуда поступило обращение"
            data-testid="objection-source-input"
          >
          <button
            type="button"
            class="lk-button lk-button--danger pd-section__action"
            :disabled="busy || !draftSource.trim()"
            data-testid="objection-set"
            @click="submit"
          >
            {{ busy ? 'Отмечаем...' : 'Отметить возражение' }}
          </button>
        </template>
      </div>
    </div>
  </div>
</template>

<script>
import { setEmployeeObjection, clearEmployeeObjection } from '@/api/employees';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Раздел карточки сотрудника про персональные данные (#2361): отметка об уведомлении
 * и управление возражением субъекта.
 *
 * Отдельный компонент, а не блок в карточке: карточка реестра упёрта в предел размера
 * по всем трём частям сразу. Заодно переиспользуется - тот же раздел понадобится для
 * владельцев транспорта, когда возражение заведут и там.
 *
 * Ставить отметку о возражении может тот же круг, что правит запись: человек скажет
 * о возражении своему работодателю, а не бюро, и обращение иначе потеряется. Снимать -
 * только администратор бюро: возражение адресовано оператору, решение принимает он.
 */
export default {
    name: 'EmployeeObjectionControl',
    props: {
        employeeId: { type: Number, default: null },
        objectedAt: { type: String, default: null },
        source: { type: String, default: '' },
        consentAt: { type: String, default: null },
        consent: { type: Boolean, default: false },
        canManageAll: { type: Boolean, default: false },
    },
    emits: ['changed', 'update:consent'],
    data() {
        // Состояние держим у себя: карточка после действия остаётся открытой, и вид
        // обязан смениться сразу. Наверх уходит событие, по которому вью перечитывает
        // список.
        return {
            draftSource: '',
            busy: false,
            localObjectedAt: this.objectedAt,
            localSource: this.source,
            // Новую запись заводят с незаполненной отметкой, и прятать обязательное
            // поле нельзя: человек не найдёт, почему не сохраняется.
            open: !!this.objectedAt || !this.consentAt,
        };
    },
    computed: {
        summary() {
            if (this.localObjectedAt) return 'возражение';
            if (this.consentAt) return 'уведомлён';
            return 'требуется отметка';
        },
    },
    watch: {
        objectedAt(value) {
            this.localObjectedAt = value;
        },
        source(value) {
            this.localSource = value;
        },
    },
    methods: {
        formatDate(value) {
            if (!value) return '';
            const d = new Date(value);
            return Number.isNaN(d.getTime()) ? '' : d.toLocaleDateString('ru-RU');
        },
        async submit() {
            const source = this.draftSource.trim();
            if (!source || this.busy || !this.employeeId) return;
            this.busy = true;
            try {
                await setEmployeeObjection(this.employeeId, source);
                this.localObjectedAt = new Date().toISOString();
                this.localSource = source;
                this.draftSource = '';
                useDeletionsStore().notify({ prefix: 'Возражение отмечено, пропуска аннулированы' });
                this.$emit('changed');
            } catch (e) {
                useDeletionsStore().notify({ prefix: `Не удалось отметить возражение: ${e.message}` });
            } finally {
                this.busy = false;
            }
        },
        async clear() {
            if (this.busy || !this.employeeId) return;
            this.busy = true;
            try {
                await clearEmployeeObjection(this.employeeId);
                this.localObjectedAt = null;
                this.localSource = '';
                useDeletionsStore().notify({ prefix: 'Отметка о возражении снята' });
                this.$emit('changed');
            } catch (e) {
                useDeletionsStore().notify({ prefix: `Не удалось снять отметку: ${e.message}` });
            } finally {
                this.busy = false;
            }
        },
    },
};
</script>

<style scoped>
.pd-section {
    margin-top: 15px;
    padding-top: 15px;
    border-top: 1px solid var(--border);
}

.pd-section__toggle {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 0;
    background: none;
    border: none;
    cursor: pointer;
    color: var(--text);
    font-size: 13px;
    font-weight: 600;
    text-align: left;
}

.pd-section__title {
    flex: 1;
}

.pd-section__badge {
    font-size: 11px;
    font-weight: 400;
    color: var(--text-muted);
}

/* Возражение видно на свёрнутом разделе: состояние меняет правила работы с записью,
   и открывать раздел ради этого никто не станет. */
.pd-section__badge--alert {
    color: var(--danger-text);
    font-weight: 600;
}

.pd-section__chevron {
    font-size: 14px;
    color: var(--text-muted);
}

.pd-section__body {
    margin-top: 12px;
}

.pd-section__objection {
    margin-top: 14px;
}

.pd-section__note {
    margin: 0 0 8px;
    padding: 10px 14px;
    border-radius: var(--radius-md);
    background: var(--danger-bg);
    color: var(--danger-text);
    font-size: 12px;
    line-height: 1.45;
}

.pd-section__muted {
    margin: 6px 0 10px;
    font-size: 11px;
    line-height: 1.4;
    color: var(--text-muted);
}

.pd-section__action {
    margin-top: 10px;
}
</style>
