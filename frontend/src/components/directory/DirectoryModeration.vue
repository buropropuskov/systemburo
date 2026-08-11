<template>
  <div
    class="org-moderation"
    :class="`org-moderation--${variant}`"
    :data-testid="`org-moderation-${kind}`"
  >
    <!-- Статический якорь тура принимающего: сам корень несёт `org-moderation-${kind}`,
         и по вычисляемому testid шаг тура сослаться не может. Якорь живёт только в
         карточке заявки - в справочнике тот же шаг подсветил бы чужой экран. -->
    <div
      class="org-moderation__head"
      :data-testid="variant === 'notice' ? 'ob-org-moderation' : null"
    >
      <span
        v-if="variant === 'notice'"
        class="org-moderation__badge"
      >На проверке</span>
      <span class="org-moderation__text">
        {{ label }} <b>{{ entryName }}</b> {{ originHint }}
      </span>
    </div>

    <!-- Столкновение наименований: бэк вернул status=conflict с существующей записью.
         Показываем её отдельно - для принимающего это не сбой, а готовая цель привязки. -->
    <div
      v-if="conflict"
      class="org-moderation__conflict"
      :data-testid="`org-moderation-${kind}-conflict`"
    >
      <span>В справочнике уже есть <b>{{ conflict.name }}</b>.</span>
      <div class="org-moderation__actions">
        <button
          class="lk-button lk-button--primary lk-button--sm"
          :disabled="busy"
          :data-testid="`org-moderation-${kind}-conflict-merge`"
          @click="merge(conflict.id)"
        >
          Привязать к ней
        </button>
        <button
          class="lk-button lk-button--ghost lk-button--sm"
          :disabled="busy"
          @click="resetMode"
        >
          Отмена
        </button>
      </div>
    </div>

    <template v-else-if="mode === 'idle'">
      <div class="org-moderation__actions">
        <button
          class="lk-button lk-button--primary lk-button--sm"
          :disabled="busy"
          :data-testid="`org-moderation-${kind}-approve`"
          @click="approve"
        >
          Добавить в справочник
        </button>
        <button
          class="lk-button lk-button--secondary lk-button--sm"
          :disabled="busy"
          :data-testid="`org-moderation-${kind}-rename-open`"
          @click="openRename"
        >
          Исправить наименование
        </button>
        <button
          class="lk-button lk-button--secondary lk-button--sm"
          :disabled="busy"
          :data-testid="`org-moderation-${kind}-merge-open`"
          @click="openMerge"
        >
          Привязать к существующей
        </button>
      </div>
    </template>

    <div
      v-else-if="mode === 'rename'"
      class="org-moderation__form"
    >
      <input
        ref="renameInput"
        v-model="renameValue"
        class="lk-input"
        :placeholder="`Наименование${kind === 'company' ? ' компании' : ' организации'}`"
        maxlength="100"
        :data-testid="`org-moderation-${kind}-rename-input`"
        @keydown.enter.prevent="rename"
        @keydown.esc="resetMode"
      >
      <div class="org-moderation__actions">
        <button
          class="lk-button lk-button--primary lk-button--sm"
          :disabled="busy || !renameValue.trim()"
          :data-testid="`org-moderation-${kind}-rename-save`"
          @click="rename"
        >
          Сохранить
        </button>
        <button
          class="lk-button lk-button--ghost lk-button--sm"
          :disabled="busy"
          @click="resetMode"
        >
          Отмена
        </button>
      </div>
    </div>

    <div
      v-else-if="mode === 'merge'"
      class="org-moderation__form"
    >
      <input
        v-model="searchQuery"
        class="lk-input"
        placeholder="Поиск по справочнику"
        :data-testid="`org-moderation-${kind}-merge-search`"
        @keydown.esc="resetMode"
      >
      <p
        v-if="loadingList"
        class="org-moderation__hint"
      >
        Загружаем справочник...
      </p>
      <p
        v-else-if="!filteredTargets.length"
        class="org-moderation__hint"
      >
        Подходящих записей не нашлось.
      </p>
      <ul
        v-else
        class="org-moderation__list"
      >
        <li
          v-for="target in filteredTargets"
          :key="target.id"
        >
          <button
            class="org-moderation__target"
            :disabled="busy"
            :data-testid="`org-moderation-${kind}-target`"
            @click="merge(target.id)"
          >
            {{ target.name }}
          </button>
        </li>
      </ul>
      <div class="org-moderation__actions">
        <button
          class="lk-button lk-button--ghost lk-button--sm"
          :disabled="busy"
          @click="resetMode"
        >
          Отмена
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { useDeletionsStore } from '@/stores/deletions';
import {
    approveDirectoryEntry,
    renameDirectoryEntry,
    mergeDirectoryEntry,
    fetchApprovedDirectory
} from '@/api/directory';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';

