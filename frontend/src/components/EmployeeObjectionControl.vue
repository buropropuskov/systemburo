<template>
  <div class="objection">
    <template v-if="localObjectedAt">
      <p
        class="objection__note"
        data-testid="objection-note"
      >
        Работник возразил против обработки своих персональных данных
        {{ formattedDate }}. Фамилия, имя, должность и документы доступны только для
        чтения, действующие пропуска аннулированы.
      </p>
      <p
        v-if="localSource"
        class="objection__source"
        data-testid="objection-source"
      >
        Основание: {{ localSource }}
      </p>
      <button
        v-if="canManageAll"
        type="button"
        class="lk-button lk-button--secondary objection__action"
        :disabled="busy"
        data-testid="objection-clear"
        @click="clear"
      >
        {{ busy ? 'Снимаем...' : 'Снять отметку' }}
      </button>
      <p
        v-else
        class="objection__hint"
      >
        Снять отметку может администратор бюро по итогам рассмотрения обращения.
      </p>
    </template>

    <template v-else>
      <label
        class="input__label"
        for="objection-source"
      >Возражение против обработки данных</label>
      <p class="objection__hint">
        Отметьте, если работник обратился с требованием прекратить обработку его
        персональных данных. Сведения останутся в системе, но станут доступны только
        для чтения, а пропуска будут аннулированы.
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
        class="lk-button lk-button--danger objection__action"
        :disabled="busy || !draftSource.trim()"
        data-testid="objection-set"
        @click="submit"
      >
        {{ busy ? 'Отмечаем...' : 'Отметить возражение' }}
      </button>
    </template>
  </div>
</template>

<script>
import { setEmployeeObjection, clearEmployeeObjection } from '@/api/employees';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Управление отметкой о возражении субъекта (#2361).
 *
 * Отдельный компонент, а не блок внутри карточки: карточка реестра упёрта в предел
 * размера по всем трём частям сразу, и дописать туда нечего. Заодно управление
 * переиспользуется - тот же блок понадобится в карточке машины, когда возражение
 * заведут и для владельцев транспорта.
 *
 * Ставить отметку может тот же круг, что правит запись: человек скажет о возражении
 * своему работодателю, а не бюро, и обращение иначе потеряется. Снимать - только
 * администратор бюро: возражение адресовано оператору, решение принимает он.
 */
export default {
    name: 'EmployeeObjectionControl',
    props: {
        employeeId: { type: Number, required: true },
        objectedAt: { type: String, default: null },
        source: { type: String, default: '' },
        canManageAll: { type: Boolean, default: false },
    },
    emits: ['changed'],
    data() {
        // Состояние держим у себя, а не ждём перечитывания карточки снаружи: карточка
        // после действия остаётся открытой, и вид обязан смениться сразу. Наверх
        // уходит событие, по которому вью перечитывает список.
        return { draftSource: '', busy: false, localObjectedAt: this.objectedAt, localSource: this.source };
    },
    computed: {
        formattedDate() {
            if (!this.localObjectedAt) return '';
            const d = new Date(this.localObjectedAt);
            return Number.isNaN(d.getTime()) ? '' : d.toLocaleDateString('ru-RU');
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
        async submit() {
            const source = this.draftSource.trim();
            if (!source || this.busy) return;
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
            if (this.busy) return;
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
.objection {
    margin-top: 15px;
    padding-top: 15px;
    border-top: 1px solid var(--border);
}

/* Заливкой, а не только цветом текста: состояние меняет правила работы с записью,
   и потеряться среди служебных подписей карточки оно не должно. */
.objection__note {
    margin: 0 0 8px;
    padding: 10px 14px;
    border-radius: var(--radius-md);
    background: var(--danger-bg);
    color: var(--danger-text);
    font-size: 12px;
    line-height: 1.45;
}

.objection__source {
    margin: 0 0 10px;
    font-size: 11px;
    color: var(--text-muted);
}

.objection__hint {
    margin: 6px 0 10px;
    font-size: 11px;
    line-height: 1.4;
    color: var(--text-muted);
}

.objection__action {
    margin-top: 10px;
}
</style>
