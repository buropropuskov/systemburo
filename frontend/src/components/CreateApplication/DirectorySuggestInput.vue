<template>
  <div
    class="ds-field"
    :class="{ 'ds-field--readonly': !editable }"
  >
    <label class="input__label">
      {{ label }}
      <span
        v-if="required"
        class="required"
      >*</span>
    </label>

    <input
      ref="input"
      class="input"
      :class="{ 'input--error': error, 'input--readonly': !editable }"
      :placeholder="editable ? placeholder : ''"
      :value="modelValue"
      :readonly="!editable"
      :data-testid="testid"
      autocomplete="off"
      @input="onInput($event.target.value)"
      @focus="onFocus"
      @blur="onBlur"
      @keydown.down.prevent="move(1)"
      @keydown.up.prevent="move(-1)"
      @keydown.enter="onEnter"
      @keydown.esc="close"
    >

    <span
      v-if="hint"
      class="input__hint"
    >{{ hint }}</span>

    <ul
      v-if="open && suggestions.length"
      class="ds-list"
      :data-testid="`${testid}-list`"
    >
      <li
        v-for="(item, index) in suggestions"
        :key="item.id"
        class="ds-item"
        :class="{ 'ds-item--active': index === activeIndex }"
        :data-testid="`${testid}-option`"
        @mousedown.prevent="select(item)"
        @mouseenter="activeIndex = index"
      >
        {{ item.name }}
      </li>
    </ul>

    <div
      v-if="error"
      class="error-message"
    >
      {{ error }}
    </div>
  </div>
</template>

<script>
/**
 * Поле наименования справочника со подсказками (#1437).
 *
 * Без права ручного ввода (editable = false) поле только показывает значение из профиля:
 * подсказки не запрашиваются, организацию заявки всё равно определит сервер по её id.
 *
 * Свободный ввод сбрасывает выбранную запись (emit select с null): текст, набранный
 * руками, может не отвечать ни одной записи справочника, и подача обязана уйти
 * наименованием, а не чужим id.
 */
export default {
    name: 'DirectorySuggestInput',
    props: {
        modelValue: { type: String, default: '' },
        label: { type: String, required: true },
        placeholder: { type: String, default: '' },
        hint: { type: String, default: '' },
        error: { type: String, default: '' },
        required: { type: Boolean, default: false },
        editable: { type: Boolean, default: false },
        testid: { type: String, required: true },
        /** Загрузчик подсказок: (query) => Promise<[{ id, name }]>. */
        fetcher: { type: Function, required: true }
    },
    emits: ['update:modelValue', 'select', 'validate'],
    data() {
        return {
            suggestions: [],
            open: false,
            activeIndex: -1,
            debounceTimer: null,
            blurTimer: null,
            // Токен последовательности: набор быстрее ответа, и медленный ответ от
            // предыдущей буквы не должен затирать подсказки текущего запроса.
            fetchSeq: 0
        };
    },
    beforeUnmount() {
        clearTimeout(this.debounceTimer);
        clearTimeout(this.blurTimer);
    },
    methods: {
        onInput(value) {
            this.$emit('update:modelValue', value);
            // Правка руками рвёт связь с выбранной записью: id больше не отвечает тексту.
            this.$emit('select', null);
            this.scheduleFetch(value);
        },

        onFocus() {
            clearTimeout(this.blurTimer);
            if (this.suggestions.length) this.open = true;
        },

        onBlur() {
            this.$emit('validate');
            // Закрытие откладываем: mousedown по подсказке приходит после blur, а
            // мгновенное закрытие съело бы выбор (порядок событий браузеро-зависим).
            this.blurTimer = setTimeout(() => { this.open = false; }, 200);
        },

        scheduleFetch(value) {
            clearTimeout(this.debounceTimer);
            if (!this.editable) return;
            const query = (value || '').trim();
            // Бэк отсекает короткий запрос сам, но лишний круг по сети не нужен.
            if (query.length < 3) {
                this.suggestions = [];
                this.open = false;
                return;
            }
            this.debounceTimer = setTimeout(() => this.fetchSuggestions(query), 250);
        },

        async fetchSuggestions(query) {
            const seq = ++this.fetchSeq;
            try {
                const items = await this.fetcher(query);
                if (seq !== this.fetchSeq) return;
                this.suggestions = Array.isArray(items) ? items : [];
                this.activeIndex = -1;
                this.open = this.suggestions.length > 0;
            } catch {
                // Подсказка - вспомогательная: сбой сети не должен ни ломать ввод, ни
                // сыпать уведомлениями поверх формы. Пользователь просто печатает дальше.
                if (seq !== this.fetchSeq) return;
                this.suggestions = [];
                this.open = false;
            }
        },

        move(step) {
            if (!this.open || !this.suggestions.length) return;
            const last = this.suggestions.length - 1;
            const next = this.activeIndex + step;
            this.activeIndex = next < 0 ? last : next > last ? 0 : next;
        },

        onEnter(event) {
            if (!this.open || this.activeIndex < 0) return;
            // Enter выбирает подсказку, а не отправляет форму.
            event.preventDefault();
            this.select(this.suggestions[this.activeIndex]);
        },

        select(item) {
            clearTimeout(this.blurTimer);
            this.$emit('update:modelValue', item.name);
            this.$emit('select', item);
            this.close();
            this.$emit('validate');
        },

        close() {
            this.open = false;
            this.activeIndex = -1;
        }
    }
}
</script>

<style scoped>
.ds-field {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 5px;
    position: relative;
}

.input__label {
    font-size: 13px;
    color: var(--text-muted);
}

.input__hint {
    font-size: 11px;
    color: var(--text-muted);
}

.required {
    color: var(--danger-text);
}

.input {
    width: 100%;
    height: 40px;
    border: 1px solid var(--border);
    outline: none;
    background: var(--surface);
    border-radius: 15px;
    padding: 5px 10px;
}

.input--readonly {
    background: var(--surface-2);
    color: var(--text-muted);
    cursor: default;
}

.input--error {
    border-color: var(--danger);
}

.ds-list {
    position: absolute;
    top: 68px;
    left: 0;
    right: 0;
    z-index: 20;
    margin: 0;
    padding: 4px;
    list-style: none;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 15px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
    max-height: 200px;
    overflow-y: auto;
}

.ds-item {
    padding: 8px 10px;
    border-radius: 10px;
    cursor: pointer;
    font-size: 14px;
    transition: background-color 150ms ease;
}

.ds-item--active,
.ds-item:hover {
    background: var(--surface-2);
}

.error-message {
    font-size: 11px;
    color: var(--danger-text);
    position: absolute;
    bottom: -15px;
    left: 0;
}
</style>
