<template>
  <div class="selector">
    <!-- Мобилка: в столбик типы бланков съедали пол-экрана до самой формы. Тип
         выбирается в прокручиваемой строке, добавляет одна кнопка под ней; на
         десктопе всё как было: тип, под ним свои вложения, под ними кнопка. -->
    <template v-if="isNarrow">
      <div class="picker-caption">
        Вложения заявки
      </div>
      <div class="category-carousel">
        <button
          v-for="category in uniqueCategories"
          :key="`chip-${category}`"
          class="category-chip"
          :class="{ 'category-chip--active': category === pickedCategory }"
          @click="pickedCategory = category"
        >
          <span class="category-chip__title">{{ category }}</span>
          <span
            v-if="getCategoryAttachments(category).length"
            class="category-chip__count"
          >{{ getCategoryAttachments(category).length }}</span>
        </button>
      </div>
      <button
        class="picker-add"
        :disabled="pickedCategoryIsFull"
        @click="addAttachment(pickedCategory)"
      >
        Добавить: {{ pickedCategory }}
      </button>

      <div
        v-if="pickedCategoryAttachments.length"
        class="created-caption"
      >
        {{ pickedCategory }}
      </div>
      <div
        v-else-if="attachments.length"
        class="created-caption created-caption--empty"
      >
        В этом типе вложений пока нет
      </div>
    </template>

    <div class="categories-container">
      <div
        v-for="category in uniqueCategories"
        v-show="!isNarrow || category === pickedCategory"
        :key="category"
        class="category"
      >
        <div
          v-if="!isNarrow"
          class="category-header"
        >
          <div class="category-title">
            {{ category }}
          </div>
          <span class="attachment-count">{{ getCategoryAttachments(category).length }}/10</span>
        </div>


        <transition-group
          name="attachment"
          tag="div"
          class="attachments-list"
        >
          <div
            v-for="attachment in getCategoryAttachments(category)"
            :key="getAttachmentKey(attachment)"
            class="attachment"
            :class="{ selected: isSelected(attachment), editing: isEditing(attachment) }"
            @click="selectAttachment(attachment, $event)"
            @mouseenter="handleMouseEnter(attachment, $event)"
            @mouseleave="handleMouseLeave"
          >
            <template v-if="isEditing(attachment)">
              <input
                ref="renameInput"
                v-model="editingName"
                type="text"
                class="attachment-name-input"
                maxlength="255"
                @click.stop
                @keydown.enter.prevent="commitRename(attachment)"
                @keydown.esc.prevent="cancelRename"
                @blur="onRenameBlur(attachment)"
              >
              <!-- На телефоне Esc нет, а blur по тапу мимо ОТМЕНЯЕТ - сохранить
                   можно только явной галочкой (или Enter с клавиатуры). -->
              <button
                v-if="isNarrow"
                class="rename-confirm"
                title="Сохранить имя"
                @mousedown.prevent
                @touchstart.prevent="commitRename(attachment)"
                @click.stop="commitRename(attachment)"
              >
                <svg
                  width="12"
                  height="12"
                  viewBox="0 0 12 12"
                  fill="none"
                  aria-hidden="true"
                ><path
                  d="M2 6.5L4.8 9.2L10 3.6"
                  stroke="currentColor"
                  stroke-width="1.8"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                /></svg>
              </button>
              <button
                v-if="isNarrow"
                class="rename-cancel"
                title="Отменить"
                @mousedown.prevent
                @touchstart.prevent="cancelRename"
                @click.stop="cancelRename"
              >
                ×
              </button>
            </template>
            <template v-else>
              <input
                v-model="selectedAttachments"
                type="checkbox"
                :value="getAttachmentKey(attachment)"
                class="attachment-checkbox"
                @click.stop
              >
              <span
                class="attachment-name"
                @dblclick.stop="startRename(attachment)"
              >{{ attachment.display_name }}</span>

              <!-- На тач-экране hover не наступает: кнопки строки держим видимыми всегда. -->
              <button
                v-if="isNarrow || hoveredAttachment === getAttachmentKey(attachment)"
                class="edit-btn"
                title="Переименовать"
                @click.stop="startRename(attachment)"
              >
                <img
                  v-if="isNarrow"
                  src="@/assets/icons/edit.png"
                  alt="Переименовать"
                  class="edit-btn__icon"
                >
                <template v-else>
                  ✎
                </template>
              </button>
              <button
                v-if="isNarrow || hoveredAttachment === getAttachmentKey(attachment)"
                class="delete-btn"
                title="Удалить"
                @click.stop="confirmDelete(attachment)"
              >
                ×
              </button>
            </template>
          </div>
        </transition-group>

        <button
          v-if="!isNarrow"
          class="add-btn"
          :disabled="getCategoryAttachments(category).length >= 10"
          @click="addAttachment(category)"
        >
          Добавить
        </button>
      </div>
    </div>

    <div class="actions">
      <button
        class="action-btn delete-selected"
        :disabled="selectedAttachments.length === 0"
        @click="confirmDeleteMultiple"
      >
        Удалить выбранные
      </button>
      <button
        class="action-btn delete-all"
        :disabled="attachments.length === 0"
        @click="confirmDeleteAll"
      >
        Удалить все
      </button>
    </div>

    <ConfirmationModal
      :show="showDeleteModal"
      title="Подтверждение удаления"
      :message="deleteMessage"
      confirm-text="Удалить"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#ff4444', borderColor: '#ff4444' }"
      @confirm="deleteAttachments"
      @cancel="cancelDelete"
    />

    <div
      v-if="showTooltip"
      class="tooltip"
      :style="tooltipStyle"
    >
      {{ tooltipText }}
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { getViewportZoom } from '@/utils/viewportScale'
import { useOnboardingStore } from '@/stores/onboarding';
import ConfirmationModal from './ConfirmationModal.vue';

