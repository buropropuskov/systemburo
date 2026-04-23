<template>
  <div class="text-constructor">
    <div class="editor-toolbar">
      <div class="toolbar-group">
        <!-- УБРАН КНОПКА "B" -->
        <button 
          @click="formatText('italic')" 
          type="button" 
          class="toolbar-btn"
          data-tooltip="Курсив"
        >
          <em>I</em>
        </button>
        <button 
          @click="formatText('underline')" 
          type="button" 
          class="toolbar-btn"
          data-tooltip="Подчеркивание"
        >
          <u>U</u>
        </button>
      </div>

      <div class="toolbar-group lists-group">
        <button 
          @click="insertList('ul')" 
          type="button" 
          class="toolbar-btn"
          data-tooltip="Маркированный список"
        >
          • L
        </button>
        <button 
          @click="insertList('ol')" 
          type="button" 
          class="toolbar-btn"
          data-tooltip="Нумерованный список"
        >
          1. L
        </button>
        <button 
          @click="insertListItem()" 
          type="button" 
          class="toolbar-btn"
          data-tooltip="Добавить элемент списка"
        >
          Li
        </button>
      </div>

      <div class="toolbar-group">
        <button 
          @click="insertHeading('h1')" 
          type="button" 
          class="toolbar-btn"
          data-tooltip="Заголовок h1"
        >
          h1
        </button>
        <button 
          @click="insertHeading('h2')" 
          type="button" 
          class="toolbar-btn"
          data-tooltip="Заголовок h2"
        >
          h2
        </button>
      </div>

      <div class="toolbar-group">
        <div class="custom-select" @mouseenter="showTooltip = true" @mouseleave="showTooltip = false">
          <div 
            class="select-header" 
            @click="toggleFontSizeDropdown" 
            :data-tooltip="fontSizeDropdownOpen ? '' : 'Размер шрифта'"
          >
            <span class="select-value">{{ selectedFontSize }}</span>
            <img src="@/assets/icons/arrow.png" class="select-arrow" :class="{ rotated: fontSizeDropdownOpen }" />
          </div>
          <div v-if="fontSizeDropdownOpen" class="select-dropdown">
            <div 
              v-for="size in fontSizes" 
              :key="size"
              class="select-option"
              :class="{ active: selectedFontSize === size }"
              @click="selectFontSize(size)"
            >
              {{ size }}
            </div>
          </div>
        </div>
      </div>

      <div class="toolbar-group">
        <div class="custom-select fixed-width-select" @mouseenter="showTooltip = true" @mouseleave="showTooltip = false">
          <div 
            class="select-header" 
            @click="toggleFontWeightDropdown" 
            :data-tooltip="fontWeightDropdownOpen ? '' : 'Жирность шрифта'"
          >
            <span class="select-value">{{ selectedFontWeight.label }}</span>
            <img src="@/assets/icons/arrow.png" class="select-arrow" :class="{ rotated: fontWeightDropdownOpen }" />
          </div>
          <div v-if="fontWeightDropdownOpen" class="select-dropdown">
            <div 
              v-for="weight in fontWeights" 
              :key="weight.value"
              class="select-option"
              :class="{ active: selectedFontWeight.value === weight.value }"
              :style="{ fontWeight: weight.value }"
              @click="selectFontWeight(weight)"
            >
              {{ weight.label }}
            </div>
          </div>
        </div>
      </div>

      <div class="toolbar-group">
        <button 
          @click="insertColor('black-text')" 
          type="button" 
          class="toolbar-btn color-btn black-text"
          data-tooltip="Черный"
        >
          A
        </button>
        <button 
          @click="insertColor('red-text')" 
          type="button" 
          class="toolbar-btn color-btn red-text"
          data-tooltip="Красный"
        >
          A
        </button>
        <button 
          @click="insertColor('green-text')" 
          type="button" 
          class="toolbar-btn color-btn green-text"
          data-tooltip="Зеленый"
        >
          A
        </button>
        <button 
          @click="insertColor('blue-text')" 
          type="button" 
          class="toolbar-btn color-btn blue-text"
          data-tooltip="Синий"
        >
          A
        </button>
      </div>

      <div class="toolbar-group">
        <button 
          @click="insertBreak()" 
          type="button" 
          class="toolbar-btn"
          data-tooltip="Отступ"
        >
          ↵
        </button>
      </div>

      <div class="toolbar-group">
        <button 
          @click="undo()" 
          type="button" 
          class="toolbar-btn undo-btn"
          data-tooltip="Назад (Ctrl+Z)"
          :disabled="historyIndex === 0"
        >
          ↶
        </button>
      </div>
    </div>

    <div class="textarea-container">
      <textarea 
        :value="modelValue"
        @input="handleInput"
        @keydown.ctrl.z.prevent="handleCtrlZ"
        :placeholder="placeholder"
        :rows="rows"
        class="constructor-textarea"
        ref="textarea"
      ></textarea>
      <div class="resize-handle" @mousedown="startResize"></div>
    </div>

    <div class="editor-preview" v-if="modelValue">
      <div class="preview-header">
        <h5>Предпросмотр:</h5>
      </div>
      <div class="preview-content-container">
        <div class="preview-content" ref="previewContent" v-html="sanitizedContent"></div>
        <div class="resize-handle preview-resize" @mousedown="startPreviewResize"></div>
      </div>
    </div>
  </div>
