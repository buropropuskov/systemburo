<template>
  <div
    class="text-constructor"
    :class="{ 'is-disabled': disabled }"
  >
    <div class="editor-toolbar">
      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-btn"
          data-tooltip="Курсив"
          :disabled="disabled"
          @click="formatText('italic')"
        >
          <em>I</em>
        </button>
        <button
          type="button"
          class="toolbar-btn"
          data-tooltip="Подчеркивание"
          :disabled="disabled"
          @click="formatText('underline')"
        >
          <u>U</u>
        </button>
      </div>

      <div class="toolbar-group lists-group">
        <button
          type="button"
          class="toolbar-btn"
          data-tooltip="Маркированный список"
          :disabled="disabled"
          @click="insertList('ul')"
        >
          • L
        </button>
        <button
          type="button"
          class="toolbar-btn"
          data-tooltip="Нумерованный список"
          :disabled="disabled"
          @click="insertList('ol')"
        >
          1. L
        </button>
        <button
          type="button"
          class="toolbar-btn"
          data-tooltip="Добавить элемент списка"
          :disabled="disabled"
          @click="insertListItem()"
        >
          Li
        </button>
      </div>

      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-btn"
          data-tooltip="Заголовок h1"
          :disabled="disabled"
          @click="insertHeading('h1')"
        >
          h1
        </button>
        <button
          type="button"
          class="toolbar-btn"
          data-tooltip="Заголовок h2"
          :disabled="disabled"
          @click="insertHeading('h2')"
        >
          h2
        </button>
      </div>

      <div class="toolbar-group">
        <div
          class="custom-select"
          @mouseenter="showTooltip = true"
          @mouseleave="showTooltip = false"
        >
          <div
            class="select-header"
            :data-tooltip="fontSizeDropdownOpen ? '' : 'Размер шрифта'"
            @click="toggleFontSizeDropdown"
          >
            <span class="select-value">{{ selectedFontSize }}</span>
            <img
              src="@/assets/icons/arrow.png"
              class="select-arrow"
              :class="{ rotated: fontSizeDropdownOpen }"
            >
          </div>
          <div
            v-if="fontSizeDropdownOpen"
            class="select-dropdown"
          >
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
        <div
          class="custom-select fixed-width-select"
          @mouseenter="showTooltip = true"
          @mouseleave="showTooltip = false"
        >
          <div
            class="select-header"
            :data-tooltip="fontWeightDropdownOpen ? '' : 'Жирность шрифта'"
            @click="toggleFontWeightDropdown"
          >
            <span class="select-value">{{ selectedFontWeight.label }}</span>
            <img
              src="@/assets/icons/arrow.png"
              class="select-arrow"
              :class="{ rotated: fontWeightDropdownOpen }"
            >
          </div>
          <div
            v-if="fontWeightDropdownOpen"
            class="select-dropdown"
          >
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
          type="button"
          class="toolbar-btn color-btn black-text"
          data-tooltip="Черный"
          :disabled="disabled"
          @click="insertColor('black-text')"
        >
          A
        </button>
        <button
          type="button"
          class="toolbar-btn color-btn red-text"
          data-tooltip="Красный"
          :disabled="disabled"
          @click="insertColor('red-text')"
        >
          A
        </button>
        <button
          type="button"
          class="toolbar-btn color-btn green-text"
          data-tooltip="Зеленый"
          :disabled="disabled"
          @click="insertColor('green-text')"
        >
          A
        </button>
        <button
          type="button"
          class="toolbar-btn color-btn blue-text"
          data-tooltip="Синий"
          :disabled="disabled"
          @click="insertColor('blue-text')"
        >
          A
        </button>
      </div>

      <div
        v-if="!disableImages"
        class="toolbar-group"
      >
        <button
          type="button"
          class="toolbar-btn image-btn"
          data-tooltip="Вставить изображение"
          :disabled="disabled"
          @click="triggerImagePicker"
        >
          <span aria-hidden="true">IMG</span>
        </button>
        <input
          ref="imageInput"
          type="file"
          accept="image/png,image/jpeg,image/gif,image/webp,image/svg+xml"
          class="image-input"
          @change="handleImageSelected"
        >
      </div>

      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-btn"
          data-tooltip="Перенос строки"
          :disabled="disabled"
          @click="insertBreak()"
        >
          ↵
        </button>
      </div>

      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-btn undo-btn"
          data-tooltip="Назад (Ctrl+Z)"
          :disabled="disabled || historyIndex === 0"
          @click="undo()"
        >
          ↶
        </button>
        <button
          type="button"
          class="toolbar-btn preview-btn"
          data-tooltip="Открыть предпросмотр"
          :disabled="!modelValue"
          @click="openPreviewModal"
        >
          ⎚
        </button>
      </div>
    </div>

    <div class="textarea-container">
      <textarea
        ref="textarea"
        :value="modelValue"
        :placeholder="placeholder"
        :rows="rows"
        :disabled="disabled"
        class="constructor-textarea"
        @input="handleInput"
        @keydown.ctrl.z.prevent="handleCtrlZ"
      />
      <div
        class="resize-handle"
        @mousedown="startResize"
      />
    </div>

    <div
      v-if="imageError"
      class="constructor-error"
      role="alert"
    >
      {{ imageError }}
    </div>

    <div
      v-if="imageBlocks.length > 1"
      class="image-blocks"
    >
      <div class="image-blocks__header">
        <span class="image-blocks__title">Расположение изображений</span>
        <span class="image-blocks__hint">перетащите карточку или используйте стрелки</span>
      </div>
      <div class="image-blocks__list">
        <div
          v-for="(block, index) in imageBlocks"
          :key="block.id"
          class="image-block"
          :class="{
            'image-block--dragging': draggingIndex === index,
            'image-block--target': dragOverIndex === index && draggingIndex !== index
          }"
          draggable="true"
          @dragstart="onBlockDragStart(index, $event)"
          @dragover.prevent="onBlockDragOver(index)"
          @dragleave="onBlockDragLeave"
          @drop.prevent="onBlockDrop(index)"
          @dragend="onBlockDragEnd"
        >
          <span class="image-block__handle" aria-hidden="true">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <circle cx="4" cy="3" r="1.4" fill="currentColor" />
              <circle cx="4" cy="7" r="1.4" fill="currentColor" />
              <circle cx="4" cy="11" r="1.4" fill="currentColor" />
              <circle cx="10" cy="3" r="1.4" fill="currentColor" />
              <circle cx="10" cy="7" r="1.4" fill="currentColor" />
              <circle cx="10" cy="11" r="1.4" fill="currentColor" />
            </svg>
          </span>
          <img
            :src="block.src"
            class="image-block__preview"
            :alt="`Изображение ${index + 1}`"
          />
          <span class="image-block__index">#{{ index + 1 }}</span>
          <div class="image-block__actions">
            <button
              type="button"
              class="image-block__btn"
              :disabled="index === 0"
              title="Переместить выше"
              @click="moveImageBlock(index, index - 1)"
            >
              ↑
            </button>
            <button
              type="button"
              class="image-block__btn"
              :disabled="index === imageBlocks.length - 1"
              title="Переместить ниже"
              @click="moveImageBlock(index, index + 1)"
            >
              ↓
            </button>
            <button
              type="button"
              class="image-block__btn image-block__btn--danger"
              title="Удалить"
              @click="removeImageBlock(index)"
            >
              ×
            </button>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="modelValue"
      class="editor-preview"
    >
      <div class="preview-header">
        <h5>Предпросмотр</h5>
        <button
          type="button"
          class="preview-expand-btn"
          @click="openPreviewModal"
        >
          Открыть в окне
        </button>
      </div>
      <div class="preview-content-container">
        <div
          ref="previewContent"
          class="preview-content"
          v-html="sanitizedContent"
        />
        <div
          class="resize-handle preview-resize"
          @mousedown="startPreviewResize"
        />
      </div>
    </div>

    <BaseModal
      :show="previewModalOpen"
      title="Предпросмотр контента"
      width="780px"
      @close="previewModalOpen = false"
    >
      <div
        class="preview-modal-content"
        v-html="sanitizedContent"
      />
    </BaseModal>
  </div>