export default {
    name: 'BlankSelector',
    components: {
        ConfirmationModal
    },
    props: {
        attachments: {
            type: Array,
            default: () => []
        },
        currentApplicationData: {
            type: Object,
            default: () => ({})
        }
    },
    emits: ['attachment-added', 'attachment-removed', 'attachment-selected', 'attachment-renamed'],
    setup() {
        // Онбординг-тур просит показать реальную форму, добавляя демо-вложение
        // нужного типа (см. watch onboardingStore.demoAttachmentType ниже).
        return { onboardingStore: useOnboardingStore() };
    },
    data() {
        return {
            allTemplates: [],
            selectedAttachment: null,
            hoveredAttachment: null,
            showDeleteModal: false,
            attachmentsToDelete: [],
            showTooltip: false,
            tooltipText: '',
            tooltipStyle: {},
            tooltipTimeout: null,
            selectedAttachments: [],
            // Демо-вложение, добавленное онбордингом (чтобы убрать его потом).
            demoAttachment: null,
            // Inline-переименование вложения (#883): ключ редактируемого + черновик имени.
            editingKey: null,
            editingName: '',
            // Узкий экран (<=768, тот же порог, что у @media): типы бланков едут в
            // карусель, вложения показываются отдельным списком под ней.
            isNarrow: false,
            // Тип, выбранный в мобильном пикере (добавляет одна кнопка под строкой).
            pickedCategory: null
        }
    },
    computed: {
        deleteMessage() {
            if (this.attachmentsToDelete.length === 1) {
                return `Вы точно хотите удалить "${this.attachmentsToDelete[0]?.display_name}"?`;
            } else if (this.attachmentsToDelete.length === this.attachments.length && this.attachments.length > 0) {
                return `Вы точно хотите удалить ВСЕ бланки (${this.attachments.length})?`;
            } else {
                return `Вы точно хотите удалить выбранные бланки (${this.attachmentsToDelete.length})?`;
            }
        },
        uniqueCategories() {
            const categories = new Set();
            this.allTemplates.forEach(template => {
                if (template.title) {
                    categories.add(template.title);
                }
            });
            return Array.from(categories);
        },

        pickedCategoryAttachments() {
            if (!this.pickedCategory) return [];
            return this.getCategoryAttachments(this.pickedCategory);
        },

        pickedCategoryIsFull() {
            if (!this.pickedCategory) return true;
            return this.getCategoryAttachments(this.pickedCategory).length >= 10;
        }
    },
    watch: {
        // На телефоне список показывает только выбранный тип: отметки на скрывшихся
        // вложениях снимаем, иначе «Удалить выбранные» удалит то, чего не видно.
        pickedCategory() {
            if (!this.isNarrow || !this.selectedAttachments.length) return;
            const visible = new Set(
                this.pickedCategoryAttachments.map(a => this.getAttachmentKey(a))
            );
            this.selectedAttachments = this.selectedAttachments.filter(k => visible.has(k));
        },

        // Первый тип встаёт выбранным сразу, чтобы кнопка добавления не была мёртвой.
        uniqueCategories: {
            immediate: true,
            handler(categories) {
                if (!this.pickedCategory || !categories.includes(this.pickedCategory)) {
                    this.pickedCategory = categories[0] || null;
                }
            }
        },
        attachments: {
            handler(newAttachments) {
                const existingIds = new Set(newAttachments.map(a => this.getAttachmentKey(a)));
                this.selectedAttachments = this.selectedAttachments.filter(id => existingIds.has(id));

                // Редактируемое вложение удалили - выходим из режима переименования.
                if (this.editingKey !== null && !existingIds.has(this.editingKey)) {
                    this.cancelRename();
                }

                if (this.selectedAttachment && !existingIds.has(this.getAttachmentKey(this.selectedAttachment))) {
                    this.selectedAttachment = null;
                    this.$emit('attachment-selected', null);
                } else if (this.selectedAttachment && existingIds.has(this.getAttachmentKey(this.selectedAttachment))) {
                    const currentAttachment = newAttachments.find(a =>
                        this.getAttachmentKey(a) === this.getAttachmentKey(this.selectedAttachment)
                    );
                    if (currentAttachment && currentAttachment !== this.selectedAttachment) {
                        this.selectedAttachment = currentAttachment;
                    }
                }
            },
            deep: true,
            immediate: true
        },
        // Онбординг просит показать форму вложения нужного типа: добавляем
        // демо-вложение, при смене типа/сбросе - убираем прежнее.
        'onboardingStore.demoAttachmentType'(type) {
            this.applyDemoAttachment(type);
        }
    },
    mounted() {
        this.fetchTemplates();
        this.initNarrowWatcher();
    },
    beforeUnmount() {
        this.clearBlurCancel();
        // Уходя со страницы посреди тура - не оставить демо-вложение висеть.
        this.removeDemoAttachment();
        if (this._narrowMql) {
            if (this._narrowMql.removeEventListener) {
                this._narrowMql.removeEventListener('change', this._onNarrowChange);
            } else if (this._narrowMql.removeListener) {
                this._narrowMql.removeListener(this._onNarrowChange);
            }
        }
    },
    methods: {
        initNarrowWatcher() {
            if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
            // Матчер держим вне data: реактивность ему не нужна, а ключи с _ в data запрещены линтом.
            this._narrowMql = window.matchMedia('(max-width: 768px)');
            this.isNarrow = this._narrowMql.matches;
            this._onNarrowChange = (e) => { this.isNarrow = e.matches; };
            if (this._narrowMql.addEventListener) {
                this._narrowMql.addEventListener('change', this._onNarrowChange);
            } else if (this._narrowMql.addListener) {
                this._narrowMql.addListener(this._onNarrowChange);
            }
        },

        getAttachmentKey(attachment) {
            return attachment.local_id || attachment.id;
        },

        isSelected(attachment) {
            if (!this.selectedAttachment) return false;
            return this.getAttachmentKey(this.selectedAttachment) === this.getAttachmentKey(attachment);
        },

        setSelectedAttachment(attachment) {
            if (attachment) {
                const currentAttachment = this.attachments.find(a =>
                    this.getAttachmentKey(a) === this.getAttachmentKey(attachment)
                );
                this.selectedAttachment = currentAttachment || null;
            } else {
                this.selectedAttachment = null;
            }
        },

        getCategoryAttachments(category) {
            return this.attachments.filter(attachment => attachment.title === category);
        },

        getNextAttachmentNumber(category) {
            const categoryAttachments = this.getCategoryAttachments(category);
            const existingNumbers = new Set();

            categoryAttachments.forEach(attachment => {
                const match = attachment.display_name.match(/\d+$/);
                if (match) {
                    existingNumbers.add(parseInt(match[0]));
                }
            });

            let number = 1;
            while (existingNumbers.has(number)) {
                number++;
            }
            return number;
        },

        async fetchTemplates() {
            try {
                const response = await apiRequest("/attachments", {
                });
                if (response.ok) {
                    const data = await response.json();
                    this.allTemplates = data;
                    // Тур мог попросить демо-вложение раньше, чем подгрузились
                    // шаблоны (watch отработал вхолостую) - применяем сейчас.
                    if (this.onboardingStore.demoAttachmentType) {
                        this.applyDemoAttachment(this.onboardingStore.demoAttachmentType);
                    }
                }
            } catch (error) {
                console.error("Error fetching attachment templates:", error);
            }
        },

        /**
         * Онбординг: добавить демо-вложение типа type ('cars'/'people'/'items'),
         * чтобы тур показал реальную форму. Прежнее демо снимается. Демо помечено
         * __onboardingDemo - CreateApplication не персистит его в localStorage.
         *
         * @param {string|null} type
         */
        applyDemoAttachment(type) {
            this.removeDemoAttachment();
            if (!type) return;

            const template = this.allTemplates.find(t => t.attachment_type === type);
            // Шаблоны ещё не загрузились - fetchTemplates применит демо после загрузки.
            if (!template) return;

            const attachment = {
                id: template.id,
                local_id: `onboarding-demo-${type}`,
                template_id: template.id,
                title: template.title,
                name: `${template.name}_demo`,
                display_name: template.display_name,
                attachment_type: template.attachment_type,
                instruction: template.instruction,
                created_at: new Date().toISOString(),
                is_active: true,
                __onboardingDemo: true
            };

            this.demoAttachment = attachment;
            this.$emit('attachment-added', attachment);
        },

        /**
         * Убрать ранее добавленное демо-вложение онбординга, если оно есть.
         */
        removeDemoAttachment() {
            if (!this.demoAttachment) return;
            const attachment = this.demoAttachment;
            this.demoAttachment = null;
            this.$emit('attachment-removed', attachment);
        },

        addAttachment(category) {
            const categoryAttachments = this.getCategoryAttachments(category);
            if (categoryAttachments.length >= 10) {
                alert(`Максимальное количество бланков в категории "${category}" — 10.`);
                return;
            }

            const template = this.allTemplates.find(t => t.title === category);

            if (!template) {
                console.warn(`No template found for category: ${category}`);
                return;
            }

            const nextNumber = this.getNextAttachmentNumber(category);
            const newAttachment = {
                id: template.id,
                local_id: Date.now() + Math.random(),
                template_id: template.id,
                title: category,
                name: `${template.name}_${nextNumber}`,
                display_name: `${template.display_name} №${nextNumber}`,
                attachment_type: template.attachment_type,
                instruction: template.instruction,
                created_at: new Date().toISOString(),
                is_active: true
            };

            this.$emit('attachment-added', newAttachment);
        },

        selectAttachment(attachment, event) {
            // Инструкция принадлежит ТИПУ вложения (админка) и могла измениться после
            // добавления черновика - объект вложения хранит снапшот на момент добавления.
            // Берём актуальную инструкцию из загруженных шаблонов, иначе восстановленный
            // черновик (или тип, которому инструкцию дописали позже) не покажет кнопку.
            const enriched = this.withCurrentInstruction(attachment);
            this.keepRowInPlace(event && event.currentTarget);
            this.selectedAttachment = enriched;
            this.$emit('attachment-selected', enriched);
        },

        /**
         * Форма другого вложения выше или ниже прежней, страница меняет высоту, и
         * браузер сдвигает прокрутку - на телефоне это выглядит как прыжок к началу.
         * Держим кликнутую строку на том же месте экрана.
         */
        keepRowInPlace(row) {
            if (!this.isNarrow || !row || typeof window === 'undefined') return;
            const before = row.getBoundingClientRect().top;
            this.$nextTick(() => {
                window.requestAnimationFrame(() => {
                    const shift = row.getBoundingClientRect().top - before;
                    if (Math.abs(shift) > 1) window.scrollBy(0, shift);
                });
            });
        },

        isEditing(attachment) {
            return this.editingKey !== null && this.editingKey === this.getAttachmentKey(attachment);
        },

        startRename(attachment) {
            this.editingKey = this.getAttachmentKey(attachment);
            this.editingName = attachment.display_name || '';
            // На телефоне автофокус тут же выбрасывает клавиатуру на пол-экрана,
            // хотя пользователь мог открыть переименование, чтобы просто посмотреть
            // полное имя. На десктопе поведение прежнее.
            if (this.isNarrow) return;
            this.$nextTick(() => {
                const input = this.$el.querySelector('.attachment-name-input');
                if (input) {
                    input.focus();
                    input.select();
                }
            });
        },

        /**
         * Blur на десктопе сохраняет (привычный inline-edit), на телефоне - отменяет:
         * там blur это тап мимо, а не осознанное подтверждение, и без Esc иначе
         * не выйти из редактирования, не тронув имя. Отмена отложена: порядок
         * blur против touchstart браузеро-зависим, и мгновенная отмена съедала бы
         * тап по галочке - commit и cancel от кнопок снимают таймер.
         */
        onRenameBlur(attachment) {
            if (!this.isNarrow) {
                this.commitRename(attachment);
                return;
            }
            this.clearBlurCancel();
            this.blurCancelTimer = setTimeout(() => this.cancelRename(), 250);
        },

        clearBlurCancel() {
            if (this.blurCancelTimer) {
                clearTimeout(this.blurCancelTimer);
                this.blurCancelTimer = null;
            }
        },

        commitRename(attachment) {
            this.clearBlurCancel();
            // Enter и blur оба зовут commit; после первого editingKey уже null - выходим.
            if (this.editingKey === null) return;
            const name = this.editingName.trim();
            this.editingKey = null;
            this.editingName = '';
            // Пустое имя или без изменений - откатываем без эмита.
            if (!name || name === attachment.display_name) return;
            this.$emit('attachment-renamed', { attachment, display_name: name });
        },

        cancelRename() {
            this.clearBlurCancel();
            this.editingKey = null;
            this.editingName = '';
        },

        withCurrentInstruction(attachment) {
            if (!attachment) return attachment;
            const templateId = attachment.template_id || attachment.id;
            const template = this.allTemplates.find(t => t.id === templateId);
            if (template && template.instruction !== attachment.instruction) {
                return { ...attachment, instruction: template.instruction };
            }
            return attachment;
        },

        confirmDelete(attachment) {
            this.attachmentsToDelete = [attachment];
            this.showDeleteModal = true;
        },

        confirmDeleteMultiple() {
            if (this.selectedAttachments.length === 0) return;

            this.attachmentsToDelete = this.attachments.filter(
                a => this.selectedAttachments.includes(this.getAttachmentKey(a))
            );
            this.showDeleteModal = true;
        },

        confirmDeleteAll() {
            if (this.attachments.length === 0) return;
            this.attachmentsToDelete = [...this.attachments];
            this.showDeleteModal = true;
        },

        cancelDelete() {
            this.showDeleteModal = false;
            this.attachmentsToDelete = [];
        },

        deleteAttachments() {
            if (this.attachmentsToDelete.length === 0) return;

            const keysToDelete = new Set(this.attachmentsToDelete.map(a => this.getAttachmentKey(a)));

            this.attachmentsToDelete.forEach(attachment => {
                this.$emit('attachment-removed', attachment);
            });

            if (this.selectedAttachment && keysToDelete.has(this.getAttachmentKey(this.selectedAttachment))) {
                this.selectedAttachment = null;
                this.$emit('attachment-selected', null);
            }

            this.selectedAttachments = this.selectedAttachments.filter(id => !keysToDelete.has(id));
            this.showDeleteModal = false;
            this.attachmentsToDelete = [];
        },

        handleMouseEnter(attachment, event) {
            this.hoveredAttachment = this.getAttachmentKey(attachment);

            if (this.tooltipTimeout) {
                clearTimeout(this.tooltipTimeout);
            }

            this.tooltipTimeout = setTimeout(() => {
                this.showTooltip = true;
                this.tooltipText = attachment.display_name;
                this.updateTooltipPosition(event);
            }, 800);
        },

        handleMouseLeave() {
            this.hoveredAttachment = null;
            this.showTooltip = false;
            this.tooltipText = '';

            if (this.tooltipTimeout) {
                clearTimeout(this.tooltipTimeout);
                this.tooltipTimeout = null;
            }
        },

        updateTooltipPosition(event) {
            if (event && event.target) {
                // position:fixed тултип внутри зазумленного <html>: rect device-px ->
                // делим на zoom (inline left/top = layout-px). Отступ +7 не делим.
                const z = getViewportZoom();
                const rect = event.target.getBoundingClientRect();
                this.tooltipStyle = {
                    left: `${(rect.left + rect.width / 2) / z}px`,
                    top: `${rect.bottom / z + 7}px`,
                    transform: 'translateX(-50%)'
                };
            }
        },

        clearSelection() {
            this.selectedAttachments = [];
            this.selectedAttachment = null;
        }
    }
}
</script>