</template>

<script>
import { sanitizeHtml } from '@/utils/sanitize';

export default {
  name: 'TextConstructor',
  props: {
    modelValue: {
      type: String,
      default: ''
    },
    placeholder: {
      type: String,
      default: 'Введите текст...'
    },
    rows: {
      type: Number,
      default: 4
    }
  },
  emits: ['update:modelValue'],
  data() {
    return {
      selectedFontSize: '14px',
      fontSizeDropdownOpen: false,
      fontSizes: ['10px', '12px', '14px', '16px', '18px', '20px'],
      fontWeightDropdownOpen: false,
      selectedFontWeight: { label: 'Regular', value: '400' },
      fontWeights: [
        { label: 'Black', value: '900' },
        { label: 'Bold', value: '600' },
        { label: 'Medium', value: '500' },
        { label: 'Regular', value: '400' },
        { label: 'Light', value: '300' }
      ],
      showTooltip: true,
      history: [''],
      historyIndex: 0,
      isResizing: false,
      isPreviewResizing: false,
      startHeight: 0,
      startY: 0,
      previewStartHeight: 0,
      previewMinHeight: 150,
      previewMaxHeight: 350
    };
  },
  computed: {
    sanitizedContent() {
      return sanitizeHtml(this.modelValue);
    }
  },
  methods: {
    handleInput(event) {
      this.addToHistory(event.target.value);
      this.$emit('update:modelValue', event.target.value);
    },

    handleCtrlZ() {
      this.undo();
    },

    addToHistory(value) {
      // Удаляем все элементы после текущего индекса
      this.history = this.history.slice(0, this.historyIndex + 1);
      // Добавляем новое значение
      this.history.push(value);
      this.historyIndex++;
    },

    undo() {
      if (this.historyIndex > 0) {
        this.historyIndex--;
        const previousValue = this.history[this.historyIndex];
        this.$emit('update:modelValue', previousValue);
        
        this.$nextTick(() => {
          this.$refs.textarea.value = previousValue;
          // Сохраняем позицию курсора
          const textarea = this.$refs.textarea;
          const currentScrollPos = textarea.scrollTop;
          textarea.focus();
          // Восстанавливаем позицию скролла
          textarea.scrollTop = currentScrollPos;
        });
      }
    },

    startResize(e) {
      this.isResizing = true;
      this.startY = e.clientY;
      this.startHeight = this.$refs.textarea.offsetHeight;
      
      document.addEventListener('mousemove', this.handleResize);
      document.addEventListener('mouseup', this.stopResize);
      e.preventDefault();
    },

    handleResize(e) {
      if (!this.isResizing) return;
      
      const deltaY = e.clientY - this.startY;
      const newHeight = Math.max(150, Math.min(350, this.startHeight + deltaY));
      
      this.$refs.textarea.style.height = newHeight + 'px';
    },

    stopResize() {
      this.isResizing = false;
      document.removeEventListener('mousemove', this.handleResize);
      document.removeEventListener('mouseup', this.stopResize);
    },

    startPreviewResize(e) {
      this.isPreviewResizing = true;
      this.startY = e.clientY;
      this.previewStartHeight = this.$refs.previewContent.offsetHeight;
      
      document.addEventListener('mousemove', this.handlePreviewResize);
      document.addEventListener('mouseup', this.stopPreviewResize);
      e.preventDefault();
    },

    handlePreviewResize(e) {
      if (!this.isPreviewResizing) return;
      
      const deltaY = e.clientY - this.startY;
      const newHeight = Math.max(this.previewMinHeight, Math.min(this.previewMaxHeight, this.previewStartHeight + deltaY));
      
      this.$refs.previewContent.style.height = newHeight + 'px';
    },

    stopPreviewResize() {
      this.isPreviewResizing = false;
      document.removeEventListener('mousemove', this.handlePreviewResize);
      document.removeEventListener('mouseup', this.stopPreviewResize);
    },

    formatText(type) {
      const textarea = this.$refs.textarea;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const selectedText = textarea.value.substring(start, end);
      
      if (!selectedText) return;
      
      let formattedText = '';
      switch (type) {
        case 'italic':
          formattedText = `<em>${selectedText}</em>`;
          break;
        case 'underline':
          formattedText = `<u>${selectedText}</u>`;
          break;
      }
      
      const newValue = textarea.value.substring(0, start) + formattedText + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);
      
      this.$nextTick(() => {
        const currentScrollPos = textarea.scrollTop;
        textarea.focus();
        textarea.setSelectionRange(start + formattedText.length, start + formattedText.length);
        // Восстанавливаем позицию скролла
        textarea.scrollTop = currentScrollPos;
      });
    },
    
    insertList(type) {
      const textarea = this.$refs.textarea;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const selectedText = textarea.value.substring(start, end);
      
      // Сохраняем позицию скролла
      const currentScrollPos = textarea.scrollTop;
      
      let list;
      if (selectedText) {
        // Если есть выделенный текст, создаем список с этим текстом как элементом
        const items = selectedText.split('\n').filter(item => item.trim());
        const listItems = items.map(item => `  <li>${item.trim()}</li>`).join('\n');
        list = type === 'ul' 
          ? `<ul>\n${listItems}\n</ul>` 
          : `<ol>\n${listItems}\n</ol>`;
      } else {
        // Если нет выделенного текста, создаем пустой список
        list = type === 'ul' 
          ? `<ul>\n  <li>Элемент списка</li>\n</ul>` 
          : `<ol>\n  <li>Элемент списка</li>\n</ol>`;
      }
      
      const newValue = textarea.value.substring(0, start) + list + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);
      
      this.$nextTick(() => {
        textarea.focus();
        // Устанавливаем курсор внутри элемента списка
        const newPosition = start + list.indexOf('<li>') + 4;
        textarea.setSelectionRange(newPosition, newPosition);
        // Восстанавливаем позицию скролла
        textarea.scrollTop = currentScrollPos;
      });
    },
    
    insertListItem() {
      const textarea = this.$refs.textarea;
      const start = textarea.selectionStart;
      const value = textarea.value;
      
      // Сохраняем позицию скролла
      const currentScrollPos = textarea.scrollTop;
      
      // Находим позицию текущей строки
      const textBeforeCursor = value.substring(0, start);
      const lastNewLine = textBeforeCursor.lastIndexOf('\n');
      const currentLineStart = lastNewLine + 1;
      const currentLine = textBeforeCursor.substring(currentLineStart);
      
      let listItem;
      if (currentLine.trim().startsWith('<li>') && currentLine.includes('</li>')) {
        // Если курсор внутри существующего элемента списка, добавляем новый после него
        const lineEnd = textBeforeCursor.indexOf('</li>', currentLineStart) + 5;
        listItem = '\n  <li>Новый элемент списка</li>';
        const newValue = value.substring(0, lineEnd) + listItem + value.substring(lineEnd);
        this.addToHistory(newValue);
        this.$emit('update:modelValue', newValue);
        
        this.$nextTick(() => {
          textarea.focus();
          const newPosition = lineEnd + listItem.length;
          textarea.setSelectionRange(newPosition, newPosition);
          textarea.scrollTop = currentScrollPos;
        });
      } else {
        // Просто добавляем элемент списка
        listItem = '<li>Новый элемент списка</li>';
        const newValue = value.substring(0, start) + listItem + value.substring(start);
        this.addToHistory(newValue);
        this.$emit('update:modelValue', newValue);
        
        this.$nextTick(() => {
          textarea.focus();
          const newPosition = start + listItem.length;
          textarea.setSelectionRange(newPosition, newPosition);
          textarea.scrollTop = currentScrollPos;
        });
      }
    },
    
    insertHeading(level) {
      const textarea = this.$refs.textarea;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const selectedText = textarea.value.substring(start, end) || 'Заголовок';
      
      // Сохраняем позицию скролла
      const currentScrollPos = textarea.scrollTop;
      
      const heading = `<${level} class="heading-${level}">${selectedText}</${level}>`;
      const newValue = textarea.value.substring(0, start) + heading + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);
      
      this.$nextTick(() => {
        textarea.focus();
        // Восстанавливаем позицию скролла
        textarea.scrollTop = currentScrollPos;
      });
    },
    
    insertColor(colorClass) {
      const textarea = this.$refs.textarea;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const selectedText = textarea.value.substring(start, end);
      
      if (!selectedText) return;
      
      // Сохраняем позицию скролла
      const currentScrollPos = textarea.scrollTop;
      
      const coloredText = `<span class="${colorClass}">${selectedText}</span>`;
      const newValue = textarea.value.substring(0, start) + coloredText + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);
      
      this.$nextTick(() => {
        textarea.focus();
        // Восстанавливаем позицию скролла
        textarea.scrollTop = currentScrollPos;
      });
    },
    
    insertBreak() {
      const textarea = this.$refs.textarea;
      const start = textarea.selectionStart;
      
      // Сохраняем позицию скролла
      const currentScrollPos = textarea.scrollTop;
      
      const newValue = textarea.value.substring(0, start) + '<br>' + textarea.value.substring(start);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);
      
      this.$nextTick(() => {
        textarea.focus();
        // Восстанавливаем позицию скролла
        textarea.scrollTop = currentScrollPos;
      });
    },
    
    toggleFontSizeDropdown() {
      this.fontSizeDropdownOpen = !this.fontSizeDropdownOpen;
      this.fontWeightDropdownOpen = false;
      // При открытии дропдауна убираем подсказку
      this.showTooltip = !this.fontSizeDropdownOpen;
    },
    
    selectFontSize(size) {
      this.selectedFontSize = size;
      this.fontSizeDropdownOpen = false;
      this.showTooltip = true;
      this.applyFontSize();
    },
    
    applyFontSize() {
      const textarea = this.$refs.textarea;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const selectedText = textarea.value.substring(start, end);
      
      if (!selectedText) return;
      
      // Сохраняем позицию скролла
      const currentScrollPos = textarea.scrollTop;
      
      const fontSizeClass = `font-size-${this.selectedFontSize.replace('px', '')}`;
      const sizedText = `<span class="${fontSizeClass}">${selectedText}</span>`;
      const newValue = textarea.value.substring(0, start) + sizedText + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);
      
      this.$nextTick(() => {
        textarea.focus();
        // Восстанавливаем позицию скролла
        textarea.scrollTop = currentScrollPos;
      });
    },
    
    toggleFontWeightDropdown() {
      this.fontWeightDropdownOpen = !this.fontWeightDropdownOpen;
      this.fontSizeDropdownOpen = false;
      // При открытии дропдауна убираем подсказку
      this.showTooltip = !this.fontWeightDropdownOpen;
    },
    
    selectFontWeight(weight) {
      this.selectedFontWeight = weight;
      this.fontWeightDropdownOpen = false;
      this.showTooltip = true;
      this.applyFontWeight();
    },
    
    applyFontWeight() {
      const textarea = this.$refs.textarea;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const selectedText = textarea.value.substring(start, end);
      
      if (!selectedText) return;
      
      // Сохраняем позицию скролла
      const currentScrollPos = textarea.scrollTop;
      
      const weightClass = `font-weight-${this.selectedFontWeight.value}`;
      const weightedText = `<span class="${weightClass}">${selectedText}</span>`;
      const newValue = textarea.value.substring(0, start) + weightedText + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);
      
      this.$nextTick(() => {
        textarea.focus();
        // Восстанавливаем позицию скролла
        textarea.scrollTop = currentScrollPos;
      });
    }
  },
  mounted() {
    // Инициализируем историю с текущим значением
    this.history = [this.modelValue];
    this.historyIndex = 0;
    
    // Устанавливаем начальную высоту для preview
    this.$nextTick(() => {
      if (this.$refs.previewContent) {
        this.$refs.previewContent.style.height = this.previewMinHeight + 'px';
        this.$refs.previewContent.style.minHeight = this.previewMinHeight + 'px';
        this.$refs.previewContent.style.maxHeight = this.previewMaxHeight + 'px';
      }
    });
    
    // Закрываем dropdown при клике вне его
    document.addEventListener('click', (e) => {
      if (!this.$el.contains(e.target)) {
        this.fontSizeDropdownOpen = false;
        this.fontWeightDropdownOpen = false;
        this.showTooltip = true;
      }
    });
  },
  watch: {
    modelValue(newValue) {
      // Обновляем историю при изменении modelValue извне
      if (this.history[this.historyIndex] !== newValue) {
        this.history = [newValue];
        this.historyIndex = 0;
      }
    }
  }
};
</script>