</template>

<script>
import { sanitizeHtml } from '@/utils/sanitize';
import BaseModal from '@/components/ui/BaseModal.vue';

const DEFAULT_MAX_IMAGE_BYTES = 5 * 1024 * 1024;
const ALLOWED_IMAGE_TYPES = [
  'image/png',
  'image/jpeg',
  'image/gif',
  'image/webp',
  'image/svg+xml'
];

export default {
  name: 'TextConstructor',
  components: { BaseModal },
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
    },
    disabled: {
      type: Boolean,
      default: false
    },
    maxImageBytes: {
      type: Number,
      default: DEFAULT_MAX_IMAGE_BYTES
    },
    disableImages: {
      type: Boolean,
      default: false
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
      previewMaxHeight: 350,
      imageError: '',
      imageErrorTimer: null,
      previewModalOpen: false,
      onDocClick: null,
      draggingIndex: null,
      dragOverIndex: null,
      imageMatchRegex: /<img\b[^>]*>/gi
    };
  },
  computed: {
    sanitizedContent() {
      return sanitizeHtml(this.modelValue);
    },
    imageBlocks() {
      const html = this.modelValue || '';
      const matches = html.match(this.imageMatchRegex) || [];
      return matches.map((tag, idx) => {
        const srcMatch = tag.match(/src=["']([^"']+)["']/i);
        return {
          id: `img-${idx}-${(srcMatch ? srcMatch[1] : '').slice(0, 20)}`,
          tag,
          src: srcMatch ? srcMatch[1] : ''
        };
      });
    }
  },
  watch: {
    modelValue(newValue) {
      if (this.history[this.historyIndex] !== newValue) {
        this.history = [newValue];
        this.historyIndex = 0;
      }
    }
  },
  mounted() {
    this.history = [this.modelValue];
    this.historyIndex = 0;

    this.$nextTick(() => {
      if (this.$refs.previewContent) {
        this.$refs.previewContent.style.height = this.previewMinHeight + 'px';
        this.$refs.previewContent.style.minHeight = this.previewMinHeight + 'px';
        this.$refs.previewContent.style.maxHeight = this.previewMaxHeight + 'px';
      }
    });

    this.onDocClick = (e) => {
      if (!this.$el.contains(e.target)) {
        this.fontSizeDropdownOpen = false;
        this.fontWeightDropdownOpen = false;
        this.showTooltip = true;
      }
    };
    document.addEventListener('click', this.onDocClick);
  },
  beforeUnmount() {
    if (this.onDocClick) {
      document.removeEventListener('click', this.onDocClick);
    }
    document.removeEventListener('mousemove', this.handleResize);
    document.removeEventListener('mouseup', this.stopResize);
    document.removeEventListener('mousemove', this.handlePreviewResize);
    document.removeEventListener('mouseup', this.stopPreviewResize);
    if (this.imageErrorTimer) {
      clearTimeout(this.imageErrorTimer);
    }
  },
  methods: {
    /**
     * Извлекает все <img> теги из modelValue, заменяя их плейсхолдерами,
     * чтобы потом восстановить в произвольном порядке.
     * @returns {{ skeleton: string, imgs: string[] }}
     */
    extractImageSkeleton() {
      const imgs = [];
      const skeleton = (this.modelValue || '').replace(this.imageMatchRegex, (match) => {
        imgs.push(match);
        return ` IMG_${imgs.length - 1} `;
      });
      return { skeleton, imgs };
    },

    /**
     * Восстанавливает HTML по skeleton'у с переупорядоченными изображениями.
     * @param {string} skeleton
     * @param {string[]} imgs
     */
    rebuildHtmlFromSkeleton(skeleton, imgs) {
      return skeleton.replace(/ IMG_(\d+) /g, (_, i) => imgs[Number(i)] || '');
    },

    moveImageBlock(from, to) {
      if (to < 0 || to >= this.imageBlocks.length) return;
      const { skeleton, imgs } = this.extractImageSkeleton();
      const [moved] = imgs.splice(from, 1);
      imgs.splice(to, 0, moved);
      const next = this.rebuildHtmlFromSkeleton(skeleton, imgs);
      this.addToHistory(next);
      this.$emit('update:modelValue', next);
    },

    removeImageBlock(index) {
      const { skeleton, imgs } = this.extractImageSkeleton();
      imgs.splice(index, 1);
      const next = this.rebuildHtmlFromSkeleton(skeleton, imgs);
      this.addToHistory(next);
      this.$emit('update:modelValue', next);
    },

    onBlockDragStart(index, event) {
      this.draggingIndex = index;
      if (event.dataTransfer) {
        event.dataTransfer.effectAllowed = 'move';
      }
    },

    onBlockDragOver(index) {
      this.dragOverIndex = index;
    },

    onBlockDragLeave() {
      this.dragOverIndex = null;
    },

    onBlockDrop(targetIndex) {
      if (this.draggingIndex === null || this.draggingIndex === targetIndex) {
        this.dragOverIndex = null;
        return;
      }
      this.moveImageBlock(this.draggingIndex, targetIndex);
      this.draggingIndex = null;
      this.dragOverIndex = null;
    },

    onBlockDragEnd() {
      this.draggingIndex = null;
      this.dragOverIndex = null;
    },

    handleInput(event) {
      this.addToHistory(event.target.value);
      this.$emit('update:modelValue', event.target.value);
    },

    handleCtrlZ() {
      this.undo();
    },

    addToHistory(value) {
      this.history = this.history.slice(0, this.historyIndex + 1);
      this.history.push(value);
      this.historyIndex++;
    },

    undo() {
      if (this.historyIndex > 0) {
        this.historyIndex--;
        const previousValue = this.history[this.historyIndex];
        this.$emit('update:modelValue', previousValue);

        this.$nextTick(() => {
          if (!this.$refs.textarea) return;
          this.$refs.textarea.value = previousValue;
          const textarea = this.$refs.textarea;
          const currentScrollPos = textarea.scrollTop;
          textarea.focus();
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

    insertAtCursor(text, opts = {}) {
      const textarea = this.$refs.textarea;
      if (!textarea) return;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const currentScrollPos = textarea.scrollTop;
      const newValue = textarea.value.substring(0, start) + text + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);

      this.$nextTick(() => {
        textarea.focus();
        const cursor = opts.selectStart != null
          ? opts.selectStart
          : start + text.length;
        textarea.setSelectionRange(cursor, cursor);
        textarea.scrollTop = currentScrollPos;
      });
    },

    formatText(type) {
      if (this.disabled) return;
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

      const currentScrollPos = textarea.scrollTop;
      const newValue = textarea.value.substring(0, start) + formattedText + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);

      this.$nextTick(() => {
        textarea.focus();
        textarea.setSelectionRange(start + formattedText.length, start + formattedText.length);
        textarea.scrollTop = currentScrollPos;
      });
    },

    insertList(type) {
      if (this.disabled) return;
      const textarea = this.$refs.textarea;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const selectedText = textarea.value.substring(start, end);

      const currentScrollPos = textarea.scrollTop;

      let list;
      if (selectedText) {
        const items = selectedText.split('\n').filter(item => item.trim());
        const listItems = items.map(item => `  <li>${item.trim()}</li>`).join('\n');
        list = type === 'ul'
          ? `<ul>\n${listItems}\n</ul>`
          : `<ol>\n${listItems}\n</ol>`;
      } else {
        list = type === 'ul'
          ? `<ul>\n  <li>Элемент списка</li>\n</ul>`
          : `<ol>\n  <li>Элемент списка</li>\n</ol>`;
      }

      const newValue = textarea.value.substring(0, start) + list + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);

      this.$nextTick(() => {
        textarea.focus();
        const newPosition = start + list.indexOf('<li>') + 4;
        textarea.setSelectionRange(newPosition, newPosition);
        textarea.scrollTop = currentScrollPos;
      });
    },

    insertListItem() {
      if (this.disabled) return;
      const textarea = this.$refs.textarea;
      const start = textarea.selectionStart;
      const value = textarea.value;

      const currentScrollPos = textarea.scrollTop;

      const textBeforeCursor = value.substring(0, start);
      const lastNewLine = textBeforeCursor.lastIndexOf('\n');
      const currentLineStart = lastNewLine + 1;
      const currentLine = textBeforeCursor.substring(currentLineStart);

      let listItem;
      if (currentLine.trim().startsWith('<li>') && currentLine.includes('</li>')) {
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
      if (this.disabled) return;
      const textarea = this.$refs.textarea;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const selectedText = textarea.value.substring(start, end) || 'Заголовок';

      const currentScrollPos = textarea.scrollTop;

      const heading = `<${level} class="heading-${level}">${selectedText}</${level}>`;
      const newValue = textarea.value.substring(0, start) + heading + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);

      this.$nextTick(() => {
        textarea.focus();
        textarea.scrollTop = currentScrollPos;
      });
    },

    insertColor(colorClass) {
      if (this.disabled) return;
      const textarea = this.$refs.textarea;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const selectedText = textarea.value.substring(start, end);

      if (!selectedText) return;

      const currentScrollPos = textarea.scrollTop;

      const coloredText = `<span class="${colorClass}">${selectedText}</span>`;
      const newValue = textarea.value.substring(0, start) + coloredText + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);

      this.$nextTick(() => {
        textarea.focus();
        textarea.scrollTop = currentScrollPos;
      });
    },

    insertBreak() {
      if (this.disabled) return;
      this.insertAtCursor('<br>');
    },

    triggerImagePicker() {
      if (this.disabled) return;
      this.clearImageError();
      if (this.$refs.imageInput) {
        this.$refs.imageInput.value = '';
        this.$refs.imageInput.click();
      }
    },

    /**
     * Обрабатывает выбор файла, валидирует тип и размер,
     * читает в data:URL и вставляет тег <img> в textarea.
     */
    async handleImageSelected(event) {
      const file = event.target?.files?.[0];
      if (!file) return;

      if (!ALLOWED_IMAGE_TYPES.includes(file.type)) {
        this.setImageError('Неподдерживаемый формат. Разрешены: PNG, JPEG, GIF, WEBP, SVG.');
        return;
      }
      if (file.size > this.maxImageBytes) {
        const maxMb = Math.round(this.maxImageBytes / (1024 * 1024));
        this.setImageError(`Файл слишком большой. Максимальный размер: ${maxMb} МБ.`);
        return;
      }

      try {
        const dataUrl = await this.readFileAsDataUrl(file);
        const safeAlt = (file.name || 'image')
          .replace(/[<>"'&]/g, '')
          .slice(0, 100);
        const imgTag = `<img src="${dataUrl}" alt="${safeAlt}" class="constructor-image">`;
        this.insertAtCursor(imgTag);
      } catch (err) {
        this.setImageError('Не удалось прочитать файл. Попробуйте ещё раз.');
        console.error('TextConstructor image read error:', err);
      } finally {
        if (this.$refs.imageInput) {
          this.$refs.imageInput.value = '';
        }
      }
    },

    readFileAsDataUrl(file) {
      return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(String(reader.result));
        reader.onerror = () => reject(reader.error || new Error('FileReader error'));
        reader.readAsDataURL(file);
      });
    },

    setImageError(msg) {
      this.imageError = msg;
      if (this.imageErrorTimer) clearTimeout(this.imageErrorTimer);
      this.imageErrorTimer = setTimeout(() => {
        this.imageError = '';
      }, 4500);
    },

    clearImageError() {
      this.imageError = '';
      if (this.imageErrorTimer) {
        clearTimeout(this.imageErrorTimer);
        this.imageErrorTimer = null;
      }
    },

    openPreviewModal() {
      if (!this.modelValue) return;
      this.previewModalOpen = true;
    },

    toggleFontSizeDropdown() {
      if (this.disabled) return;
      this.fontSizeDropdownOpen = !this.fontSizeDropdownOpen;
      this.fontWeightDropdownOpen = false;
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

      const currentScrollPos = textarea.scrollTop;

      const fontSizeClass = `font-size-${this.selectedFontSize.replace('px', '')}`;
      const sizedText = `<span class="${fontSizeClass}">${selectedText}</span>`;
      const newValue = textarea.value.substring(0, start) + sizedText + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);

      this.$nextTick(() => {
        textarea.focus();
        textarea.scrollTop = currentScrollPos;
      });
    },

    toggleFontWeightDropdown() {
      if (this.disabled) return;
      this.fontWeightDropdownOpen = !this.fontWeightDropdownOpen;
      this.fontSizeDropdownOpen = false;
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

      const currentScrollPos = textarea.scrollTop;

      const weightClass = `font-weight-${this.selectedFontWeight.value}`;
      const weightedText = `<span class="${weightClass}">${selectedText}</span>`;
      const newValue = textarea.value.substring(0, start) + weightedText + textarea.value.substring(end);
      this.addToHistory(newValue);
      this.$emit('update:modelValue', newValue);

      this.$nextTick(() => {
        textarea.focus();
        textarea.scrollTop = currentScrollPos;
      });
    }
  }
};
</script>

