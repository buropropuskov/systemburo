<template>
  <div
    class="tpf"
    :class="{ 'tpf--disabled': disabled }"
  >
    <div class="tpf__head">
      <label class="field-label">{{ label }}</label>
      <button
        type="button"
        class="tpf__reset"
        :disabled="disabled"
        data-testid="tpf-reset"
        @click="resetToDefault"
      >
        Стандартный
      </button>
    </div>

    <input
      ref="inputEl"
      type="text"
      class="lk-input tpf__input"
      :value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      data-testid="tpf-input"
      @input="onInput"
    >

    <p
      v-if="parsedSegments.length"
      class="tpf__breakdown"
      data-testid="tpf-breakdown"
    >
      <span
        v-for="(seg, i) in parsedSegments"
        :key="i"
        :class="['tpf__seg', `tpf__seg--${seg.kind}`]"
        :title="seg.reason"
      >{{ seg.text }}</span>
    </p>

    <p
      v-if="previewText"
      class="tpf__preview"
      data-testid="tpf-preview"
    >
      <span
        class="tpf__preview-arrow"
        aria-hidden="true"
      >→</span>{{ previewText }}
    </p>

    <div
      class="tpf__palette"
      role="listbox"
      :aria-label="`Плейсхолдеры: ${label}`"
    >
      <div
        v-for="group in groupedTokens"
        :key="group.name"
        class="tpf__group"
      >
        <span class="tpf__group-name">{{ group.name }}</span>
        <div class="tpf__chips">
          <button
            v-for="t in group.tokens"
            :key="t.key"
            type="button"
            class="tpf__chip"
            :disabled="disabled"
            :title="`${t.label}: ${t.example}`"
            data-testid="tpf-chip"
            @mousedown.prevent="insertToken(t)"
          >
            {{ t.label }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue';

/**
 * Мини-конструктор шаблона пути файлового архива (#1615, срез C3): поле ввода +
 * палитра токенов-плейсхолдеров из реестра `internal/blankpath/tokens.go`. Один
 * компонент переиспользуется и для шаблона папки заявки, и для шаблона имени файла -
 * различает их проп scope, который фильтрует палитру по dir_allowed/file_allowed.
 *
 * Разбор шаблона на подсвеченные сегменты идёт локально (простой regex-сплит по
 * "{...}"), а не через blankpath: полноценные правила схлопывания разделителей и
 * подгонки длины остаются серверными, здесь только визуальная подсветка токенов.
 * Токен без совпадения в реестре подсвечивается сразу (клиентская проверка по props.tokens);
 * несовпадение по контексту (например "{тип}" в шаблоне папки) прилетает из props.problems -
 * то же превью, которым родитель уже дебаунсит один запрос на оба поля.
 */

const props = defineProps({
  modelValue: { type: String, default: '' },
  // Полный реестр плейсхолдеров (GET /file-archive/tokens), без фильтрации по scope -
  // компонент сам решает, что показать в палитре.
  tokens: { type: Array, default: () => [] },
  // 'dir' - шаблон каталогов, 'file' - шаблон имени файла. Определяет, какие токены
  // допустимы в палитре (dir_allowed/file_allowed) - "{тип}" вложения не имеет смысла
  // в имени папки заявки, у которой вложений несколько разных типов.
  scope: { type: String, required: true, validator: v => v === 'dir' || v === 'file' },
  label: { type: String, required: true },
  placeholder: { type: String, default: '' },
  // Значение кнопки "Стандартный" - blankpath.DefaultDirTemplate/DefaultFileTemplate,
  // передаётся родителем константой (в API не выведены, значения по умолчанию сервер
  // и так подставляет сам, если настройка ни разу не сохранялась).
  defaultTemplate: { type: String, required: true },
  // Претензии к шаблону с бэка ({token, reason}) - неизвестный плейсхолдер или
  // плейсхолдер не в своём контексте, полученные из последнего ответа превью.
  problems: { type: Array, default: () => [] },
  // Пример разложения ЭТОГО поля (уровни каталога через "/" или имя файла) -
  // из того же ответа превью, что и problems.
  previewText: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
});

const emit = defineEmits(['update:modelValue']);

const TOKEN_RE = /\{[^{}]*\}|[^{}]+/g;

const groupedTokens = computed(() => {
  const scopeKey = props.scope === 'dir' ? 'dir_allowed' : 'file_allowed';
  const order = [];
  const byGroup = new Map();
  for (const t of props.tokens) {
    if (!t[scopeKey]) continue;
    if (!byGroup.has(t.group)) {
      byGroup.set(t.group, { name: t.group, tokens: [] });
      order.push(byGroup.get(t.group));
    }
    byGroup.get(t.group).tokens.push(t);
  }
  return order;
});

const parsedSegments = computed(() => {
  const text = props.modelValue || '';
  const segments = [];
  // matchAll клонирует regex под капотом и не трогает TOKEN_RE.lastIndex - в отличие
  // от ручного exec-цикла, это не побочный эффект на внешнем состоянии внутри computed.
  for (const m of text.matchAll(TOKEN_RE)) {
    const chunk = m[0];
    if (chunk.startsWith('{') && chunk.endsWith('}')) {
      const key = chunk.slice(1, -1);
      const known = props.tokens.some(t => t.key === key);
      const problem = props.problems.find(p => p.token === key);
      segments.push({
        text: chunk,
        kind: !known ? 'unknown' : (problem ? 'warn' : 'ok'),
        reason: problem ? problem.reason : (!known ? 'неизвестный плейсхолдер' : ''),
      });
    } else {
      segments.push({ text: chunk, kind: 'text', reason: '' });
    }
  }
  return segments;
});

const inputEl = ref(null);

function onInput(e) {
  emit('update:modelValue', e.target.value);
}

function resetToDefault() {
  if (props.disabled) return;
  emit('update:modelValue', props.defaultTemplate);
}

// @mousedown.prevent на чипе не даёт полю потерять фокус - selectionStart/End,
// прочитанные здесь, всё ещё указывают на курсор пользователя, а не на 0 (клик по
// кнопке иначе снял бы фокус ДО обработчика и обнулил бы выделение).
function insertToken(t) {
  if (props.disabled) return;
  const el = inputEl.value;
  const value = props.modelValue || '';
  const start = el ? (el.selectionStart ?? value.length) : value.length;
  const end = el ? (el.selectionEnd ?? value.length) : value.length;
  const piece = `{${t.key}}`;
  const next = value.slice(0, start) + piece + value.slice(end);
  emit('update:modelValue', next);
  const pos = start + piece.length;
  requestAnimationFrame(() => {
    if (!inputEl.value) return;
    inputEl.value.focus();
    inputEl.value.setSelectionRange(pos, pos);
  });
}
</script>

<style scoped>
.tpf {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tpf--disabled {
  opacity: 0.65;
}

.tpf__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.field-label {
  font-size: 0.85em;
  color: var(--text-muted);
  font-weight: 500;
}

.tpf__reset {
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text-muted);
  border-radius: 999px;
  padding: 3px 12px;
  font-size: 0.75em;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.tpf__reset:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent-text);
  background: var(--accent-tint);
}

