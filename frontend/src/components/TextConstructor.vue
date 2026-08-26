<template>
  <div
    class="text-constructor"
    :class="{ 'is-disabled': disabled }"
  >
    <div
      class="editor-toolbar"
      @mousedown.prevent
    >
      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-btn"
          :class="{ active: editor && editor.isActive('bold') }"
          data-tooltip="Жирный"
          :disabled="disabled"
          @click="runCommand((c) => c.toggleBold())"
        >
          <strong>B</strong>
        </button>
        <button
          type="button"
          class="toolbar-btn"
          :class="{ active: editor && editor.isActive('italic') }"
          data-tooltip="Курсив"
          :disabled="disabled"
          @click="runCommand((c) => c.toggleItalic())"
        >
          <em>I</em>
        </button>
        <button
          type="button"
          class="toolbar-btn"
          :class="{ active: editor && editor.isActive('underline') }"
          data-tooltip="Подчеркивание"
          :disabled="disabled"
          @click="runCommand((c) => c.toggleUnderline())"
        >
          <u>U</u>
        </button>
      </div>

      <div class="toolbar-group lists-group">
        <button
          type="button"
          class="toolbar-btn"
          :class="{ active: editor && editor.isActive('bulletList') }"
          data-tooltip="Маркированный список"
          :disabled="disabled"
          @click="runCommand((c) => c.toggleBulletList())"
        >
          • L
        </button>
        <button
          type="button"
          class="toolbar-btn"
          :class="{ active: editor && editor.isActive('orderedList') }"
          data-tooltip="Нумерованный список"
          :disabled="disabled"
          @click="runCommand((c) => c.toggleOrderedList())"
        >
          1. L
        </button>
      </div>

      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-btn"
          :class="{ active: editor && editor.isActive('heading', { level: 1 }) }"
          data-tooltip="Заголовок h1"
          :disabled="disabled"
          @click="runCommand((c) => c.toggleHeading({ level: 1 }))"
        >
          h1
        </button>
        <button
          type="button"
          class="toolbar-btn"
          :class="{ active: editor && editor.isActive('heading', { level: 2 }) }"
          data-tooltip="Заголовок h2"
          :disabled="disabled"
          @click="runCommand((c) => c.toggleHeading({ level: 2 }))"
        >
          h2
        </button>
      </div>

      <div class="toolbar-group align-group">
        <button
          type="button"
          class="toolbar-btn align-btn"
          :class="{ active: isAlignActive('left') }"
          data-tooltip="По левому краю"
          :disabled="disabled"
          @click="applyAlign('left')"
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 16 16"
            aria-hidden="true"
          >
            <rect
              x="2"
              y="3"
              width="12"
              height="1.6"
              rx="0.8"
              fill="currentColor"
            />
            <rect
              x="2"
              y="7.2"
              width="8"
              height="1.6"
              rx="0.8"
              fill="currentColor"
            />
            <rect
              x="2"
              y="11.4"
              width="11"
              height="1.6"
              rx="0.8"
              fill="currentColor"
            />
          </svg>
        </button>
        <button
          type="button"
          class="toolbar-btn align-btn"
          :class="{ active: isAlignActive('center') }"
          data-tooltip="По центру"
          :disabled="disabled"
          @click="applyAlign('center')"
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 16 16"
            aria-hidden="true"
          >
            <rect
              x="2"
              y="3"
              width="12"
              height="1.6"
              rx="0.8"
              fill="currentColor"
            />
            <rect
              x="4"
              y="7.2"
              width="8"
              height="1.6"
              rx="0.8"
              fill="currentColor"
            />
            <rect
              x="2.5"
              y="11.4"
              width="11"
              height="1.6"
              rx="0.8"
              fill="currentColor"
            />
          </svg>
        </button>
        <button
          type="button"
          class="toolbar-btn align-btn"
          :class="{ active: isAlignActive('right') }"
          data-tooltip="По правому краю"
          :disabled="disabled"
          @click="applyAlign('right')"
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 16 16"
            aria-hidden="true"
          >
            <rect
              x="2"
              y="3"
              width="12"
              height="1.6"
              rx="0.8"
              fill="currentColor"
            />
            <rect
              x="6"
              y="7.2"
              width="8"
              height="1.6"
              rx="0.8"
              fill="currentColor"
            />
            <rect
              x="3"
              y="11.4"
              width="11"
              height="1.6"
              rx="0.8"
              fill="currentColor"
            />
          </svg>
        </button>
      </div>

      <div class="toolbar-group">
        <div
          class="custom-select"
        >
          <div
            class="select-header"
            :data-tooltip="fontSizeDropdownOpen ? '' : 'Размер шрифта'"
            @click="toggleFontSizeDropdown"
          >
            <span class="select-value">{{ selectedFontSize }}</span>
            <svg
              class="select-arrow"
              :class="{ rotated: fontSizeDropdownOpen }"
              viewBox="0 0 10 6"
              fill="none"
              xmlns="http://www.w3.org/2000/svg"
              aria-hidden="true"
            >
              <path
                d="M1 1L5 5L9 1"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </div>
          <transition name="select-fade">
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
          </transition>
        </div>
      </div>

      <div class="toolbar-group">
        <div
          class="custom-select fixed-width-select"
        >
          <div
            class="select-header"
            :data-tooltip="fontWeightDropdownOpen ? '' : 'Жирность шрифта'"
            @click="toggleFontWeightDropdown"
          >
            <span class="select-value">{{ selectedFontWeight.label }}</span>
            <svg
              class="select-arrow"
              :class="{ rotated: fontWeightDropdownOpen }"
              viewBox="0 0 10 6"
              fill="none"
              xmlns="http://www.w3.org/2000/svg"
              aria-hidden="true"
            >
              <path
                d="M1 1L5 5L9 1"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </div>
          <transition name="select-fade">
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
          </transition>
        </div>
      </div>

      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-btn color-btn black-text"
          data-tooltip="Черный"
          :disabled="disabled"
          @click="applyColor('black-text')"
        >
          A
        </button>
        <button
          type="button"
          class="toolbar-btn color-btn red-text"
          data-tooltip="Красный"
          :disabled="disabled"
          @click="applyColor('red-text')"
        >
          A
        </button>
        <button
          type="button"
          class="toolbar-btn color-btn green-text"
          data-tooltip="Зеленый"
          :disabled="disabled"
          @click="applyColor('green-text')"
        >
          A
        </button>
        <button
          type="button"
          class="toolbar-btn color-btn blue-text"
          data-tooltip="Синий"
          :disabled="disabled"
          @click="applyColor('blue-text')"
        >
          A
        </button>
      </div>

      <div class="toolbar-group">
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
          @click="runCommand((c) => c.setHardBreak())"
        >
          ↵
        </button>
      </div>

      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-btn undo-btn"
          data-tooltip="Назад (Ctrl+Z)"
          :disabled="disabled || !canUndo"
          @click="runCommand((c) => c.undo())"
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
      <div
        v-if="$slots['header-actions']"
        class="tc-header-actions"
      >
        <slot name="header-actions" />
      </div>
    </div>

    <!-- Полоса вложений живёт между панелью и текстом, как в почтовом клиенте:
         вложения относятся к письму целиком, а не к месту курсора. Пустой слот
         ничего не рисует, поэтому у прочих мест применения вид не меняется. -->
    <div
      v-if="$slots.attachments"
      class="tc-attachments"
    >
      <slot name="attachments" />
    </div>

    <EditorContent
      :editor="editor"
      class="editor-content"
      :style="{ minHeight: minContentHeight }"
    />

    <div
      v-if="imageError"
      class="constructor-error"
      role="alert"
    >
      {{ imageError }}
    </div>

    <BaseModal
      :show="previewModalOpen"
      title="Предпросмотр контента"
      width="780px"
      @close="previewModalOpen = false"
    >
      <div
        class="preview-modal-content text-constructor-content"
        v-html="sanitizedContent"
      />
    </BaseModal>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';
