<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modal-overlay"
        @click.self="handleClose"
      >
        <div
          ref="modalContent"
          class="modal-content horizontal-modal"
        >
        <div class="modal-header">
          <h3>Создать новую таблицу</h3>
          <button
            class="modal-close"
            @click="handleClose"
          >
            <svg
              width="10"
              height="10"
              viewBox="0 0 14 14"
              fill="none"
            >
              <path
                d="M13 1L1 13M1 1L13 13"
                stroke="#666"
                stroke-width="2"
                stroke-linecap="round"
              />
            </svg>
          </button>
        </div>

        <div class="modal-body-horizontal">
          <!-- Левая часть - основная информация -->
          <div class="modal-main-info">
            <div class="main-fields">
              <div class="form-group-compact">
                <label class="form-label-compact">Наименование таблицы *</label>
                <input
                  v-model="newTable.display_name"
                  placeholder="ПОСТ №27"
                  class="input-compact"
                >
              </div>

              <div class="form-group-compact">
                <label class="form-label-compact">Системное имя *</label>
                <input
                  v-model="newTable.name"
                  placeholder="post_27"
                  class="input-compact"
                  @input="validateSystemName"
                >
                <span class="form-hint">Латинские буквы, цифры и подчеркивания</span>
                <span
                  v-if="nameError"
                  class="form-error"
                >{{ nameError }}</span>
              </div>

              <div class="form-group-compact">
                <label class="form-label-compact">Тип таблицы *</label>
                <div class="custom-select">
                  <div
                    class="select-header"
                    @click="toggleTypeDropdown"
                  >
                    <span class="select-value">{{ getTableTypeLabel(newTable.table_type) }}</span>
                    <AppIcon
                      name="arrow"
                      class="select-arrow"
                      :class="{ rotated: typeDropdownOpen }"
                    />
                  </div>
                  <transition name="dropdown-fade">
                    <div
                      v-if="typeDropdownOpen"
                      class="select-dropdown"
                    >
                      <div
                        class="select-option"
                        :class="{ active: newTable.table_type === 'cars' }"
                        @click="selectType('cars')"
                      >
                        Машины
                      </div>
                      <div
                        class="select-option"
                        :class="{ active: newTable.table_type === 'people' }"
                        @click="selectType('people')"
                      >
                        Люди
                      </div>
                    </div>
                  </transition>
                </div>
              </div>
            </div>
          </div>

          <!-- Правая часть - настройки -->
          <div class="modal-cells-section">
            <div class="cells-header-compact">
              <h4 class="cells-title-compact">
                Настройки отображения
              </h4>
            </div>

            <div class="cells-scroll-container">
              <div class="settings-grid">
                <div class="checkbox-group">
                  <label class="checkbox-label">
                    <input
                      v-model="newTable.show_fact_table"
                      type="checkbox"
                      class="checkbox-input"
                    >
                    <span class="checkbox-text">Отображать таблицу "по факту"</span>
                  </label>
                  <p class="field-hint">
                    На странице с основной таблицей отображается таблица
                    "по факту". В ней отображаются люди/машины, данные которых
                    заранее не известны.
                  </p>
                </div>

                <div
                  v-if="newTable.show_fact_table"
                  class="setting-item"
                >
                  <label class="form-label-compact">Подсказка для таблицы "по факту"</label>
                  <TextConstructor
                    v-model="newTable.fact_table_hint"
                    :placeholder="getDefaultHint(newTable.table_type)"
                    rows="3"
                  />
                </div>

                <div class="setting-item">
                  <label class="form-label-compact">Инструкция к таблице</label>
                  <TextConstructor
                    v-model="newTable.instruction"
                    placeholder="Введите инструкцию для таблицы..."
                    rows="4"
                  />
                </div>

                <div class="setting-item">
                  <label class="form-label-compact">Предупреждение</label>
                  <textarea
                    v-model="newTable.warning"
                    class="warning-textarea"
                    placeholder="Показывается заявителю всегда (необязательно)"
                    rows="2"
                  />
                  <p class="field-hint">
                    Свободное предупреждение при добавлении машины/человека
                    с этой таблицей. Окна по времени задаются после создания.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button
            class="modal-btn modal-btn--cancel"
            @click="handleClose"
          >
            Отмена
          </button>
          <button
            class="modal-btn modal-btn--confirm"
            @click="createTable"
          >
            Создать
          </button>
        </div>
      </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useDeletionsStore } from '@/stores/deletions'
import TextConstructor from './TextConstructor.vue'
import AppIcon from '@/components/icons/AppIcon.vue'