<style scoped>
.text-constructor {
  border: 1px solid #e6e6e6;
  border-radius: 12px;
  margin-bottom: 10px;
  background: #fff;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.text-constructor:focus-within {
  border-color: #4F5BDF;
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.12);
}

.text-constructor.is-disabled {
  opacity: 0.65;
  background: #f7f7f9;
}

.editor-toolbar {
  display: flex;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid #e6e6e6;
  align-items: center;
  flex-wrap: nowrap;
  background: #fafafa;
  border-radius: 12px 12px 0 0;
}

.toolbar-group {
  display: flex;
  gap: 4px;
  align-items: center;
  padding-right: 8px;
  border-right: 1px solid #e6e6e6;
}

.toolbar-group:last-child {
  border-right: none;
  padding-right: 0;
}

.toolbar-group.lists-group {
  width: 130px;
  justify-content: space-between;
}

.toolbar-btn {
  padding: 4px 6px;
  border: 1px solid #ddd;
  background: white;
  border-radius: 8px;
  cursor: pointer;
  font-size: 12px;
  font-family: inherit;
  color: #1a1a1a;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease, transform 0.05s ease;
  min-width: 32px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
}

.toolbar-btn:hover:not(:disabled) {
  background: #eef0ff;
  border-color: #4F5BDF;
  color: #4F5BDF;
}