import { useEditor, EditorContent } from '@tiptap/vue-3';
import StarterKit from '@tiptap/starter-kit';
import Underline from '@tiptap/extension-underline';
import Placeholder from '@tiptap/extension-placeholder';
import { sanitizeHtml } from '@/utils/sanitize';
import BaseModal from '@/components/ui/BaseModal.vue';
import {
  ColorClass,
  FontSizeClass,
  FontWeightClass,
  ClassHeading,
  ConstructorImage,
  TextAlignClass,
} from './text-constructor/extensions';

const ALLOWED_IMAGE_TYPES = [
  'image/png',
  'image/jpeg',
  'image/gif',
  'image/webp',
  'image/svg+xml',
];

const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: 'Введите текст...' },
  rows: { type: Number, default: 4 },
  disabled: { type: Boolean, default: false },
  maxImageBytes: { type: Number, default: 5 * 1024 * 1024 },
});

const emit = defineEmits(['update:modelValue']);

const fontSizes = ['10px', '12px', '14px', '16px', '18px', '20px'];
const fontWeights = [
  { label: 'Black', value: '900' },
  { label: 'Bold', value: '600' },
  { label: 'Medium', value: '500' },
  { label: 'Regular', value: '400' },
  { label: 'Light', value: '300' },
];

const selectedFontSize = ref('14px');
const selectedFontWeight = ref({ label: 'Regular', value: '400' });
const fontSizeDropdownOpen = ref(false);
const fontWeightDropdownOpen = ref(false);
const previewModalOpen = ref(false);
const imageError = ref('');
const imageInput = ref(null);
const canUndo = ref(false);
let imageErrorTimer = null;
let onDocClick = null;