<style scoped>
.selector {
    width: 200px;
    flex-shrink: 0;
    height: 490px;
    border-radius: 30px;
    border: 1px solid #e6e6e6;
    padding: 15px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    position: relative;
    transition: 0.4s;
}

.categories-container {
    flex: 1;
    overflow-y: auto;
    scrollbar-width: none;
    -ms-overflow-style: none;
    margin-bottom: 10px;
    border-bottom: 1px solid #e6e6e6;
}

.categories-container::-webkit-scrollbar {
    display: none;
}

.actions {
    flex-shrink: 0;
    display: flex;
    gap: 8px;
    flex-direction: column;
    margin-top: 10px;
    background: white;
    z-index: 10;
}

.action-btn {
    flex: 1;
    padding: 4px 8px;
    border-radius: 10px;
    font-size: 11px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.3s ease;
    border: 1px solid #e6e6e6;
    background: white;
    color: #333;
}

.action-btn.delete-selected:hover:not(:disabled),
.action-btn.delete-all:hover:not(:disabled) {
    background: #ff4444;
    color: white;
    border-color: #ff4444;
}

.action-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.category {
    margin-bottom: 20px;
    display: flex;
    flex-direction: column;
    min-height: 0;
    transition: 0.4s;
}

.category-header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 5px;
}

