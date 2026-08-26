<template>
  <div class="selector">
    <!-- Мобилка: один общий список вложений всех типов; добавление - одной
         кнопкой с выпадающим меню типов (карусель-переключатель убрана). На
         десктопе всё как было: колонки типов со своими кнопками. -->
    <template v-if="isNarrow">
      <div class="picker-caption">
        Вложения заявки
      </div>
      <div class="picker-add-wrap">
        <button
          class="picker-add"
          data-testid="picker-add"
          :aria-expanded="addMenuOpen ? 'true' : 'false'"
          @click.stop="addMenuOpen = !addMenuOpen"
        >
          Добавить вложение
          <svg
            class="picker-add__arrow"
            :class="{ 'picker-add__arrow--open': addMenuOpen }"
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <polyline points="6 9 12 15 18 9" />
          </svg>
        </button>
        <transition name="picker-menu">
          <div
            v-if="addMenuOpen"
            class="picker-add-menu"
            data-testid="picker-add-menu"
          >
            <button
              v-for="category in uniqueCategories"
              :key="`add-${category}`"
              class="picker-add-menu__item"
              :disabled="getCategoryAttachments(category).length >= 10"
              @click="addFromMenu(category)"
            >
              <span>{{ templateNameFor(category) }}</span>
              <span
                v-if="getCategoryAttachments(category).length"
                class="picker-add-menu__count"
              >{{ getCategoryAttachments(category).length }}</span>
            </button>
          </div>
        </transition>
      </div>
    </template>

    <!-- Якорь онбординга - на контейнере всех категорий: списки бланков живут
         внутри v-for, и одинаковый testid на каждом из них тур цеплял по
         первому - на бланке «Сотрудники» это была пустая категория «Автомобили».
         data-selected-type даёт туру дождаться, пока выделение переедет. -->
    <div
      class="categories-container"
      data-testid="ob-blank-list"
      :data-selected-type="selectedAttachment && selectedAttachment.attachment_type"
    >
      <div
        v-for="category in uniqueCategories"
        v-show="!isNarrow || getCategoryAttachments(category).length > 0"
        :key="category"
        class="category"
      >
        <div class="category-header">
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
            :data-testid="isSelected(attachment) ? 'ob-blank-selected' : null"

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
                <AppIcon
                  v-if="isNarrow"
                  name="edit"
                  class="edit-btn__icon"
                />
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
import { useDeletionsStore } from '@/stores/deletions';
import ConfirmationModal from './ConfirmationModal.vue';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
    name: 'BlankSelector',
    components: {
      AppIcon,
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
        },
        // Выбор родителя: вложение можно открыть не только кликом по чипу
        // (восстановление черновика, удаление выбранного) - подсветка должна
        // следовать за формой, иначе форма открыта, а в списке ничего не выбрано.
        activeAttachment: {
            type: Object,
            default: null
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
            pickedCategory: null,
            // Открыто ли выпадающее меню типов у кнопки «Добавить» (мобилка).
            addMenuOpen: false
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
        },

        // Наименование ВЛОЖЕНИЯ, которое создаст кнопка («Автозаявка»), а не группы
        // («АВТОЗАЯВКИ») - из него же addAttachment строит display_name «... №N».
        pickedTemplateName() {
            return this.templateNameFor(this.pickedCategory);
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
                } else if (!this.selectedAttachment && this.activeAttachment) {
                    // Родитель выбрал вложение раньше, чем список доехал пропсом
                    // (восстановление черновика) - подсвечиваем, когда список пришёл.
                    this.setSelectedAttachment(this.activeAttachment);
                }
            },
            deep: true,
            immediate: true
        },

        activeAttachment: {
            handler(attachment) {
                const sameKey = attachment && this.selectedAttachment
                    && this.getAttachmentKey(attachment) === this.getAttachmentKey(this.selectedAttachment);
                if (sameKey) return;
                this.setSelectedAttachment(attachment);
            },
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
        // Меню типов у кнопки «Добавить» закрывается кликом вне него.
        this._onAddMenuOutside = (e) => {
            if (!this.addMenuOpen) return;
            if (!this.$el.querySelector('.picker-add-wrap')?.contains(e.target)) {
                this.addMenuOpen = false;
            }
        };
        document.addEventListener('click', this._onAddMenuOutside);
    },
    beforeUnmount() {
        this.clearBlurCancel();
        if (this._onAddMenuOutside) document.removeEventListener('click', this._onAddMenuOutside);
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

        /** Наименование вложения по типу-группе (display_name шаблона), для меню и подписей. */
        templateNameFor(category) {
            const template = this.allTemplates.find(t => t.title === category);
            return (template && template.display_name) || category;
        },

        /** Пункт меню «Добавить» на мобилке: создаёт вложение типа и закрывает меню. */
        addFromMenu(category) {
            this.addAttachment(category);
            this.addMenuOpen = false;
        },

        addAttachment(category) {
            const categoryAttachments = this.getCategoryAttachments(category);
            if (categoryAttachments.length >= 10) {
                useDeletionsStore().notify({
                    prefix: 'В категории ',
                    bold: category,
                    suffix: ' уже 10 бланков - это максимум',
                    type: 'warning',
                });
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
/* Панель - карточка на фоне страницы, поэтому несёт --surface. Без своего фона сквозь
   неё светил --bg, а полоса кнопок ниже (у неё фон есть, он перекрывает уезжающий под
   неё список) читалась серой заплатой поверх тёмной панели. */
.selector {
    width: 200px;
    flex-shrink: 0;
    height: 490px;
    border-radius: 30px;
    background: var(--surface);
    border: 1px solid var(--border);
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
    border-bottom: 1px solid var(--border);
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
    background: var(--surface);
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
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--text);
}

.action-btn.delete-selected:hover:not(:disabled),
.action-btn.delete-all:hover:not(:disabled) {
    background: var(--danger);
    color: var(--fill-text);
    border-color: var(--danger);
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
    color: var(--text-muted);
    text-transform: uppercase;
}

.attachment-count {
    font-size: 9px;
    color: var(--text-muted);
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
    /* Мягкая акцентная кнопка. Была заливка «акцент 40%» с обычным цветом текста -
       тёмный текст по синему (в светлой теме 2.2 на hover). Теперь бледная
       подложка + акцентный текст (4.6), а на hover полная заливка со светлой
       подписью (5.4/5.1). */
    background: color-mix(in srgb, var(--accent) 12%, var(--surface));
    border: 1px solid color-mix(in srgb, var(--accent) 40%, var(--surface));
    border-radius: 10px;
    font-size: 12px;
    font-weight: 500;
    color: var(--accent-text);
    cursor: pointer;
    transition: 0.4s ease;
    margin-top: 8px;
    flex-shrink: 0;
}

.add-btn:hover:not(:disabled) {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--fill-text);
}

.add-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.attachment {
    max-width: 130px;
    min-height: 25px;
    border: 1px solid var(--border);
    border-radius: 10px;
    font-size: 12px;
    font-weight: 500;
    color: var(--text);
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
    border-color: var(--accent);
}

.attachment.selected {
    border-color: var(--accent);
    background-color: color-mix(in srgb, var(--accent) 10%, var(--surface));
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
    color: var(--danger-text);
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
    background-color: color-mix(in srgb, var(--danger) 10%, var(--surface));
}

.edit-btn {
    background: none;
    border: none;
    font-size: 11px;
    color: var(--accent-text);
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
    background-color: color-mix(in srgb, var(--accent) 10%, var(--surface));
}

/* В режиме переименования чип занимает всю ширину панели - чтобы инпут был удобным. */
.attachment.editing {
    max-width: none;
}

.attachment-name-input {
    flex: 1;
    min-width: 0;
    border: 1px solid var(--accent);
    border-radius: 8px;
    padding: 2px 6px;
    font-size: 12px;
    font-weight: 500;
    color: var(--text);
    outline: none;
    background: var(--surface);
}

.tooltip {
    position: fixed;
    background: var(--hint-bg);
    color: var(--hint-text);
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
    border-bottom: 5px solid var(--overlay);
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
        /* Высота по контенту, прокрутки нет - выпадающее меню «Добавить» не должно
           обрезаться нижней кромкой карточки (десктопный overflow:hidden наследуется). */
        overflow: visible;
    }

    .picker-caption,
    .created-caption {
        font-size: 10px;
        font-weight: 700;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.03em;
        margin-bottom: 8px;
    }

    /* Пустой тип - обычная фраза, не рубрика. */
    .created-caption--empty {
        color: var(--text-muted);
        text-transform: none;
        font-weight: 500;
        font-size: 12px;
        letter-spacing: 0;
    }

    /* Кнопка «Добавить» с выпадающим меню типов - один общий список вложений
       ниже, добавление одной кнопкой (карусель-переключатель убрана). */
    .picker-add-wrap {
        position: relative;
        margin-bottom: 16px;
    }

    .picker-add {
        width: 100%;
        min-height: 50px;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        gap: 8px;
        border: none;
        border-radius: var(--radius-pill);
        background: var(--color-primary);
        color: var(--accent-contrast);
        font-size: 16px;
        font-weight: 600;
        cursor: pointer;
    }

    .picker-add__arrow {
        transition: transform 0.2s ease;
    }

    .picker-add__arrow--open {
        transform: rotate(180deg);
    }

    /* Меню по эталону BaseDropdown: радиус 20, мягкая тень, разделители,
       hover #f5f5f5, углы через overflow:hidden. */
    .picker-add-menu {
        position: absolute;
        top: calc(100% + 6px);
        left: 0;
        right: 0;
        z-index: 20;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        box-shadow: 0 6px 20px var(--shadow-drop);
    }

    .picker-add-menu__item {
        width: 100%;
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 8px;
        min-height: 48px;
        padding: 12px 16px;
        border: none;
        border-bottom: 1px solid var(--border);
        background: transparent;
        color: var(--color-text, var(--text));
        font-size: 15px;
        font-weight: 500;
        text-align: left;
        cursor: pointer;
        transition: background-color 0.15s ease;
    }

    .picker-add-menu__item:last-child {
        border-bottom: none;
    }

    .picker-add-menu__item:hover:not(:disabled),
    .picker-add-menu__item:active:not(:disabled) {
        background: var(--surface-2);
    }

    .picker-add-menu__item:disabled {
        color: var(--text-muted);
        cursor: not-allowed;
    }

    .picker-add-menu__count {
        flex-shrink: 0;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-width: 20px;
        height: 20px;
        padding: 0 6px;
        border-radius: var(--radius-pill);
        background: color-mix(in srgb, var(--accent) 14%, var(--surface));
        color: var(--accent-text);
        font-size: 12px;
        font-weight: 600;
    }

    .picker-menu-enter-active,
    .picker-menu-leave-active {
        transition: opacity 0.15s ease, transform 0.15s ease;
    }

    .picker-menu-enter-from,
    .picker-menu-leave-to {
        opacity: 0;
        transform: translateY(-6px);
    }

    /* Заголовок типа над секцией вложений в общем списке - лёгкая рубрика. */
    .category-header {
        display: flex;
        align-items: baseline;
        gap: 8px;
        margin-bottom: 6px;
        padding: 0;
        border: none;
    }

    .category-title {
        font-size: 11px;
        font-weight: 700;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.03em;
    }

    .attachment-count {
        font-size: 11px;
        color: var(--text-muted);
        font-weight: 600;
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
        background: var(--surface);
        cursor: pointer;
        flex-shrink: 0;
    }

    .rename-confirm {
        color: var(--success-text);
    }

    .rename-cancel {
        color: var(--danger-text);
        font-size: 20px;
    }

    .edit-btn__icon {
        color: var(--text);
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