const minContentHeight = computed(() => `${Math.max(1, props.rows) * 26}px`);
const sanitizedContent = computed(() => sanitizeHtml(props.modelValue));

const editor = useEditor({
  content: props.modelValue || '',
  editable: !props.disabled,
  extensions: [
    StarterKit.configure({ heading: false }),
    Underline,
    ClassHeading,
    ColorClass,
    FontSizeClass,
    FontWeightClass,
    ConstructorImage,
    TextAlignClass,
    Placeholder.configure({ placeholder: () => props.placeholder }),
  ],
  onUpdate: ({ editor: instance }) => {
    canUndo.value = instance.can().undo();
    const html = instance.isEmpty ? '' : instance.getHTML();
    if (html !== props.modelValue) {
      emit('update:modelValue', html);
    }
  },
});

watch(
  () => props.modelValue,
  (value) => {
    const instance = editor.value;
    if (!instance) return;
    const current = instance.isEmpty ? '' : instance.getHTML();
    if ((value || '') !== current) {
      instance.commands.setContent(value || '', false);
    }
  }
);

watch(
  () => props.disabled,
  (value) => {
    editor.value?.setEditable(!value);
  }
);

onMounted(() => {
  onDocClick = (event) => {
    const root = editor.value?.options?.element?.closest('.text-constructor');
    if (root && !root.contains(event.target)) {
      fontSizeDropdownOpen.value = false;
      fontWeightDropdownOpen.value = false;
    }
  };
  document.addEventListener('click', onDocClick);
});

onBeforeUnmount(() => {
  if (onDocClick) document.removeEventListener('click', onDocClick);
  if (imageErrorTimer) clearTimeout(imageErrorTimer);
  editor.value?.destroy();
});

/**
 * Запускает chain-команду на редакторе. Фокус дёргаем только когда редактор уже
 * активен: без этого тап по кнопке тулбара на телефоне открывал клавиатуру, хотя
 * пользователь ещё не собирался печатать. При наборе фокус наоборот сохраняем
 * (вместе с mousedown.prevent на панели), чтобы клавиатура не схлопывалась.
 * @param {(chain: import('@tiptap/core').ChainedCommands) => import('@tiptap/core').ChainedCommands} build
 */