export default {
  name: 'TableConstructorCreateModal',
  components: {
    AppIcon,
    TextConstructor
  },
  props: {
    show: { type: Boolean, default: false }
  },
  emits: ['created', 'close'],
  watch: {
    show(v) {
      if (!v) {
        this.resetForm()
        this.typeDropdownOpen = false
      }
    }
  },
  data() {
    return {
      newTable: {
        name: '',
        display_name: '',
        table_type: 'people',
        show_fact_table: false,
        fact_table_hint: '',
        instruction: '',
        map_link: '',
        status: 'active',
        status_comment: '',
        location_description: '',
        warning: '',
        is_active: true
      },
      nameError: '',
      typeDropdownOpen: false
    }
  },
  mounted() {
    document.addEventListener('click', this.handleOutsideClick)
  },
  beforeUnmount() {
    document.removeEventListener('click', this.handleOutsideClick)
  },
  methods: {
    handleOutsideClick(e) {
      // Через Teleport this.$el остаётся anchor-узлом в исходном месте, а не
      // содержимым модалки в body - .contains() для него всегда false и любой
      // клик закрывал dropdown. Проверяем по ref на .modal-content внутри
      // тела модалки.
      const root = this.$refs.modalContent
      if (root && !root.contains(e.target)) {
        this.typeDropdownOpen = false
      }
    },

    validateSystemName() {
      const nameRegex = /^[a-z0-9_]*$/
      if (!nameRegex.test(this.newTable.name)) {
        this.nameError = 'Только латинские буквы, цифры и подчеркивания'
      } else {
        this.nameError = ''
      }
    },

    toggleTypeDropdown() {
      this.typeDropdownOpen = !this.typeDropdownOpen
    },

    selectType(type) {
      this.newTable.table_type = type
      this.newTable.fact_table_hint = ''
      this.typeDropdownOpen = false
    },

    getTableTypeLabel(type) {
      return type === 'cars' ? 'Машины' : 'Люди'
    },

    getDefaultHint(tableType) {
      if (tableType === 'cars') {
        return 'При прибытии автомобиля ПО ФАКТУ: спроси у водителя организацию, посмотри, есть ли организация в таблице слева, если организация есть - пропустить'
      } else {
        return 'При проходе человека ПО ФАКТУ: проверьте документы, сверьте с данными в системе'
      }
    },

    async createTable() {
      if (!this.newTable.name.trim() || !this.newTable.display_name.trim()) {
        useDeletionsStore().notify({ prefix: 'Заполните ', bold: 'обязательные поля', type: 'error' });
        return
      }

      const nameRegex = /^[a-z0-9_]+$/
      if (!nameRegex.test(this.newTable.name)) {
        useDeletionsStore().notify({ prefix: 'Системное имя: только ', bold: 'латиница, цифры, _', type: 'error' });
        return
      }

      try {
        const response = await apiRequest('/system-tables', {
          method: 'POST',
          body: JSON.stringify(this.newTable)
        })

        if (response.ok) {
          const result = await response.json()
          this.$emit('created', result)
          this.resetForm()
        } else {
          // wrapJsonUnwrap на !success кладёт текст ошибки бэка в message (в самом
          // envelope ключ - error); сырой response.text() дал бы JSON целиком.
          let message = 'Ошибка при создании таблицы'
          try {
            const body = await response.json()
            if (body && body.message) message = body.message
          } catch { /* тело не JSON - остаётся дефолт */ }
          useDeletionsStore().notify({ prefix: 'Ошибка создания: ', bold: message, type: 'error' });
        }
      } catch (error) {
        useDeletionsStore().notify({ prefix: 'Ошибка сети: ', bold: error.message, type: 'error' });
      }
    },

    handleClose() {
      this.resetForm()
      this.$emit('close')
    },

    resetForm() {
      this.newTable = {
        name: '',
        display_name: '',
        table_type: 'people',
        show_fact_table: false,
        fact_table_hint: '',
        instruction: '',
        map_link: '',
        status: 'active',
        status_comment: '',
        location_description: '',
        warning: '',
        is_active: true
      }
      this.nameError = ''
      this.typeDropdownOpen = false
    }
  }
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  animation: overlayAppear 0.3s ease-out;
}

@keyframes overlayAppear {
  from {
    background: var(--overlay);
    backdrop-filter: blur(0px);
  }
  to {
    background: var(--overlay);
    backdrop-filter: blur(0.1px);
  }
}

.modal-content {
  background: var(--surface);
  border-radius: 35px;
  padding: 0;
  width: 420px;
  max-width: 90vw;
  box-shadow: 0 20px 60px var(--shadow-drop);
  animation: modalAppear 0.3s ease-out;
  overflow: hidden;
}

.modal-content.horizontal-modal {
  width: 900px;
  max-width: 90vw;
}