<style scoped>
.text-constructor {
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  margin-bottom: 10px;
}

.editor-toolbar {
  display: flex;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid #e6e6e6;
  align-items: center;
  flex-wrap: nowrap;
}

.toolbar-group {
  display: flex;
  gap: 4px;
  align-items: center;
  padding-right: 8px;
  border-right: 1px solid #e0e0e0;
}

.toolbar-group:last-child {
  border-right: none;
  padding-right: 0;
}

/* Фиксированная ширина для группы списков */
.toolbar-group.lists-group {
  width: 130px;
  justify-content: space-between;
}

.toolbar-btn {
  padding: 4px 6px;
  border: 1px solid #ddd;
  background: white;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s ease;
  min-width: 32px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
}

.toolbar-btn:hover:not(:disabled) {
  background: #f0f0f0;
  border-color: #ccc;
}

.toolbar-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.undo-btn {
  padding: 4px 6px;
  background: #f8f9fa;
  border-color: #e9ecef;
}

.undo-btn:hover:not(:disabled) {
  background: #e9ecef;
  border-color: #dee2e6;
}

/* Общие стили для подсказок всех кнопок и селектов */
.toolbar-btn:hover:not(:disabled)::after,
.custom-select:hover .select-header::after {
  content: attr(data-tooltip);
  position: absolute;
  bottom: -30px;
  left: 50%;
  transform: translateX(-50%);
  background: #000;
  color: white;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 11px;
  white-space: nowrap;
  z-index: 1000;
  font-weight: normal;
}

