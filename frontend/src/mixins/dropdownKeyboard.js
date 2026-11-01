// Клавиатура штатного списка. Слоты с произвольными действиями сохраняют свою модель.
export default {
  data() {
    return { activeOption: 0, keyboardQuery: '', keyboardQueryAt: 0, keyboardIntent: null };
  },
  computed: {
    optionsId() { return `base-dropdown-options-${this.$.uid}`; },
  },
  watch: {
    isOpen(open) {
      this.keyboardQuery = '';
      const intent = this.keyboardIntent;
      this.keyboardIntent = null;
      if (open) {
        const selected = this.filteredOptions.findIndex(this.isSelected);
        this.activeOption = Math.max(0, selected);
        if (intent) this.$nextTick(() => {
          if (this.isOpen && document.activeElement === this.dropdownTrigger()) this.focusInitialOption(intent);
        });
      }
    },
    filteredOptions() {
      const previous = document.activeElement;
      const ownedFocus = this.$refs.options?.contains(previous);
      this.$nextTick(() => {
        const index = this.optionButtons().indexOf(previous);
        this.activeOption = index >= 0 ? index
          : Math.max(0, Math.min(this.activeOption, this.filteredOptions.length - 1));
        if (ownedFocus && index < 0) this.focusOption(this.activeOption);
      });
    },
  },
  methods: {
    optionButtons() {
      return Array.from(this.$refs.options?.querySelectorAll('.base-dropdown__item') || []);
    },
    dropdownTrigger() {
      return this.$refs.triggerButton || this.$refs.dropdown?.querySelector('button');
    },
    focusOption(index) {
      const buttons = this.optionButtons();
      if (!buttons.length) {
        (this.$refs.searchInput || this.dropdownTrigger() || this.$refs.options)?.focus({ preventScroll: true });
        return;
      }
      this.activeOption = Math.max(0, Math.min(index, buttons.length - 1));
      const button = buttons[this.activeOption];
      button.focus({ preventScroll: true });
      button.scrollIntoView?.({ block: 'nearest', inline: 'nearest', behavior: 'instant' });
    },
    focusInitialOption(intent) {
      const selected = this.filteredOptions.findIndex(this.isSelected);
      this.focusOption(selected >= 0 ? selected : intent === 'last' ? this.filteredOptions.length - 1 : 0);
    },
    onTriggerKeydown(event) {
      if (this.disabled || this.inline || this.$slots.content || event.defaultPrevented
        || event.altKey || event.ctrlKey || event.metaKey || event.isComposing
        || this.$refs.menu?.contains(event.target)) return;
      if (!['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(event.key)) return;
      const custom = Boolean(this.$slots.trigger);
      const intent = event.key === 'ArrowUp' ? 'last' : 'first';
      this.keyboardIntent = intent;
      // Нативное нажатие принадлежит владельцу слота: он может готовить контекст
      // асинхронно либо открыть диалог напрямую. Фокус переносим лишь после isOpen.
      if (custom && ['Enter', ' '].includes(event.key)) return;
      if (custom && !this.isOpen && !this.dropdownTrigger()?.hasAttribute('aria-haspopup')) {
        this.keyboardIntent = null;
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      if (this.isOpen) {
        this.keyboardIntent = null;
        this.focusInitialOption(intent);
      } else if (custom) this.dropdownTrigger()?.click();
      else this.open();
    },
    onMenuKeydown(event) {
      // Вложенный список отвечает за свои клавиши, даже без Teleport.
      if (event.target.closest('.base-dropdown__menu') !== this.$refs.menu) return;
      if (this.$slots.content) return;
      if (event.key === 'Tab' && this.isOpen && !this.inline) {
        // Пусть браузер/модалка продолжит Tab от исходной кнопки, а не от body-портала.
        if (!event.defaultPrevented) this.dropdownTrigger()?.focus({ preventScroll: true });
        this.close();
        return;
      }
      if (event.altKey || event.ctrlKey || event.metaKey || event.isComposing) return;
      const buttons = this.optionButtons();
      const index = buttons.indexOf(event.target.closest('.base-dropdown__item'));
      const inSearch = event.target === this.$refs.searchInput;
      if (index < 0 && !inSearch) return;
      const key = event.key;
      let next;
      if (key === 'ArrowDown') next = inSearch ? 0 : Math.min(index + 1, buttons.length - 1);
      else if (key === 'ArrowUp') next = inSearch ? buttons.length - 1 : Math.max(0, index - 1);
      else if (!inSearch && key === 'Home') next = 0;
      else if (!inSearch && key === 'End') next = buttons.length - 1;
      else if (!inSearch && key.length === 1 && key !== ' ') {
        const now = Date.now(), letter = key.toLocaleLowerCase();
        const previous = now - this.keyboardQueryAt < 700 ? this.keyboardQuery : '';
        this.keyboardQuery = previous === letter ? letter : previous + letter;
        this.keyboardQueryAt = now;
        const start = this.keyboardQuery.length === 1 ? index + 1 : index;
        for (let offset = 0; offset < buttons.length; offset++) {
          const candidate = (start + offset) % buttons.length;
          const label = String(this.filteredOptions[candidate][this.labelKey] ?? '').toLocaleLowerCase();
          if (label.startsWith(this.keyboardQuery)) { next = candidate; break; }
        }
      }
      if (next === undefined) return;
      if (key.length !== 1) this.keyboardQuery = '';
      event.preventDefault();
      event.stopPropagation();
      this.focusOption(next);
    },
  },
};