/**
 * Разбор организации или компании «на проверке» (#1437, #1875).
 *
 * Подача с незнакомым наименованием заводит запись со статусом pending, чтобы у заявки
 * был живой organization_id. Разбирают её в двух местах: принимающий - прямо в детали
 * заявки, администратор справочника - в карточке записи. Действия одни и те же:
 * подтвердить, исправить наименование, привязать к существующей записи.
 *
 * Результат уходит наверх событием resolved - потребитель обязан показать новое
 * наименование и погасить плашку, не дожидаясь перезагрузки страницы. При привязке
 * разбираемая запись физически удаляется, и `id` в событии - уже цель привязки.
 */
export default {
    name: 'DirectoryModeration',
    props: {
        /** 'organization' либо 'company' - какой справочник разбираем. */
        kind: { type: String, required: true },
        entryId: { type: Number, required: true },
        entryName: { type: String, default: '' },
        /**
         * Оформление: 'notice' - самостоятельная плашка на карточке заявки,
         * 'panel' - вложение в секцию справочника (фон и рамку даёт секция).
         */
        variant: {
            type: String,
            default: 'notice',
            validator: (value) => ['notice', 'panel'].includes(value)
        }
    },
    emits: ['resolved'],
    setup() {
        return { deletionsStore: useDeletionsStore() };
    },
    data() {
        return {
            mode: 'idle',
            busy: false,
            renameValue: '',
            searchQuery: '',
            targets: [],
            loadingList: false,
            /** Запись, с которой столкнулось наименование (ответ status=conflict). */
            conflict: null
        };
    },
    computed: {
        label() {
            return this.kind === 'company' ? 'Компания' : 'Организация';
        },

        // В справочнике «этой заявкой» указывать не на что - экран о заявках не знает.
        originHint() {
            return this.variant === 'panel'
                ? 'заведена подачей заявки и ещё не разобрана.'
                : 'заведена этой заявкой и ещё не разобрана.';
        },

        filteredTargets() {
            const query = this.searchQuery.trim();
            if (!query) return this.targets;
            const variants = buildSearchVariants(query);
            return this.targets.filter((target) => matchesSearch(target.name, variants));
        }
    },
    methods: {
        resetMode() {
            this.mode = 'idle';
            this.conflict = null;
            this.searchQuery = '';
        },

        openRename() {
            this.conflict = null;
            this.mode = 'rename';
            this.renameValue = this.entryName;
            this.$nextTick(() => this.$refs.renameInput?.focus());
        },

        async openMerge() {
            this.conflict = null;
            this.mode = 'merge';
            this.searchQuery = '';
            await this.loadTargets();
        },

        async loadTargets() {
            this.loadingList = true;
            try {
                const items = await fetchApprovedDirectory(this.kind);
                // Сама разбираемая запись целью быть не может (бэк отвечает 400).
                this.targets = items.filter((item) => item.id !== this.entryId);
            } catch (error) {
                this.targets = [];
                this.notifyError(error, 'Не удалось загрузить справочник');
            } finally {
                this.loadingList = false;
            }
        },

        async approve() {
            await this.run(
                () => approveDirectoryEntry(this.kind, this.entryId),
                (result) => this.handleModerationResult(result, `${this.label} добавлена в справочник`),
                'Не удалось подтвердить запись'
            );
        },

        async rename() {
            const name = this.renameValue.trim();
            if (!name) return;
            await this.run(
                () => renameDirectoryEntry(this.kind, this.entryId, name),
                (result) => this.handleModerationResult(result, 'Наименование исправлено'),
                'Не удалось исправить наименование'
            );
        },

        async merge(targetId) {
            await this.run(
                () => mergeDirectoryEntry(this.kind, this.entryId, targetId),
                (result) => {
                    const target = result?.target;
                    this.deletionsStore.notify({
                        prefix: `${this.label} привязана к `,
                        bold: target?.name || 'существующей записи',
                        type: 'success'
                    });
                    this.$emit('resolved', { kind: this.kind, id: target?.id ?? null, name: target?.name || '' });
                },
                'Не удалось привязать запись'
            );
        },

        /**
         * Общая обёртка действия: гасит повторные клики, показывает ошибку бэка и
         * оставляет плашку на месте при сбое - разбор не состоялся, его надо повторить.
         */
        async run(action, onSuccess, fallbackMessage) {
            if (this.busy) return;
            this.busy = true;
            try {
                onSuccess(await action());
            } catch (error) {
                this.notifyError(error, fallbackMessage);
            } finally {
                this.busy = false;
            }
        },

        /**
         * Исход подтверждения и переименования одинаков: либо запись разобрана, либо
         * наименование столкнулось с существующей записью и предлагается привязка.
         */
        handleModerationResult(result, successText) {
            if (result?.status === 'conflict') {
                this.conflict = result.existing || null;
                if (!this.conflict) {
                    // Конфликт без записи - привязывать не к чему, показываем текст бэка.
                    this.deletionsStore.notify({ bold: result?.message || 'Наименование уже занято', type: 'warning' });
                }
                return;
            }
            const entry = result?.entry;
            this.deletionsStore.notify({ prefix: `${successText}: `, bold: entry?.name || this.entryName, type: 'success' });
            this.$emit('resolved', { kind: this.kind, id: entry?.id ?? this.entryId, name: entry?.name || this.entryName });
        },

        notifyError(error, fallbackMessage) {
            this.deletionsStore.notify({ bold: error?.message || fallbackMessage, type: 'error' });
        }
    }
};
</script>