/* Специфичные стили для подсказок селектов */
.custom-select {
  position: relative;
  width: fit-content;
}

.custom-select .select-header::after {
  content: none;
}

.custom-select:hover .select-header::after {
  content: attr(data-tooltip);
}

/* Убираем подсказку когда дропдаун открыт */
.custom-select:hover .select-header[data-tooltip=""]::after {
  content: none;
}

/* Фиксированная ширина для селектора жирности */
.custom-select.fixed-width-select {
  width: 75px;
}

.color-btn {
  font-weight: bold;
}

.black-text { color: #000 !important; }
.red-text { color: #FF0000 !important; }
.green-text { color: #079D1D !important; }
.blue-text { color: #4F5BDF !important; }

/* Стили для кастомного select */
.custom-select {
  position: relative;
  width: fit-content;
}

.select-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px;
  border: 1px solid #ddd;
  border-radius: 4px;
  background: white;
  cursor: pointer;
  font-size: 12px;
  height: 28px;
  transition: all 0.2s ease;
  position: relative;
  width: 100%;
}

.select-header:hover {
  border-color: #ccc;
  background: #f8f9fa;
}

.select-value {
  color: #000;
}

.select-arrow {
  width: 5px;
  height: 5px;
  transition: transform 0.2s ease;
  margin-left: 4px;
  transform: rotate(90deg);
}

.select-arrow.rotated {
  transform: rotate(-90deg);
}

.select-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: white;
  border: 1px solid #ddd;
  border-radius: 4px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  z-index: 1000;
  margin-top: 2px;
  max-height: 200px;
  overflow-y: auto;
}

.select-option {
  padding: 6px 8px;
  font-size: 12px;
  cursor: pointer;
  transition: background-color 0.2s ease;
  color: #000;
}

.select-option:hover {
  background: #f0f0f0;
}

.select-option.active {
  background: #4F5BDF;
  color: white;
}

.textarea-container {
  position: relative;
  overflow: hidden;
}

.constructor-textarea {
  width: 100%;
  padding: 12px;
  border: none;
  font-family: 'Courier New', monospace;
  font-size: 14px;
  line-height: 1.4;
  border-radius: 0;
  min-height: 150px;
  max-height: 350px;
  resize: none;
  display: block;
  overflow-y: auto;
}

.constructor-textarea:focus {
  outline: none;
}

/* Скрываем скроллбар */
.constructor-textarea::-webkit-scrollbar {
  display: none;
}

.constructor-textarea {
  -ms-overflow-style: none;
  scrollbar-width: none;
}

.resize-handle {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 8px;
  background: transparent;
  cursor: ns-resize;
  z-index: 10;
}

.resize-handle:hover {
  background: #4F5BDF;
  opacity: 0.3;
}

.resize-handle:active {
  background: #4F5BDF;
  opacity: 0.5;
}

.editor-preview {
  border-top: 1px solid #e6e6e6;
  background: #fafafa;
  border-radius: 0 0 15px 15px;
}

.preview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 12px 8px 12px;
}

.preview-header h5 {
  margin: 0;
  color: #000;
  font-size: 0.9em;
}

.preview-content-container {
  position: relative;
  padding: 0 12px 12px 12px;
}

.preview-content {
  background: white;
  padding: 12px;
  border-radius: 6px;
  border: 1px solid #e6e6e6;
  overflow-y: auto;
  min-height: 150px;
  max-height: 350px;
  resize: none;
}

.preview-resize {
  position: absolute;
  bottom: 12px;
  left: 12px;
  right: 12px;
  height: 8px;
  background: transparent;
  cursor: ns-resize;
  z-index: 10;
}

.preview-resize:hover {
  background: #4F5BDF;
  opacity: 0.3;
}

.preview-resize:active {
  background: #4F5BDF;
  opacity: 0.5;
}

/* Классы для размеров шрифта */
.font-size-10 { font-size: 10px !important; }
.font-size-12 { font-size: 12px !important; }
.font-size-14 { font-size: 14px !important; }
.font-size-16 { font-size: 16px !important; }
.font-size-18 { font-size: 18px !important; }
.font-size-20 { font-size: 20px !important; }

/* Классы для жирности шрифта */
.font-weight-300 { font-weight: 300 !important; }
.font-weight-400 { font-weight: 400 !important; }
.font-weight-500 { font-weight: 500 !important; }
.font-weight-600 { font-weight: 600 !important; }
.font-weight-900 { font-weight: 900 !important; }

/* Классы для цветов текста */
.black-text { color: #000 !important; }
.red-text { color: #FF0000 !important; }
.green-text { color: #079D1D !important; }
.blue-text { color: #4F5BDF !important; }

/* Стили для заголовков с разными шрифтами */
.heading-h1,
.heading-h1 :deep(*) { 
  font-size: 24px !important; 
  font-weight: 700 !important;
  color: #000 !important;
  margin: 16px 0 8px 0 !important;
  line-height: 1.2 !important;
}

.heading-h2,
.heading-h2 :deep(*) { 
  font-size: 20px !important; 
  font-weight: 600 !important;
  color: #000 !important;
  margin: 14px 0 6px 0 !important;
  line-height: 1.3 !important;
}

/* Ограничение размеров шрифта в preview */
.preview-content :deep(*) {
  font-size: 14px !important;
  max-font-size: 20px !important;
  min-font-size: 10px !important;
}

.preview-content :deep(.font-size-10) { font-size: 10px !important; }
.preview-content :deep(.font-size-12) { font-size: 12px !important; }
.preview-content :deep(.font-size-14) { font-size: 14px !important; }
.preview-content :deep(.font-size-16) { font-size: 16px !important; }
.preview-content :deep(.font-size-18) { font-size: 18px !important; }
.preview-content :deep(.font-size-20) { font-size: 20px !important; }

.preview-content :deep(.font-weight-300) { font-weight: 300 !important; }
.preview-content :deep(.font-weight-400) { font-weight: 400 !important; }
.preview-content :deep(.font-weight-500) { font-weight: 500 !important; }
.preview-content :deep(.font-weight-600) { font-weight: 600 !important; }
.preview-content :deep(.font-weight-900) { font-weight: 900 !important; }

.preview-content :deep(.black-text) { color: #000 !important; }
.preview-content :deep(.red-text) { color: #FF0000 !important; }
.preview-content :deep(.green-text) { color: #079D1D !important; }
.preview-content :deep(.blue-text) { color: #4F5BDF !important; }

.preview-content :deep(.heading-h1),
.preview-content :deep(.heading-h1 *) { 
  font-size: 24px !important; 
  font-weight: 700 !important;
  color: #000 !important;
  margin: 10px 0 8px 0 !important;
  line-height: 1.2 !important;
}

.preview-content :deep(.heading-h2),
.preview-content :deep(.heading-h2 *) { 
  font-size: 20px !important; 
  font-weight: 600 !important;
  color: #000 !important;
  margin: 8px 0 6px 0 !important;
  line-height: 1.3 !important;
}

/* Специальные стили для strong внутри заголовков */
.heading-h1 strong,
.heading-h2 strong,
.preview-content :deep(.heading-h1 strong),
.preview-content :deep(.heading-h2 strong) {
  font-size: inherit !important;
  font-weight: inherit !important;
  color: inherit !important;
}

/* Отступы для списков в preview */
.preview-content :deep(ul),
.preview-content :deep(ol) {
  padding-left: 24px !important;
}

.preview-content :deep(li) {
  line-height: 1.4 !important;
}

/* Скрываем скроллбар для toolbar */
.editor-toolbar::-webkit-scrollbar {
  display: none;
}

.editor-toolbar {
  -ms-overflow-style: none;
  scrollbar-width: none;
}

@media (max-width: 768px) {
  .editor-toolbar {
    flex-wrap: wrap;
    gap: 4px;
  }
  
  .toolbar-group {
    padding-right: 4px;
    margin-right: 4px;
  }
  
  .toolbar-group.lists-group {
    width: 110px;
  }
  
  .toolbar-btn {
    min-width: 28px;
    height: 26px;
    font-size: 11px;
    padding: 4px 6px;
  }
  
  .custom-select.fixed-width-select {
    width: 70px;
  }
  
  .select-header {
    font-size: 11px;
    padding: 4px 6px;
  }
}
</style>