function runCommand(build) {
  if (props.disabled || !editor.value) return;
  const chain = editor.value.isFocused ? editor.value.chain().focus() : editor.value.chain();
  build(chain).run();
}

function applyColor(colorClass) {
  runCommand((c) => c.setColorClass(colorClass));
}

/**
 * Выравнивание: если выделена картинка - выравниваем её (float left/right или center),
 * иначе выравниваем текущий абзац/заголовок. Кнопки тулбара общие для текста и картинки.
 */
function applyAlign(alignment) {
  if (props.disabled || !editor.value) return;
  if (editor.value.isActive('image')) {
    runCommand((c) => c.setImageAlign(alignment));
  } else {
    runCommand((c) => c.setTextAlignClass(alignment));
  }
}

function isAlignActive(alignment) {
  if (!editor.value) return false;
  return (
    editor.value.isActive('image', { align: alignment }) ||
    editor.value.isActive({ textAlign: alignment })
  );
}

function toggleFontSizeDropdown() {
  if (props.disabled) return;
  fontSizeDropdownOpen.value = !fontSizeDropdownOpen.value;
  fontWeightDropdownOpen.value = false;
}

function selectFontSize(size) {
  selectedFontSize.value = size;
  fontSizeDropdownOpen.value = false;
  runCommand((c) => c.setFontSizeClass(`font-size-${size.replace('px', '')}`));
}

function toggleFontWeightDropdown() {
  if (props.disabled) return;
  fontWeightDropdownOpen.value = !fontWeightDropdownOpen.value;
  fontSizeDropdownOpen.value = false;
}

function selectFontWeight(weight) {
  selectedFontWeight.value = weight;
  fontWeightDropdownOpen.value = false;
  runCommand((c) => c.setFontWeightClass(`font-weight-${weight.value}`));
}

function openPreviewModal() {
  if (!props.modelValue) return;
  previewModalOpen.value = true;
}

function triggerImagePicker() {
  if (props.disabled) return;
  clearImageError();
  if (imageInput.value) {
    imageInput.value.value = '';
    imageInput.value.click();
  }
}

function readFileAsDataUrl(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error || new Error('FileReader error'));
    reader.readAsDataURL(file);
  });
}

async function handleImageSelected(event) {
  const file = event.target?.files?.[0];
  if (!file) return;

  if (!ALLOWED_IMAGE_TYPES.includes(file.type)) {
    setImageError('Неподдерживаемый формат. Разрешены: PNG, JPEG, GIF, WEBP, SVG.');
    return;
  }
  if (file.size > props.maxImageBytes) {
    const maxMb = Math.round(props.maxImageBytes / (1024 * 1024));
    setImageError(`Файл слишком большой. Максимальный размер: ${maxMb} МБ.`);
    return;
  }

  try {
    const dataUrl = await readFileAsDataUrl(file);
    const safeAlt = (file.name || 'image').replace(/[<>"'&]/g, '').slice(0, 100);
    runCommand((c) => c.setImage({ src: dataUrl, alt: safeAlt }));
  } catch (err) {
    setImageError('Не удалось прочитать файл. Попробуйте ещё раз.');
    console.error('TextConstructor image read error:', err);
  } finally {
    if (imageInput.value) imageInput.value.value = '';
  }
}

function setImageError(message) {
  imageError.value = message;
  if (imageErrorTimer) clearTimeout(imageErrorTimer);
  imageErrorTimer = setTimeout(() => {
    imageError.value = '';
  }, 4500);
}

function clearImageError() {
  imageError.value = '';
  if (imageErrorTimer) {
    clearTimeout(imageErrorTimer);
    imageErrorTimer = null;
  }
}

defineExpose({ editor });
</script>

<style scoped>
.text-constructor {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  margin-bottom: 10px;
  background: var(--surface);
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.text-constructor:focus-within {
  border-color: var(--accent);
  box-shadow: var(--shadow-focus);
}

.text-constructor.is-disabled {
  opacity: 0.7;
}

.editor-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--color-border);
}

/* Слот действий справа от инструментов (напр. согласие + отправка в форме заявки). */
.tc-header-actions {
  margin-left: auto;
  flex-shrink: 0;
  display: flex;
  align-items: center;
}

.toolbar-group {
  display: flex;
  align-items: center;
  gap: 4px;
}

.toolbar-btn {
  position: relative;
  min-width: 32px;
  height: 32px;
  padding: 0 8px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--surface);
  color: var(--text);
  font-size: 14px;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease, opacity 0.15s ease;
}

