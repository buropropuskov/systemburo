<template>
  <div class="ds-field">
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

    <transition name="ds-fade">
      <ul
        v-if="open && (suggestions.length || createOption)"
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
        <li
          v-if="createOption"
          class="ds-item"
          :class="{ 'ds-item--active': activeIndex === suggestions.length }"
          :data-testid="`${testid}-option-create`"
          @mousedown.prevent="acceptCreateOption"
          @mouseenter="activeIndex = suggestions.length"
        >
          {{ createOption }}
        </li>
      </ul>
    </transition>

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
 * Минимум символов для пункта «Создать». Тот же порог, что у поиска похожих записей на
 * бэке: по одной-двум буквам предлагать «завести наименование» рано, человек ещё печатает.
 * Держится здесь, а не выводится из ответа сервера - требование про порог должно читаться
 * в коде компонента, а не следовать из того, с какого символа бэк считает признаки.
 */
const createOptionMinLength = 3;

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
            fetchSeq: 0,
            // Каноничное оформление введённого текста и признак «такое наименование в
            // справочнике уже есть» - оба приходят с подсказками (#1437). Канон
            // подставляется в поле при потере фокуса, а не во время набора: иначе
            // подстановка дёргала бы каретку на каждой букве.
            canonical: '',
            matched: null,
            // Ключа дедупликации у наименования нет (одни кавычки или точки) - подача
            // такое отклонит, поэтому предупреждение говорит другое.
            degenerate: false,
            // Текст, к которому относятся canonical и matched: пока пользователь
            // печатает дальше, старый ответ уже ничего не описывает.
            answeredFor: '',
            // Канон, который пользователь уже принял пунктом «Создать».
            acceptedCanonical: ''
        };
    },
    computed: {
        /**
         * Пункт «Создать» в выпадающем окне: каноничное написание введённого текста.
         * Показывается, когда бэк ответил, что такого наименования в справочнике нет -
         * тогда выбирать нечего, и единственное осмысленное действие это завести запись.
         * Клик ставит аккуратное написание сразу, не дожидаясь ухода из поля.
         *
         * Не показывается, когда наименование уже есть (надо выбрать его из списка), когда
         * из ввода записи не выйдет (одни кавычки или дефисы) и пока ответ не пришёл.
         */
        createOption() {
            if (!this.editable || this.degenerate || this.matched !== false) return '';
            const value = (this.modelValue || '').trim();
            if (value.length < createOptionMinLength || value !== this.answeredFor) return '';
            // Уже принятый канон второй раз не предлагаем: иначе каждый возврат в поле
            // снова открывает окно с тем же пунктом, по которому человек только что кликнул.
            if (value === this.acceptedCanonical) return '';
            return this.canonical || value;
        }
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
            if (this.suggestions.length || this.createOption) this.open = true;
        },

        onBlur() {
            this.applyCanonical();
            this.$emit('validate');
            // Закрытие откладываем: mousedown по подсказке приходит после blur, а
            // мгновенное закрытие съело бы выбор (порядок событий браузеро-зависим).
            this.blurTimer = setTimeout(() => { this.open = false; }, 200);
        },

        /**
         * Подставляет каноничное оформление в поле: «ооо "братишк» -> «ООО "Братишк"».
         * Только при потере фокуса и только для того текста, на который канон и получен -
         * иначе подстановка затирала бы то, что человек ещё печатает. Значение остаётся
         * редактируемым: бэк применит те же правила повторно, поэтому правка руками
         * ничего не ломает.
         */
        applyCanonical() {
            const value = (this.modelValue || '').trim();
            if (!this.editable || !value) return;
            if (value !== this.answeredFor || !this.canonical || this.canonical === value) return;
            this.$emit('update:modelValue', this.canonical);
            this.answeredFor = this.canonical;
        },

        scheduleFetch(value) {
            clearTimeout(this.debounceTimer);
            if (!this.editable) return;
            const query = (value || '').trim();
            this.resetAnswer();
            // Подсказки бэк отдаёт от трёх символов, но канон и признак совпадения нужны
            // и на коротком наименовании: их считают из той же строки.
            if (!query) {
                this.suggestions = [];
                this.open = false;
                return;
            }
            this.debounceTimer = setTimeout(() => this.fetchSuggestions(query), 250);
        },

        async fetchSuggestions(query) {
            const seq = ++this.fetchSeq;
            try {
                const answer = await this.fetcher(query);
                if (seq !== this.fetchSeq) return;
                this.suggestions = Array.isArray(answer?.items) ? answer.items : [];
                this.canonical = answer?.canonical || '';
                this.matched = answer?.matched ?? null;
                this.degenerate = answer?.degenerate === true;
                this.answeredFor = query;
                this.activeIndex = -1;
                // Окно показываем и без похожих записей: в нём остаётся пункт «Создать»,
                // по которому видно, что именно уйдёт в справочник.
                this.open = this.suggestions.length > 0 || Boolean(this.createOption);
            } catch {
                // Подсказка - вспомогательная: сбой сети не должен ни ломать ввод, ни
                // сыпать уведомлениями поверх формы. Пользователь просто печатает дальше.
                if (seq !== this.fetchSeq) return;
                this.suggestions = [];
                this.resetAnswer();
                this.open = false;
            }
        },

        // Пока ответа на текущий текст нет, канон и признак совпадения относятся к
        // прошлому вводу: держать их - значит подставить чужое оформление или показать
        // предупреждение о наименовании, которого в поле уже нет.
        resetAnswer() {
            this.canonical = '';
            this.matched = null;
            this.degenerate = false;
            this.answeredFor = '';
        },

        move(step) {
            // Пункт «Создать» - последняя позиция списка, поэтому стрелки ходят по
            // подсказкам И по нему: иначе с клавиатуры канон не принять.
            const options = this.suggestions.length + (this.createOption ? 1 : 0);
            if (!this.open || !options) return;
            const last = options - 1;
            const next = this.activeIndex + step;
            this.activeIndex = next < 0 ? last : next > last ? 0 : next;
        },

        onEnter(event) {
            if (!this.open || this.activeIndex < 0) return;
            // Enter выбирает подсказку или принимает канон, а не отправляет форму.
            event.preventDefault();
            if (this.activeIndex < this.suggestions.length) {
                this.select(this.suggestions[this.activeIndex]);
                return;
            }
            this.acceptCreateOption();
        },

        /**
         * Принимает пункт «Создать»: ставит каноничное написание в поле. Запись остаётся
         * ненайденной, поэтому связь с id не появляется - подача уйдёт наименованием, и
         * справочник пополнится «на проверке», как и раньше.
         */
        acceptCreateOption() {
            clearTimeout(this.blurTimer);
            const canonical = this.createOption;
            if (!canonical) return;
            this.$emit('update:modelValue', canonical);
            this.$emit('select', null);
            this.canonical = canonical;
            this.answeredFor = canonical;
            this.acceptedCanonical = canonical;
            this.close();
            this.$emit('validate');
        },

        select(item) {
            clearTimeout(this.blurTimer);
            this.$emit('update:modelValue', item.name);
            this.$emit('select', item);
            // Наименование из справочника уже аккуратное и заведомо существует:
            // ни канонизировать его, ни предупреждать о проверке не нужно.
            this.canonical = item.name;
            this.matched = true;
            this.degenerate = false;
            this.answeredFor = item.name;
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

/* Список появляется и уходит плавно: только opacity и transform, как во всех анимациях
   проекта. Небольшой сдвиг вверх на входе - чтобы окно «раскрывалось» из поля. */
.ds-fade-enter-active,
.ds-fade-leave-active {
    transition: opacity 160ms ease-out, transform 160ms ease-out;
}

.ds-fade-enter-from,
.ds-fade-leave-to {
    opacity: 0;
    transform: translateY(-4px);
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
    box-shadow: 0 3px 10px var(--shadow-drop);
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