@keyframes modalAppear {
  from {
    opacity: 0;
    transform: scale(0.8) translateY(-20px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.modal-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.modal-close {
  background: none;
  border: none;
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s ease;
}

.modal-close:hover {
  background-color: var(--surface-2);
}

.modal-body-horizontal {
  display: flex;
  height: 400px;
}

.modal-main-info {
  width: 25%;
  padding: 20px;
  border-right: 1px solid var(--border);
  background: var(--surface-2);
  /* visible нужен чтобы выпадающий список "Тип таблицы" не клиппился
     scroll-контекстом панели и был кликабелен. */
  overflow: visible;
  position: relative;
  z-index: 2;
}

.main-fields {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group-compact {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-label-compact {
  font-size: 12px;
  font-weight: 500;
  color: var(--text);
}

.input-compact {
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 13px;
  background: var(--surface);
  transition: border-color 0.2s;
  height: 35px;
}

.input-compact:focus {
  border-color: var(--accent);
  outline: none;
}

.warning-textarea {
  width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 13px;
  font-family: inherit;
  background: var(--surface);
  transition: border-color 0.2s;
  resize: vertical;
  box-sizing: border-box;
}
.warning-textarea:focus {
  border-color: var(--accent);
  outline: none;
}

.form-hint {
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 2px;
}

.form-error {
  font-size: 11px;
  color: var(--danger-text);
  margin-top: 2px;
}

.custom-select {
  position: relative;
  width: 100%;
}

.select-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 15px;
  background: var(--surface);
  cursor: pointer;
  font-size: 13px;
  height: 35px;
  transition: all 0.2s ease;
}

.select-header:hover {
  border-color: var(--border);
  background: var(--surface-2);
}

.select-value {
  color: var(--text);
}

.select-arrow {
  width: 7px;
  height: 7px;
  transition: transform 0.2s ease;
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
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 4px 12px var(--shadow-drop);
  z-index: 10;
  margin-top: 4px;
  overflow: hidden;
}

.select-option {
  padding: 8px 12px;
  font-size: 13px;
  cursor: pointer;
  transition: background 0.2s;
  border-bottom: 1px solid var(--border);
}

.select-option:last-child {
  border-bottom: none;
}

.select-option:hover {
  background: var(--accent-tint);
  color: var(--accent-text);
}

.select-option.active {
  background: var(--accent-tint);
  color: var(--accent-text);
  font-weight: 500;
}

.dropdown-fade-enter-active,
.dropdown-fade-leave-active {
  transition: all 0.2s ease;
}

.dropdown-fade-enter-from,
.dropdown-fade-leave-to {
  opacity: 0;
  transform: translateY(-5px);
}

.modal-cells-section {
  width: 75%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.cells-header-compact {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
}

.cells-title-compact {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
}

.cells-scroll-container {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
}

.settings-grid {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.setting-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* Чекбокс "Отображать таблицу по факту" - тот же визуальный стиль, что в
   TableConstructor "Основное", чтобы пользователь видел консистентный UI
   и до создания таблицы, и при редактировании после. */
.checkbox-group {
  padding: 12px;
  background: var(--accent-tint);
  border-radius: 20px;
  border: 1px solid var(--border);
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.checkbox-input {
  width: 16px;
  height: 16px;
  cursor: pointer;
  accent-color: var(--accent-text);
}

.checkbox-text {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

.field-hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 20px;
  border-top: 1px solid var(--border);
}

.modal-btn {
  padding: 8px 20px;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  transition: background-color 0.2s ease;
  min-width: 90px;
}

.modal-btn--cancel {
  background: var(--surface-2);
  color: var(--text-muted);
  border: 1px solid var(--border);
}

.modal-btn--cancel:hover {
  background: var(--accent-tint);
}

.modal-btn--confirm {
  background: var(--accent);
  color: var(--accent-contrast);
}

.modal-btn--confirm:hover:not(:disabled) {
  background: var(--accent-hover);
}

/* Анимации */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.3s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .modal-overlay,
.modal-fade-leave-active .modal-overlay {
  transition: all 0.3s ease;
}

.modal-fade-enter-active .modal-content,
.modal-fade-leave-active .modal-content {
  transition: all 0.3s ease;
}

.modal-fade-enter-from .modal-overlay,
.modal-fade-leave-to .modal-overlay {
  background: transparent;
  backdrop-filter: blur(0px);
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
  opacity: 0;
  transform: scale(0.8) translateY(-20px);
}

/* Скроллбар */
.cells-scroll-container::-webkit-scrollbar {
  width: 6px;
}

.cells-scroll-container::-webkit-scrollbar-track {
  background: var(--surface-2);
  border-radius: 3px;
}

.cells-scroll-container::-webkit-scrollbar-thumb {
  background: var(--border);
  border-radius: 3px;
}

.cells-scroll-container::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
}

@media (max-width: 768px) {
  .modal-body-horizontal {
    flex-direction: column;
    height: auto;
  }

  .modal-main-info,
  .modal-cells-section {
    width: 100%;
  }

  .modal-main-info {
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
}
</style>