.category-title {
    font-size: 10px;
    font-weight: 700;
    color: #a2a2a2;
    text-transform: uppercase;
}

.attachment-count {
    font-size: 9px;
    color: #a2a2a2;
}

.attachments-list {
    display: flex;
    flex-direction: column;
    gap: 5px;
    flex: 1;
    overflow-y: auto;
    scrollbar-width: none;
    -ms-overflow-style: none;
    min-height: 0;
}

.attachments-list::-webkit-scrollbar {
    display: none;
}

.attachment-enter-active,
.attachment-leave-active {
    transition: all 0.2s ease;
}

.attachment-enter-from {
    opacity: 0;
    transform: scale(0.8);
}

.attachment-leave-to {
    opacity: 0;
    transform: scale(0.8);
}

.attachment-move {
    transition: transform 0.2s ease;
}

.add-btn {
    width: 85px;
    height: 25px;
    background: rgba(79, 91, 223, 0.4);
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    font-size: 12px;
    font-weight: 500;
    color: #fff;
    cursor: pointer;
    transition: 0.4s ease;
    margin-top: 8px;
    flex-shrink: 0;
}

.add-btn:hover:not(:disabled) {
    background: rgba(79, 91, 223, 1);
}

.add-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.attachment {
    max-width: 130px;
    min-height: 25px;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    font-size: 12px;
    font-weight: 500;
    color: #000;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 0 8px;
    cursor: pointer;
    transition: all 0.3s ease;
    position: relative;
    flex-shrink: 0;
}