.toolbar-btn:active:not(:disabled) {
  transform: translateY(1px);
}

.toolbar-btn:focus-visible {
  outline: none;
  border-color: #4F5BDF;
  box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.25);
}

.toolbar-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.image-btn {
  font-weight: 600;
  letter-spacing: 0.5px;
}

.image-input {
  display: none;
}

.undo-btn {
  background: #f8f9fa;
  border-color: #e9ecef;
}

.undo-btn:hover:not(:disabled) {
  background: #eef0ff;
  border-color: #4F5BDF;
}

.preview-btn {
  background: #f8f9fa;
  border-color: #e9ecef;
}

.preview-btn:hover:not(:disabled) {
  background: #eef0ff;
  border-color: #4F5BDF;
  color: #4F5BDF;
}

/* Tooltip pattern, общий для кнопок и селектов */
.toolbar-btn:hover:not(:disabled)::after,
.custom-select:hover .select-header::after {
  content: attr(data-tooltip);
  position: absolute;
  bottom: -32px;
  left: 50%;
  transform: translateX(-50%);
  background: #1a1a1a;
  color: white;
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 11px;
  white-space: nowrap;
  z-index: 1000;
  font-weight: normal;
  font-family: inherit;
  pointer-events: none;
  opacity: 0;
  animation: tooltipFadeIn 0.15s ease forwards;
}