<style scoped>
.org-moderation {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-top: 12px;
    padding: 12px;
    border-radius: var(--radius-md);
    background: var(--warning-bg);
    border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
    color: var(--warning-text);
}

/* В справочнике блок разбора тоже читается как предупреждение - жёлтым, как в карточке
   заявки: секция вокруг несёт только заголовок раздела. Отступ секции (.card padding
   16px) держит блок в стороне от её скругления, поэтому рамка не срезается краем.
   Свой отступ сверху не нужен - его уже дал заголовок секции. */
.org-moderation--panel {
    margin-top: 0;
}

.org-moderation__head {
    display: flex;
    align-items: baseline;
    gap: 8px;
    flex-wrap: wrap;
}

.org-moderation__badge {
    flex: none;
    padding: 2px 8px;
    border-radius: 999px;
    background: color-mix(in srgb, var(--warning) 22%, var(--surface));
    font-size: 11px;
    font-weight: 600;
    white-space: nowrap;
}

.org-moderation__text {
    font-size: 13px;
    line-height: 1.4;
}

.org-moderation__conflict {
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: 13px;
}

.org-moderation__actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
}

.org-moderation__form {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.org-moderation__hint {
    margin: 0;
    font-size: 12px;
    opacity: 0.8;
}

.org-moderation__list {
    margin: 0;
    padding: 0;
    list-style: none;
    max-height: 180px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.org-moderation__target {
    width: 100%;
    text-align: left;
    padding: 7px 10px;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--surface);
    color: var(--text);
    font-size: 13px;
    cursor: pointer;
    transition: background-color 150ms ease;
}

.org-moderation__target:hover:not(:disabled) {
    background: var(--surface-2);
}

.org-moderation__target:disabled {
    cursor: default;
    opacity: 0.6;
}

/* Тач-таргеты ≥44px (WCAG 2.5.5, эталон адаптивности #1097): компактные кнопки
   разбора на телефоне иначе дают 24px и мажутся пальцем. */
@media (max-width: 767.98px) {
    .org-moderation__actions .lk-button,
    .org-moderation__target,
    .org-moderation .lk-input {
        min-height: 44px;
    }
}
</style>