.attachment:hover {
    border-color: #4F5BDF;
}

.attachment.selected {
    border-color: #4F5BDF;
    background-color: rgba(79, 91, 223, 0.1);
}

.attachment-checkbox {
    margin: 0;
    width: 12px;
    height: 12px;
    cursor: pointer;
    flex-shrink: 0;
}

.attachment-name {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
}

.delete-btn {
    background: none;
    border: none;
    font-size: 16px;
    color: #ff4444;
    cursor: pointer;
    padding: 0;
    width: 16px;
    height: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    transition: background-color 0.3s ease;
    flex-shrink: 0;
}

.delete-btn:hover {
    background-color: rgba(255, 68, 68, 0.1);
}

.edit-btn {
    background: none;
    border: none;
    font-size: 11px;
    color: #4F5BDF;
    cursor: pointer;
    padding: 0;
    width: 16px;
    height: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    transition: background-color 0.3s ease;
    flex-shrink: 0;
}

.edit-btn:hover {
    background-color: rgba(79, 91, 223, 0.1);
}

/* В режиме переименования чип занимает всю ширину панели - чтобы инпут был удобным. */
.attachment.editing {
    max-width: none;
}

.attachment-name-input {
    flex: 1;
    min-width: 0;
    border: 1px solid #4F5BDF;
    border-radius: 8px;
    padding: 2px 6px;
    font-size: 12px;
    font-weight: 500;
    color: #000;
    outline: none;
    background: #fff;
}