/* Только на устройствах с настоящим hover: на таче :hover залипает после тапа,
   и кнопка выглядит зажатой, а подсказка повисает. */
@media (hover: hover) {
  .toolbar-btn:hover:not(:disabled) {
    background: var(--accent-tint);
    border-color: var(--accent);
  }
}

.toolbar-btn.active {
  background: var(--color-primary);
  border-color: var(--accent);
  color: var(--accent-contrast);
}

.toolbar-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.align-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.align-btn svg {
  display: block;
}

.color-btn.black-text { color: #000; }
.color-btn.red-text { color: #ff0000; }
.color-btn.green-text { color: #079d1d; }
.color-btn.blue-text { color: #4f5bdf; }

.color-btn.active {
  color: inherit;
}

@media (hover: hover) {
  .color-btn:hover:not(:disabled) {
    color: inherit;
  }

  .toolbar-btn[data-tooltip]:hover::after {
    content: attr(data-tooltip);
    position: absolute;
    bottom: calc(100% + 6px);
    left: 50%;
    transform: translateX(-50%);
    padding: 4px 8px;
    background: var(--hint-bg);
    color: var(--hint-text);
    font-size: 11px;
    white-space: nowrap;
    border-radius: 6px;
    pointer-events: none;
    z-index: 10;
  }
}

.image-input {
  display: none;
}

.custom-select {
  position: relative;
}

.custom-select.fixed-width-select {
  width: 92px;
}

.select-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  min-width: 64px;
  height: 32px;
  padding: 0 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--surface);
  cursor: pointer;
  font-size: 13px;
  transition: border-color 0.15s ease;
}

@media (hover: hover) {
  .select-header:hover {
    border-color: var(--accent);
  }
}

.select-arrow {
  width: 10px;
  height: 10px;
  flex-shrink: 0;
  color: var(--text-muted);
  transition: transform 0.2s ease;
}

.select-arrow.rotated {
  transform: rotate(180deg);
}

.select-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  z-index: 20;
  max-height: 220px;
  overflow-y: auto;
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  box-shadow: 0 6px 18px rgba(15, 23, 42, 0.12);
}

.select-fade-enter-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.select-fade-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.select-fade-enter-from,
.select-fade-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

.select-option {
  padding: 6px 10px;
  font-size: 13px;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.select-option.active {
  background: var(--accent-tint);
  color: var(--accent-text);
}

@media (hover: hover) {
  .select-option:hover {
    background: var(--accent-tint);
    color: var(--accent-text);
  }
}

.tc-attachments {
    padding: 6px 8px;
    border-bottom: 1px solid var(--border);
}

.editor-content {
  padding: 12px 14px;
}

.editor-content :deep(.ProseMirror) {
  outline: none;
  min-height: inherit;
  line-height: 150%;
  word-break: break-word;
  /* Дефолтный размер вводимого текста (без выбранного font-size-класса). */
  font-size: 14px;
}

/* Чтобы плавающие (float) картинки не вылезали за пределы редактора. */
.editor-content :deep(.ProseMirror)::after {
  content: '';
  display: block;
  clear: both;
}

.editor-content :deep(.ProseMirror p.is-editor-empty:first-child::before) {
  content: attr(data-placeholder);
  float: left;
  height: 0;
  color: var(--text-muted);
  pointer-events: none;
}

.constructor-error {
  padding: 6px 14px 10px;
  color: var(--danger-text);
  font-size: 12px;
}

.preview-modal-content {
  padding: 24px 28px;
  max-height: calc(var(--app-vh, 1vh) * 70);
  overflow-y: auto;
  font-size: 15px;
  line-height: 1.6;
  color: var(--text);
  word-break: break-word;
}

/* Рендер форматирования внутри редактора и в модалке предпросмотра (round-trip с потребителями) */
.editor-content :deep(strong),
.preview-modal-content :deep(strong) { font-weight: 600; }
.editor-content :deep(em),
.preview-modal-content :deep(em) { font-style: italic; }
.editor-content :deep(u),
.preview-modal-content :deep(u) { text-decoration: underline; }
.editor-content :deep(ul),
.editor-content :deep(ol),
.preview-modal-content :deep(ul),
.preview-modal-content :deep(ol) { padding-left: 20px; }
.editor-content :deep(h1),
.preview-modal-content :deep(h1) { font-size: 24px; font-weight: 700; margin: 0 0 8px; line-height: 1.2; }
.editor-content :deep(h2),
.preview-modal-content :deep(h2) { font-size: 20px; font-weight: 600; margin: 8px 0 6px; line-height: 1.3; }

.editor-content :deep(.black-text),
.preview-modal-content :deep(.black-text) { color: #000; }
.editor-content :deep(.red-text),
.preview-modal-content :deep(.red-text) { color: #ff0000; }
.editor-content :deep(.green-text),
.preview-modal-content :deep(.green-text) { color: #079d1d; }
.editor-content :deep(.blue-text),
.preview-modal-content :deep(.blue-text) { color: var(--accent-text); }

.editor-content :deep(.font-size-10),
.preview-modal-content :deep(.font-size-10) { font-size: 10px; }
.editor-content :deep(.font-size-12),
.preview-modal-content :deep(.font-size-12) { font-size: 12px; }
.editor-content :deep(.font-size-14),
.preview-modal-content :deep(.font-size-14) { font-size: 14px; }
.editor-content :deep(.font-size-16),
.preview-modal-content :deep(.font-size-16) { font-size: 16px; }
.editor-content :deep(.font-size-18),
.preview-modal-content :deep(.font-size-18) { font-size: 18px; }
.editor-content :deep(.font-size-20),
.preview-modal-content :deep(.font-size-20) { font-size: 20px; }

.editor-content :deep(.font-weight-300),
.preview-modal-content :deep(.font-weight-300) { font-weight: 300; }
.editor-content :deep(.font-weight-400),
.preview-modal-content :deep(.font-weight-400) { font-weight: 400; }
.editor-content :deep(.font-weight-500),
.preview-modal-content :deep(.font-weight-500) { font-weight: 500; }
.editor-content :deep(.font-weight-600),
.preview-modal-content :deep(.font-weight-600) { font-weight: 600; }
.editor-content :deep(.font-weight-900),
.preview-modal-content :deep(.font-weight-900) { font-weight: 900; }

.editor-content :deep(.text-align-left),
.preview-modal-content :deep(.text-align-left) { text-align: left; }
.editor-content :deep(.text-align-center),
.preview-modal-content :deep(.text-align-center) { text-align: center; }
.editor-content :deep(.text-align-right),
.preview-modal-content :deep(.text-align-right) { text-align: right; }

.editor-content :deep(img),
.preview-modal-content :deep(img) {
  max-width: 100%;
  border-radius: 8px;
}

.editor-content :deep(img:not([height])),
.preview-modal-content :deep(img:not([height])) {
  height: auto;
}

/* Выравнивание картинок (float-обтекание) в предпросмотре */
.preview-modal-content :deep(.constructor-image.img-align-left) {
  float: left;
  margin: 0 14px 10px 0;
}

.preview-modal-content :deep(.constructor-image.img-align-right) {
  float: right;
  margin: 0 0 10px 14px;
}

.preview-modal-content :deep(.constructor-image.img-align-center) {
  display: block;
  float: none;
  margin: 10px auto;
}

.preview-modal-content::after {
  content: '';
  display: block;
  clear: both;
}
</style>