.tpf__reset:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.tpf__input {
  width: 100%;
  font-family: 'Courier New', monospace;
}

.tpf__breakdown {
  margin: 0;
  font-size: 0.8em;
  line-height: 1.6;
  word-break: break-word;
}

.tpf__seg {
  font-family: 'Courier New', monospace;
}

.tpf__seg--text {
  color: var(--text-muted);
}

.tpf__seg--ok {
  color: var(--accent-text);
  background: var(--accent-tint);
  border-radius: 4px;
  padding: 1px 3px;
}

.tpf__seg--unknown,
.tpf__seg--warn {
  color: var(--danger-text);
  background: var(--danger-bg);
  border-radius: 4px;
  padding: 1px 3px;
  cursor: help;
}

.tpf__preview {
  margin: 0;
  font-size: 0.82em;
  color: var(--text-muted);
  word-break: break-all;
}

.tpf__preview-arrow {
  margin-right: 6px;
  color: var(--accent-text);
}

.tpf__palette {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.tpf__group {
  display: flex;
  align-items: baseline;
  gap: 8px;
  flex-wrap: wrap;
}

.tpf__group-name {
  flex-shrink: 0;
  font-size: 0.72em;
  font-weight: 600;
  color: var(--text-muted);
  min-width: 84px;
}

.tpf__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tpf__chip {
  border: 1px solid var(--border);
  background: var(--surface-2);
  color: var(--text);
  border-radius: 999px;
  padding: 3px 10px;
  font-size: 0.75em;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.tpf__chip:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent-text);
  background: var(--accent-tint);
}

.tpf__chip:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Карусель чипов + запрет автозума iOS на фокусе поля (#1097, mobile-adaptive-etalon). */
@media (max-width: 768px) {
  .tpf__input {
    font-size: 16px;
  }

  .tpf__group {
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: none;
    padding-bottom: 2px;
  }

  .tpf__group::-webkit-scrollbar {
    display: none;
  }

  .tpf__group-name {
    position: sticky;
    left: 0;
    background: var(--surface);
  }

  .tpf__chips {
    flex-wrap: nowrap;
  }

  .tpf__chip {
    flex-shrink: 0;
  }
}
</style>