.tooltip {
    position: fixed;
    background: rgba(0, 0, 0, 0.8);
    color: white;
    padding: 6px 10px;
    border-radius: 15px;
    font-size: 11px;
    white-space: nowrap;
    z-index: 1001;
    pointer-events: none;
    animation: fadeIn 0.2s ease-in-out;
}

.tooltip::before {
    content: '';
    position: absolute;
    top: -5px;
    left: 50%;
    transform: translateX(-50%);
    width: 0;
    height: 0;
    border-left: 5px solid transparent;
    border-right: 5px solid transparent;
    border-bottom: 5px solid rgba(0, 0, 0, 0.8);
}

@keyframes fadeIn {
    from {
        opacity: 0;
        transform: translateX(-50%) translateY(-5px);
    }
    to {
        opacity: 1;
        transform: translateX(-50%) translateY(0);
    }
}

/* ── Мобилка: выбор типа строкой + список созданных вложений ── */
@media (max-width: 768px) {
    .selector {
        width: 100%;
        height: auto;
        max-height: none;
        border-radius: var(--radius-lg);
        padding: 12px;
    }

    .picker-caption,
    .created-caption {
        font-size: 10px;
        font-weight: 700;
        color: #a2a2a2;
        text-transform: uppercase;
        letter-spacing: 0.03em;
        margin-bottom: 8px;
    }

    /* Пустой тип - обычная фраза, не рубрика. */
    .created-caption--empty {
        color: #c4c4c4;
        text-transform: none;
        font-weight: 500;
        font-size: 12px;
        letter-spacing: 0;
    }

    /* Типы - одна прокручиваемая строка (паттерн быстрых периодов календаря):
       чипы уезжают под край, видно, что ряд продолжается. */
    /* Отрицательные margin уводили строку левее карточки и налезали на её край -
       прокручиваем внутри контента, а не под ним. */
    .category-carousel {
        display: flex;
        flex-direction: row;
        flex-wrap: nowrap;
        gap: 8px;
        margin: 0;
        padding: 0 0 2px;
        overflow-x: auto;
        overflow-y: hidden;
        scroll-snap-type: x proximity;
        -webkit-overflow-scrolling: touch;
        overscroll-behavior-x: contain;
        scrollbar-width: none;
    }

    .category-carousel::-webkit-scrollbar {
        display: none;
    }

    .category-chip {
        flex: 0 0 auto;
        display: inline-flex;
        align-items: center;
        gap: 6px;
        height: 36px;
        padding: 0 14px;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-pill);
        background: #fff;
        color: #666;
        font-size: 13px;
        font-weight: 500;
        white-space: nowrap;
        cursor: pointer;
        scroll-snap-align: start;
        transition: background-color 0.2s ease, color 0.2s ease, border-color 0.2s ease;
    }

    /* Выбранный тип - светлая пилюля с синей рамкой: сплошная заливка сливалась
       с синей кнопкой добавления прямо под строкой. */
    .category-chip--active {
        background: var(--color-primary-tint);
        border-color: var(--color-primary);
        color: var(--color-primary);
        font-weight: 600;
    }

    .category-chip__count {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-width: 18px;
        height: 18px;
        padding: 0 5px;
        border-radius: var(--radius-pill);
        background: rgba(0, 0, 0, 0.08);
        font-size: 11px;
        font-weight: 600;
    }

    .category-chip--active .category-chip__count {
        background: rgba(79, 91, 223, 0.16);
        color: var(--color-primary);
    }

    /* Одна кнопка на выбранный тип - раньше кнопка сидела в каждом чипе и
       вылезала за его границы. */
    .picker-add {
        width: 100%;
        min-height: 44px;
        margin: 10px 0 16px;
        border: none;
        border-radius: var(--radius-md);
        background: var(--color-primary);
        color: #fff;
        font-size: 14px;
        font-weight: 600;
        cursor: pointer;
    }

    .picker-add:disabled {
        background: #a2a2a2;
        opacity: 0.6;
        cursor: not-allowed;
    }

    .categories-container {
        overflow-y: visible;
        margin-bottom: 0;
        border-bottom: none;
    }

    .category {
        margin-bottom: 14px;
    }

    /* Поле переименования по размеру строки: 12px и padding 2px делали его
       заметно ниже и мельче остальных. */
    .attachment-name-input {
        min-height: 32px;
        padding: 4px 8px;
        font-size: 13px;
        border-radius: var(--radius-sm);
    }

    /* Тач-таргеты: строка вложения и кнопки под палец. Ширина 130px была
       рассчитана на узкую колонку десктопа - здесь селектор во всю страницу. */
    .attachment {
        max-width: none;
        width: 100%;
        min-height: 44px;
        gap: 8px;
        padding: 0 10px;
        border-radius: var(--radius-md);
        font-size: 13px;
    }

    .attachment-checkbox {
        width: 16px;
        height: 16px;
    }

    .edit-btn,
    .delete-btn {
        min-width: 36px;
        min-height: 36px;
    }

    .rename-confirm,
    .rename-cancel {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-width: 36px;
        min-height: 36px;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-sm);
        background: #fff;
        cursor: pointer;
        flex-shrink: 0;
    }

    .rename-confirm {
        color: #2e9e5b;
    }

    .rename-cancel {
        color: var(--color-danger);
        font-size: 20px;
    }

    .edit-btn__icon {
        width: 16px;
        height: 16px;
        opacity: 0.65;
    }

    .delete-btn {
        font-size: 20px;
    }

    .actions {
        flex-direction: row;
        gap: 8px;
        margin-top: 12px;
        padding-top: 12px;
        border-top: 1px solid var(--color-border);
    }

    .action-btn {
        min-height: 36px;
        font-size: 12px;
        border-radius: var(--radius-pill);
    }
}
</style>