@keyframes tooltipFadeIn {
  to { opacity: 1; }
}

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

.custom-select:hover .select-header[data-tooltip=""]::after {
  content: none;
}

.custom-select.fixed-width-select {
  width: 78px;
}

.color-btn {
  font-weight: bold;
}

.black-text { color: #000 !important; }
.red-text { color: #FF0000 !important; }
.green-text { color: #079D1D !important; }
.blue-text { color: #4F5BDF !important; }

.select-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px;
  border: 1px solid #ddd;
  border-radius: 8px;
  background: white;
  cursor: pointer;
  font-size: 12px;
  font-family: inherit;
  height: 28px;
  transition: border-color 0.15s ease, background-color 0.15s ease;
  position: relative;
  width: 100%;
}

.select-header:hover {
  border-color: #4F5BDF;
  background: #f8f9ff;
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
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.08);
  z-index: 1000;
  margin-top: 4px;
  max-height: 200px;
  overflow-y: auto;
  padding: 4px;
}

.select-option {
  padding: 6px 10px;
  font-size: 12px;
  cursor: pointer;
  transition: background-color 0.15s ease;
  color: #000;
  border-radius: 6px;
}

.select-option:hover {
  background: #eef0ff;
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
  padding: 14px;
  border: none;
  font-family: inherit;
  font-size: 14px;
  line-height: 1.5;
  border-radius: 0;
  min-height: 150px;
  max-height: 350px;
  resize: none;
  display: block;
  overflow-y: auto;
  background: transparent;
  color: #1a1a1a;
}

.constructor-textarea:focus {
  outline: none;
}

.constructor-textarea:disabled {
  cursor: not-allowed;
  background: transparent;
}

.constructor-textarea::placeholder {
  color: #9aa0a6;
  font-style: italic;
}

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
  transition: background-color 0.15s ease, opacity 0.15s ease;
}

.resize-handle:hover {
  background: #4F5BDF;
  opacity: 0.3;
}

.resize-handle:active {
  background: #4F5BDF;
  opacity: 0.5;
}

.constructor-error {
  margin: 10px 12px 0;
  padding: 8px 12px;
  border-radius: 8px;
  background: #fff1f1;
  border: 1px solid #f5c2c2;
  color: #b3261e;
  font-size: 12px;
}

.editor-preview {
  border-top: 1px solid #e6e6e6;
  background: #fafafa;
  border-radius: 0 0 12px 12px;
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
  font-weight: 600;
}

.preview-expand-btn {
  background: transparent;
  border: 1px solid transparent;
  color: #4F5BDF;
  font-family: inherit;
  font-size: 12px;
  cursor: pointer;
  padding: 4px 10px;
  border-radius: 8px;
  transition: background-color 0.15s ease, border-color 0.15s ease;
}

.preview-expand-btn:hover {
  background: #eef0ff;
  border-color: #4F5BDF;
}

.preview-content-container {
  position: relative;
  padding: 0 12px 12px 12px;
}

.preview-content {
  background: white;
  padding: 12px;
  border-radius: 10px;
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

.preview-modal-content {
  font-family: inherit;
  font-size: 14px;
  line-height: 1.5;
  color: #1a1a1a;
  max-height: 70vh;
  overflow-y: auto;
}

/* Изображения внутри редактора и preview */
.preview-content :deep(img),
.preview-modal-content :deep(img) {
  max-width: 100%;
  height: auto;
  border-radius: 10px;
  margin: 6px 0;
  display: inline-block;
}

.preview-content :deep(.constructor-image),
.preview-modal-content :deep(.constructor-image) {
  border: 1px solid #e6e6e6;
}

/* Размеры и жирность шрифта */
.font-size-10 { font-size: 10px !important; }
.font-size-12 { font-size: 12px !important; }
.font-size-14 { font-size: 14px !important; }
.font-size-16 { font-size: 16px !important; }
.font-size-18 { font-size: 18px !important; }
.font-size-20 { font-size: 20px !important; }

.font-weight-300 { font-weight: 300 !important; }
.font-weight-400 { font-weight: 400 !important; }
.font-weight-500 { font-weight: 500 !important; }
.font-weight-600 { font-weight: 600 !important; }
.font-weight-900 { font-weight: 900 !important; }

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

.preview-content :deep(*),
.preview-modal-content :deep(*) {
  font-size: 14px;
  max-font-size: 20px !important;
  min-font-size: 10px !important;
}

.preview-content :deep(.font-size-10),
.preview-modal-content :deep(.font-size-10) { font-size: 10px !important; }
.preview-content :deep(.font-size-12),
.preview-modal-content :deep(.font-size-12) { font-size: 12px !important; }
.preview-content :deep(.font-size-14),
.preview-modal-content :deep(.font-size-14) { font-size: 14px !important; }
.preview-content :deep(.font-size-16),
.preview-modal-content :deep(.font-size-16) { font-size: 16px !important; }
.preview-content :deep(.font-size-18),
.preview-modal-content :deep(.font-size-18) { font-size: 18px !important; }
.preview-content :deep(.font-size-20),
.preview-modal-content :deep(.font-size-20) { font-size: 20px !important; }

.preview-content :deep(.font-weight-300),
.preview-modal-content :deep(.font-weight-300) { font-weight: 300 !important; }
.preview-content :deep(.font-weight-400),
.preview-modal-content :deep(.font-weight-400) { font-weight: 400 !important; }
.preview-content :deep(.font-weight-500),
.preview-modal-content :deep(.font-weight-500) { font-weight: 500 !important; }
.preview-content :deep(.font-weight-600),
.preview-modal-content :deep(.font-weight-600) { font-weight: 600 !important; }
.preview-content :deep(.font-weight-900),
.preview-modal-content :deep(.font-weight-900) { font-weight: 900 !important; }

.preview-content :deep(.black-text),
.preview-modal-content :deep(.black-text) { color: #000 !important; }
.preview-content :deep(.red-text),
.preview-modal-content :deep(.red-text) { color: #FF0000 !important; }
.preview-content :deep(.green-text),
.preview-modal-content :deep(.green-text) { color: #079D1D !important; }
.preview-content :deep(.blue-text),
.preview-modal-content :deep(.blue-text) { color: #4F5BDF !important; }

.preview-content :deep(.heading-h1),
.preview-content :deep(.heading-h1 *),
.preview-modal-content :deep(.heading-h1),
.preview-modal-content :deep(.heading-h1 *) {
  font-size: 24px !important;
  font-weight: 700 !important;
  color: #000 !important;
  margin: 10px 0 8px 0 !important;
  line-height: 1.2 !important;
}

.preview-content :deep(.heading-h2),
.preview-content :deep(.heading-h2 *),
.preview-modal-content :deep(.heading-h2),
.preview-modal-content :deep(.heading-h2 *) {
  font-size: 20px !important;
  font-weight: 600 !important;
  color: #000 !important;
  margin: 8px 0 6px 0 !important;
  line-height: 1.3 !important;
}

.heading-h1 strong,
.heading-h2 strong,
.preview-content :deep(.heading-h1 strong),
.preview-content :deep(.heading-h2 strong),
.preview-modal-content :deep(.heading-h1 strong),
.preview-modal-content :deep(.heading-h2 strong) {
  font-size: inherit !important;
  font-weight: inherit !important;
  color: inherit !important;
}

.preview-content :deep(ul),
.preview-content :deep(ol),
.preview-modal-content :deep(ul),
.preview-modal-content :deep(ol) {
  padding-left: 24px !important;
}

.preview-content :deep(li),
.preview-modal-content :deep(li) {
  line-height: 1.4 !important;
}

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

.image-blocks {
  margin-top: 12px;
  padding: 12px;
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-bg-secondary);
}

.image-blocks__header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}

.image-blocks__title {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text);
}

.image-blocks__hint {
  font-size: 11px;
  color: var(--color-text-muted);
}

.image-blocks__list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.image-block {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: #fff;
  cursor: grab;
  transition: border-color 0.15s ease, box-shadow 0.15s ease, opacity 0.15s ease;
}

.image-block:hover {
  border-color: var(--color-primary);
}

.image-block--dragging {
  opacity: 0.4;
  cursor: grabbing;
}

.image-block--target {
  border-color: var(--color-primary);
  box-shadow: var(--shadow-focus);
}

.image-block__handle {
  color: var(--color-text-muted);
  flex-shrink: 0;
  display: flex;
}

.image-block__preview {
  width: 40px;
  height: 40px;
  object-fit: cover;
  border-radius: var(--radius-sm);
  flex-shrink: 0;
  border: 1px solid var(--color-border);
}

.image-block__index {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-primary);
  min-width: 24px;
}

.image-block__actions {
  margin-left: auto;
  display: flex;
  gap: 4px;
}

.image-block__btn {
  width: 28px;
  height: 28px;
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.image-block__btn:hover:not(:disabled) {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: #fff;
}

.image-block__btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.image-block__btn--danger:hover:not(:disabled) {
  background: var(--color-danger);
  border-color: var(--color-danger);
}
</style>
