<template>
  <div class="attachments-management-container dashboard-card">
    <div class="management-header">
      <h3 class="management-title">
        Вложения заявок (бланки)
      </h3>
      <div class="header-controls">
        <button 
          class="archive-header-button" 
          :class="{ active: showArchive }"
          @click="toggleArchiveView"
        >
          {{ showArchive ? 'Активные' : 'Архив' }}
        </button>
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск вложений...'"
        />
        
        <button
          class="add-header-button"
          @click="showAddModal = true"
        >
          Создать вложение
        </button>
        <RefreshButton @refresh="refreshData" />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - список вложений -->
      <div
        class="table-section"
        :class="{'with-details': selectedAttachment}"
      >
        <div class="table-container">
          <div class="table-header">
            <div
              class="header-col id-col"
              @click="sortBy('id')"
            >
              <p :class="{ 'active-sort': sortField === 'id' }">
                ID
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'id',
                  'desc': sortField === 'id' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div
              class="header-col name-col"
              @click="sortBy('display_name')"
            >
              <p :class="{ 'active-sort': sortField === 'display_name' }">
                Наименование
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'display_name',
                  'desc': sortField === 'display_name' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div
              class="header-col type-col"
              @click="sortBy('attachment_type')"
            >
              <p :class="{ 'active-sort': sortField === 'attachment_type' }">
                Тип
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'attachment_type',
                  'desc': sortField === 'attachment_type' && sortDirection === 'desc'
                }" 
              >
            </div>
          </div>

          <div class="table-body">
            <div 
              v-for="attachment in sortedAttachments" 
              :key="attachment.id" 
              class="table-row"
              :class="{
                'selected': selectedAttachment && selectedAttachment.id === attachment.id,
                'inactive': !attachment.is_active
              }"
              @click="selectAttachment(attachment)"
            >
              <div class="table-col id-col">
                <span class="cell-content id-value">{{ attachment.id }}</span>
              </div>
              <div class="table-col name-col">
                <span
                  class="truncate-text"
                  :title="attachment.display_name"
                >
                  {{ attachment.display_name }}
                  <span
                    v-if="!attachment.is_active"
                    class="inactive-badge"
                  >(архив)</span>
                </span>
              </div>
              <div class="table-col type-col">
                <span
                  class="type-badge"
                  :class="attachment.attachment_type"
                >
                  {{ getAttachmentTypeLabel(attachment.attachment_type) }}
                </span>
              </div>
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">
              {{ showArchive ? 'В архиве' : 'Всего активных' }}: {{ filteredAttachments.length }}
            </span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали вложения -->
      <div
        v-if="selectedAttachment"
        class="details-section"
      >
        <div class="details-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <div class="attachment-info-title">
                <h3 class="details-title">
                  {{ selectedAttachment.display_name }}
                </h3>
                <span
                  class="attachment-type-badge"
                  :class="selectedAttachment.attachment_type"
                >
                  {{ getAttachmentTypeLabel(selectedAttachment.attachment_type) }}
                </span>
                <span
                  v-if="!selectedAttachment.is_active"
                  class="archive-badge"
                >В архиве</span>
              </div>
              <div class="attachment-info-row">
                <span class="system-name">{{ selectedAttachment.name }}</span>
              </div>
            </div>
            <div class="details-header-actions">
              <button 
                v-if="!selectedAttachment.is_active"
                class="action-btn restore-btn"
                @click="restoreAttachment(selectedAttachment)"
              >
                Восстановить
              </button>
              <button 
                v-else
                class="delete-icon-btn"
                @click="confirmDeleteAttachment(selectedAttachment)"
              >
                <img
                  src="@/assets/icons/delete.png"
                  class="delete-icon"
                >
              </button>
            </div>
          </div>
          
          <div class="details-body">
            <div class="compact-form">
              <div class="form-row">
                <div class="form-group compact">
                  <label class="detail-label">Наименование вложения:</label>
                  <input 
                    v-model="selectedAttachment.display_name" 
                    class="form-input-sm"
                    :disabled="!selectedAttachment.is_active"
                    placeholder="Название вложения"
                    autocomplete="off"
                    @change="updateAttachmentDisplayName"
                  >
                </div>
                <div class="form-group compact">
                  <label class="detail-label">Системное имя:</label>
                  <input 
                    v-model="selectedAttachment.name" 
                    class="form-input-sm"
                    :disabled="!selectedAttachment.is_active"
                    placeholder="avtozayavka"
                    autocomplete="off"
                    @change="updateAttachmentName"
                  >
                  <span class="form-hint">Латинские буквы, цифры и подчеркивания</span>
                </div>
              </div>

              <div class="form-row">
                <div class="form-group compact">
                  <label class="detail-label">Заголовок:</label>
                  <input 
                    v-model="selectedAttachment.title" 
                    class="form-input-sm"
                    :disabled="!selectedAttachment.is_active"
                    placeholder="АВТОЗАЯВКИ"
                    autocomplete="off"
                    @change="updateAttachmentTitle"
                  >
                  <span class="form-hint">Отображается в заголовке категории (всегда в верхнем регистре)</span>
                </div>
                
                <div class="form-group compact">
                  <label class="detail-label">Тип вложения:</label>
                  <div class="custom-select">
                    <div 
                      class="select-header" 
                      :class="{ 'disabled': !selectedAttachment.is_active }"
                      @click="selectedAttachment.is_active && toggleAttachmentTypeDropdown()"
                    >
                      <span class="select-value">{{ getAttachmentTypeLabel(selectedAttachment.attachment_type) }}</span>
                      <img 
                        v-if="selectedAttachment.is_active"
                        src="@/assets/icons/arrow.png" 
                        class="select-arrow" 
                        :class="{ rotated: attachmentTypeDropdownOpen }" 
                      >
                    </div>
                    <transition name="dropdown-fade">
                      <div
                        v-if="attachmentTypeDropdownOpen"
                        class="select-dropdown"
                      >
                        <div 
                          class="select-option"
                          :class="{ active: selectedAttachment.attachment_type === 'cars' }"
                          @click="selectAttachmentType('cars')"
                        >
                          Машины
                        </div>
                        <div 
                          class="select-option"
                          :class="{ active: selectedAttachment.attachment_type === 'people' }"
                          @click="selectAttachmentType('people')"
                        >
                          Люди
                        </div>
                        <div 
                          class="select-option"
                          :class="{ active: selectedAttachment.attachment_type === 'items' }"
                          @click="selectAttachmentType('items')"
                        >
                          ТМЦ
                        </div>
                      </div>
                    </transition>
                  </div>
                </div>
              </div>

              <div class="instruction-section">
                <div class="section-header-with-actions">
                  <label class="detail-label">Инструкция к вложению:</label>
                  <div
                    v-if="instructionHasChanges && selectedAttachment.is_active"
                    class="editor-actions"
                  >
                    <button
                      class="compact-btn cancel-btn"
                      @click="cancelInstructionEdit"
                    >
                      Отмена
                    </button>
                    <button
                      class="compact-btn save-btn"
                      @click="saveInstruction"
                    >
                      Сохранить
                    </button>
                  </div>
                </div>
                <TextConstructor
                  ref="instructionConstructor"
                  v-model="selectedAttachment.instruction"
                  :disabled="!selectedAttachment.is_active"
                  placeholder="Введите инструкцию для вложения..."
                  rows="8"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <div
        v-else
        class="no-selection-message"
      >
        <p>{{ showArchive ? 'Выберите архивное вложение' : 'Выберите вложение для просмотра и редактирования' }}</p>
      </div>
    </div>

    <div
      v-if="filteredAttachments.length === 0"
      class="no-results"
    >
      <div class="no-results-icon">
        {{ showArchive ? '🗄️' : '📄' }}
      </div>
      <p>{{ showArchive ? 'Архив пуст' : 'Вложения не найдены' }}</p>
    </div>

    <!-- Модальное окно создания вложения -->
    <Teleport to="body">
      <div
        v-if="showAddModal"
        class="modal-overlay"
        @click.self="closeAddModal"
      >
        <div class="modal-content horizontal-modal">
          <div class="modal-header">
            <h3>Создать новое вложение</h3>
            <button
              class="modal-close"
              @click="closeAddModal"
            >
              ×
            </button>
          </div>
        
          <div class="modal-body-horizontal">
            <!-- Левая часть - основная информация -->
            <div class="modal-main-info">
              <div class="main-fields">
                <div class="form-group-compact">
                  <label class="form-label-compact">Наименование вложения *</label>
                  <input
                    v-model="newAttachment.display_name"
                    placeholder="Автозаявка"
                    class="input-compact"
                    :class="{ 'has-duplicate': duplicateCheck.display_name }"
                    @input="checkExistingAttachments"
                  >
                  <div
                    v-if="duplicateCheck.display_name"
                    class="duplicate-alert"
                  >
                    <p>Найдено похожее вложение:</p>
                    <div
                      class="duplicate-item"
                      @click="restoreDuplicate(duplicateCheck.display_name)"
                    >
                      <span>{{ duplicateCheck.display_name.display_name }}</span>
                      <span class="duplicate-status">(в архиве)</span>
                    </div>
                  </div>
                </div>

                <div class="form-group-compact">
                  <label class="form-label-compact">Тип вложения *</label>
                  <div class="custom-select">
                    <div
                      class="select-header"
                      @click="toggleNewAttachmentTypeDropdown"
                    >
                      <span class="select-value">{{ getAttachmentTypeLabel(newAttachment.attachment_type) }}</span>
                      <img
                        src="@/assets/icons/arrow.png"
                        class="select-arrow"
                        :class="{ rotated: newAttachmentTypeDropdownOpen }"
                      >
                    </div>
                    <transition name="dropdown-fade">
                      <div
                        v-if="newAttachmentTypeDropdownOpen"
                        class="select-dropdown"
                      >
                        <div 
                          class="select-option"
                          :class="{ active: newAttachment.attachment_type === 'cars' }"
                          @click="selectNewAttachmentType('cars')"
                        >
                          Машины
                        </div>
                        <div 
                          class="select-option"
                          :class="{ active: newAttachment.attachment_type === 'people' }"
                          @click="selectNewAttachmentType('people')"
                        >
                          Люди
                        </div>
                        <div 
                          class="select-option"
                          :class="{ active: newAttachment.attachment_type === 'items' }"
                          @click="selectNewAttachmentType('items')"
                        >
                          ТМЦ
                        </div>
                      </div>
                    </transition>
                  </div>
                </div>
              
                <div class="form-group-compact">
                  <label class="form-label-compact">Системное имя *</label>
                  <input
                    v-model="newAttachment.name"
                    placeholder="avtozayavka"
                    class="input-compact"
                    :class="{ 'has-duplicate': duplicateCheck.name, 'has-error': nameError }"
                    @input="validateSystemName"
                    @blur="checkExistingAttachments"
                  >
                  <span class="form-hint">Латинские буквы, цифры и подчеркивания</span>
                  <span
                    v-if="nameError"
                    class="form-error"
                  >{{ nameError }}</span>
                  <div
                    v-if="duplicateCheck.name"
                    class="duplicate-alert"
                  >
                    <p>Найдено вложение с таким системным именем:</p>
                    <div
                      class="duplicate-item"
                      @click="restoreDuplicate(duplicateCheck.name)"
                    >
                      <span>{{ duplicateCheck.name.display_name }}</span>
                      <span class="duplicate-status">(в архиве)</span>
                    </div>
                  </div>
                </div>

                <div class="form-group-compact">
                  <label class="form-label-compact">Заголовок *</label>
                  <input
                    v-model="newAttachment.title"
                    placeholder="АВТОЗАЯВКИ"
                    class="input-compact"
                    :class="{ 'has-duplicate': duplicateCheck.title }"
                    @input="checkExistingAttachments"
                  >
                  <span class="form-hint">Отображается в заголовке категории</span>
                  <div
                    v-if="duplicateCheck.title"
                    class="duplicate-alert"
                  >
                    <p>Найдено вложение с таким заголовком:</p>
                    <div
                      class="duplicate-item"
                      @click="restoreDuplicate(duplicateCheck.title)"
                    >
                      <span>{{ duplicateCheck.title.display_name }}</span>
                      <span class="duplicate-status">(в архиве)</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Правая часть - инструкция -->
            <div class="modal-cells-section">
              <div class="cells-header-compact">
                <h4 class="cells-title-compact">
                  Инструкция к вложению
                </h4>
              </div>
            
              <div class="cells-scroll-container">
                <div class="settings-grid">
                  <div class="setting-item">
                    <TextConstructor
                      v-model="newAttachment.instruction"
                      placeholder="Введите инструкцию для вложения..."
                      rows="12"
                    />
                    <span class="setting-hint">
                      Инструкция будет отображаться при выборе данного вложения
                    </span>
                  </div>

                  <div class="setting-item">
                    <h5 class="fields-preview-title">
                      Предварительный просмотр:
                    </h5>
                    <div class="preview-card">
                      <div class="preview-header">
                        <div class="preview-title">
                          {{ newAttachment.title || 'ЗАГОЛОВОК' }}
                        </div>
                      </div>
                      <div class="preview-attachment">
                        <span class="preview-attachment-name">
                          {{ newAttachment.display_name || 'Название вложения' }}
                        </span>
                      </div>
                      <button class="preview-add-btn">
                        Добавить
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        
          <div class="modal-footer">
            <button
              class="modal-cancel"
              @click="closeAddModal"
            >
              Отмена
            </button>
            <button
              class="modal-confirm"
              @click="createAttachment"
            >
              Создать
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Уведомления -->
    <div
      v-if="notification.show"
      class="notification"
      :class="notification.type"
    >
      <span class="notification-message">{{ notification.message }}</span>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';
import TextConstructor from './TextConstructor.vue';

export default {
  name: 'AttachmentsManagement',
  components: {
    SearchComponent,
    RefreshButton,
    TextConstructor
  },
  data() {
    return {
      searchQuery: '',
      newAttachment: {
        name: '',
        display_name: '',
        title: '',
        attachment_type: 'cars',
        instruction: '',
        is_active: true
      },
      attachments: [], // Все вложения (активные и архивные)
      showAddModal: false,
      selectedAttachment: null,
      sortField: null,
      sortDirection: 'asc',
      nameError: '',
      originalInstruction: '',
      attachmentTypeDropdownOpen: false,
      newAttachmentTypeDropdownOpen: false,
      notification: {
        show: false,
        message: '',
        type: 'info'
      },
      showArchive: false,
      duplicateCheck: {
        display_name: null,
        name: null,
        title: null
      }
    };
  },
  computed: {
    filteredAttachments() {
      let filtered = this.attachments;
      
      // Фильтр по активности
      if (this.showArchive) {
        filtered = filtered.filter(attachment => !attachment.is_active);
      } else {
        filtered = filtered.filter(attachment => attachment.is_active);
      }
      
      // Поиск
      if (!this.searchQuery) return filtered;
      
      const query = this.searchQuery.toLowerCase();
      return filtered.filter(attachment => 
        attachment.display_name?.toLowerCase().includes(query) || 
        attachment.name?.toLowerCase().includes(query) ||
        attachment.title?.toLowerCase().includes(query) ||
        attachment.id?.toString().includes(query)
      );
    },
    sortedAttachments() {
      const attachments = [...this.filteredAttachments];
      
      if (!this.sortField) {
        return attachments.sort((a, b) => a.display_name.localeCompare(b.display_name));
      }
      
      return attachments.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'id':
            valueA = a.id;
            valueB = b.id;
            break;
          case 'display_name':
            valueA = a.display_name;
            valueB = b.display_name;
            break;
          case 'attachment_type':
            valueA = a.attachment_type;
            valueB = b.attachment_type;
            break;
          default:
            return 0;
        }
        
        if (valueA < valueB) {
          return this.sortDirection === 'asc' ? -1 : 1;
        }
        if (valueA > valueB) {
          return this.sortDirection === 'asc' ? 1 : -1;
        }
        return 0;
      });
    },
    instructionHasChanges() {
      return this.selectedAttachment && this.selectedAttachment.instruction !== this.originalInstruction;
    },
    allInactiveAttachments() {
      return this.attachments.filter(attachment => !attachment.is_active);
    }
  },
  mounted() {
    this.refreshData();
    document.addEventListener('click', (e) => {
      if (!this.$el.contains(e.target)) {
        this.attachmentTypeDropdownOpen = false;
        this.newAttachmentTypeDropdownOpen = false;
      }
    });
  },
  methods: {
    validateSystemName() {
      const nameRegex = /^[a-z0-9_]*$/;
      if (!nameRegex.test(this.newAttachment.name)) {
        this.nameError = "Только латинские буквы, цифры и подчеркивания";
      } else {
        this.nameError = '';
      }
    },
    
    async checkExistingAttachments() {
      // Проверяем наличие совпадений только среди неактивных вложений
      const inactiveAttachments = this.allInactiveAttachments;
      
      // Сброс предыдущих проверок
      this.duplicateCheck = {
        display_name: null,
        name: null,
        title: null
      };
      
      if (this.newAttachment.display_name) {
        const duplicate = inactiveAttachments.find(a => 
          a.display_name.toLowerCase() === this.newAttachment.display_name.toLowerCase()
        );
        if (duplicate) {
          this.duplicateCheck.display_name = duplicate;
        }
      }
      
      if (this.newAttachment.name) {
        const duplicate = inactiveAttachments.find(a => 
          a.name.toLowerCase() === this.newAttachment.name.toLowerCase()
        );
        if (duplicate) {
          this.duplicateCheck.name = duplicate;
        }
      }
      
      if (this.newAttachment.title) {
        const duplicate = inactiveAttachments.find(a => 
          a.title.toUpperCase() === this.newAttachment.title.toUpperCase()
        );
        if (duplicate) {
          this.duplicateCheck.title = duplicate;
        }
      }
    },
    
    async restoreDuplicate(attachment) {
      if (confirm(`Восстановить архивное вложение "${attachment.display_name}"?`)) {
        await this.restoreAttachment(attachment);
        this.duplicateCheck = {
          display_name: null,
          name: null,
          title: null
        };
        this.newAttachment = {
          name: '',
          display_name: '',
          title: '',
          attachment_type: 'cars',
          instruction: '',
          is_active: true
        };
        this.showAddModal = false;
      }
    },
    
    async refreshData() {
      await this.fetchAllAttachments();
    },
    
    async fetchAllAttachments() {
      try {
        const response = await apiRequest("/attachments/all", {
        });
        if (response.ok) {
          const data = await response.json();
          this.attachments = data;
        }
      } catch (error) {
        console.error("Error fetching all attachments:", error);
        this.showNotification("Ошибка при загрузке вложений", "error");
      }
    },
    
    toggleArchiveView() {
      this.showArchive = !this.showArchive;
      this.selectedAttachment = null;
    },
    
    async createAttachment() {
      // Проверяем, есть ли совпадения среди неактивных вложений
      const hasDuplicates = Object.values(this.duplicateCheck).some(item => item !== null);
      
      if (hasDuplicates) {
        const duplicateNames = Object.values(this.duplicateCheck)
          .filter(item => item !== null)
          .map(item => item.display_name)
          .join(', ');
        
        if (!confirm(`Найдены архивные вложения: ${duplicateNames}. Восстановить их вместо создания нового?`)) {
          // Пользователь отказался восстанавливать, продолжаем создание
        } else {
          // Восстанавливаем все найденные дубликаты
          for (const duplicate of Object.values(this.duplicateCheck)) {
            if (duplicate) {
              await this.restoreAttachment(duplicate);
            }
          }
          this.closeAddModal();
          return;
        }
      }
      
      // Стандартная валидация
      if (!this.newAttachment.name.trim() || !this.newAttachment.display_name.trim() || !this.newAttachment.title.trim()) {
        this.showNotification("Заполните все обязательные поля", "warning");
        return;
      }
      
      const nameRegex = /^[a-z0-9_]+$/;
      if (!nameRegex.test(this.newAttachment.name)) {
        this.showNotification("Системное имя может содержать только латинские буквы, цифры и подчеркивания", "warning");
        return;
      }
      
      // Проверка существования среди активных вложений
      const activeAttachments = this.attachments.filter(a => a.is_active);
      const existingName = activeAttachments.find(a => a.name === this.newAttachment.name);
      if (existingName) {
        this.showNotification("Вложение с таким системным именем уже существует", "warning");
        return;
      }
      
      this.newAttachment.title = this.newAttachment.title.toUpperCase();
      
      try {
        const response = await apiRequest("/attachments", {
          method: "POST",
          body: JSON.stringify(this.newAttachment),
        });
        
        if (response.ok) {
          this.newAttachment = {
            name: '',
            display_name: '',
            title: '',
            attachment_type: 'cars',
            instruction: '',
            is_active: true
          };
          this.duplicateCheck = {
            display_name: null,
            name: null,
            title: null
          };
          this.showAddModal = false;
          await this.refreshData();
          this.showNotification("Вложение успешно создано", "success");
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при создании вложения", "error");
        }
      } catch (error) {
        console.error("Error creating attachment:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    
    async updateAttachment(attachment, field = null) {
      if (!attachment.is_active) {
        this.showNotification("Невозможно редактировать архивное вложение", "warning");
        return;
      }
      
      try {
        const response = await apiRequest(`/attachments/${attachment.id}`, {
          method: "PUT",
          body: JSON.stringify(attachment),
        });
        
        if (response.ok) {
          let message = "Вложение успешно обновлено";
          if (field === 'display_name') {
            message = "Наименование успешно изменено";
          } else if (field === 'name') {
            message = "Системное имя успешно изменено";
          } else if (field === 'title') {
            message = "Заголовок успешно изменен";
          } else if (field === 'attachment_type') {
            message = "Тип вложения успешно изменен";
          } else if (field === 'instruction') {
            message = "Инструкция успешно изменена";
          }
          
          this.showNotification(message, "success");
          await this.refreshData();
          this.originalInstruction = attachment.instruction || '';
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при обновлении вложения", "error");
        }
      } catch (error) {
        console.error("Error updating attachment:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    
    async updateAttachmentDisplayName() {
      if (this.selectedAttachment && this.selectedAttachment.is_active) {
        await this.updateAttachment(this.selectedAttachment, 'display_name');
      }
    },
    
    async updateAttachmentName() {
      if (this.selectedAttachment && this.selectedAttachment.is_active) {
        const nameRegex = /^[a-z0-9_]+$/;
        if (!nameRegex.test(this.selectedAttachment.name)) {
          this.showNotification("Системное имя может содержать только латинские буквы, цифры и подчеркивания", "warning");
          return;
        }
        await this.updateAttachment(this.selectedAttachment, 'name');
      }
    },
    
    async updateAttachmentTitle() {
      if (this.selectedAttachment && this.selectedAttachment.is_active) {
        this.selectedAttachment.title = this.selectedAttachment.title.toUpperCase();
        await this.updateAttachment(this.selectedAttachment, 'title');
      }
    },
    
    async confirmDeleteAttachment(attachment) {
      if (!attachment.is_active) {
        this.showNotification("Вложение уже находится в архиве", "warning");
        return;
      }
      
      if (!confirm(`Переместить вложение "${attachment.display_name}" в архив?`)) return;
      
      try {
        const response = await apiRequest(`/attachments/${attachment.id}`, {
          method: "DELETE",
        });
        
        if (response.ok) {
          this.selectedAttachment = null;
          await this.refreshData();
          this.showNotification("Вложение перемещено в архив", "success");
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при архивировании вложения", "error");
        }
      } catch (error) {
        console.error("Error archiving attachment:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    
    async restoreAttachment(attachment) {
      try {
        const response = await apiRequest(`/attachments/${attachment.id}/restore`, {
          method: "PUT",
        });
        
        if (response.ok) {
          await this.refreshData();
          
          // Если восстанавливаем выбранное вложение, обновляем его
          if (this.selectedAttachment && this.selectedAttachment.id === attachment.id) {
            this.selectedAttachment.is_active = true;
          }
          
          this.showNotification("Вложение успешно восстановлено", "success");
          
          // Если находимся в архиве и восстанавливаем, переключаемся на активные
          if (this.showArchive) {
            this.showArchive = false;
          }
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при восстановлении вложения", "error");
        }
      } catch (error) {
        console.error("Error restoring attachment:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    
    selectAttachment(attachment) {
      this.selectedAttachment = JSON.parse(JSON.stringify(attachment));
      this.originalInstruction = attachment.instruction || '';
    },
    
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'asc';
      }
    },
    
    getAttachmentTypeLabel(type) {
      switch(type) {
        case 'cars': return 'Машины';
        case 'people': return 'Люди';
        case 'items': return 'ТМЦ';
        default: return type;
      }
    },
    
    saveInstruction() {
      if (this.selectedAttachment && this.selectedAttachment.is_active) {
        this.updateAttachment(this.selectedAttachment, 'instruction');
      }
    },
    
    cancelInstructionEdit() {
      if (this.selectedAttachment) {
        this.selectedAttachment.instruction = this.originalInstruction;
      }
    },
    
    toggleAttachmentTypeDropdown() {
      if (this.selectedAttachment && this.selectedAttachment.is_active) {
        this.attachmentTypeDropdownOpen = !this.attachmentTypeDropdownOpen;
      }
    },
    
    selectAttachmentType(type) {
      if (this.selectedAttachment && this.selectedAttachment.is_active) {
        this.selectedAttachment.attachment_type = type;
        this.attachmentTypeDropdownOpen = false;
        this.updateAttachment(this.selectedAttachment, 'attachment_type');
      }
    },
    
    toggleNewAttachmentTypeDropdown() {
      this.newAttachmentTypeDropdownOpen = !this.newAttachmentTypeDropdownOpen;
    },
    
    selectNewAttachmentType(type) {
      this.newAttachment.attachment_type = type;
      this.newAttachmentTypeDropdownOpen = false;
    },
    
    closeAddModal() {
      this.showAddModal = false;
      this.newAttachment = {
        name: '',
        display_name: '',
        title: '',
        attachment_type: 'cars',
        instruction: '',
        is_active: true
      };
      this.duplicateCheck = {
        display_name: null,
        name: null,
        title: null
      };
      this.nameError = '';
    },
    
    showNotification(message, type = 'info') {
      this.notification = {
        show: true,
        message,
        type
      };
      
      setTimeout(() => {
        this.hideNotification();
      }, 3000);
    },
    
    hideNotification() {
      this.notification.show = false;
    }
  },
};
</script>

<style scoped>
.attachments-management-container {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  height: 500px;
  position: relative;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  height: 50px;
}

.management-title {
  font-size: 1.2em;
  margin: 0;
  font-weight: 600;
  color: #000;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.add-header-button {
  padding: 8px 16px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 50px;
  cursor: pointer;
  font-size: 0.9em;
  transition: background-color 0.2s ease;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.add-header-button:hover {
  background: #3a45b2;
}

.archive-header-button {
  padding: 2px 16px;
  background: #f8f9fa;
  color: #666;
  border: 1px solid #e6e6e6;
  border-radius: 50px;
  cursor: pointer;
  font-size: 0.7em;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.archive-header-button:hover {
  background: #e9ecef;
  border-color: #ccc;
}

.archive-header-button.active {
  background: #6b7280;
  color: white;
  border-color: #6b7280;
}

.content-container {
  display: flex;
  height: 450px;
  width: 100%;
}

.table-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e6e6e6;
}

.table-section.with-details {
  width: 40%;
}

.table-container {
  background: #fff;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
  height: 43px;
  align-items: center;
}

.delete-icon-btn {
  outline: none;
  border: none;
  width: 30px;
  height: 30px;
  padding: 5px;
  border-radius: 10px;
  display: flex;
  align-items:center;
  justify-content: center;
  transition: .2s;
  background: #f8f9fa;
}

.delete-icon {
  width: 20px;
  height: 20px;
}

.delete-icon-btn:hover {
  background-color: #e6e6e6;
  cursor:pointer;
}

.action-btn {
  padding: 8px 16px;
  border: none;
  border-radius: 10px;
  cursor: pointer;
  font-size: 0.85em;
  font-weight: 500;
  transition: all 0.2s ease;
}

.restore-btn {
  background: #10b981;
  color: white;
}

.restore-btn:hover {
  background: #0da271;
}

.header-col {
  padding: 0 8px;
  font-size: 14px;
  color: #a2a2a2;
  font-weight: 600;
  text-align: left;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: .2s;
  cursor: pointer;
  user-select: none;
}

.header-col:hover {
  color: #000;
}

.header-col:hover .sort-icon {
  filter: brightness(0);
}

.sort-icon {
  width: 12px;
  height: 12px;
  transition: .2s;
}

.sort-icon.sorted {
  filter: brightness(0);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: #000 !important;
  font-weight: 600 !important;
}

.id-col {
  width: 15%;
  min-width: 60px;
}

.name-col {
  width: 55%;
  min-width: 200px;
}

.type-col {
  width: 30%;
  min-width: 100px;
}

.table-body {
  flex: 1;
  overflow-y: auto;
  max-height: 407px;
}

.table-row {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #f0f0f0;
  align-items: center;
  transition: background-color 0.2s ease;
  cursor: pointer;
  height: 42px;
  font-size: 14px;
}

.table-row:hover {
  background-color: #fafafa;
}

.table-row.selected {
  background-color: #f8f9ff;
}

.table-row.inactive {
  opacity: 0.7;
  background-color: #f9f9f9;
}

.table-row.inactive:hover {
  background-color: #f0f0f0;
}

.table-row:last-child {
  border-bottom: none;
}

.table-col {
  padding: 0 8px;
}

.cell-content {
  display: block;
  padding: 4px 0;
}

.id-value {
  font-weight: 600;
  color: #000;
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.inactive-badge {
  font-size: 0.75em;
  color: #666;
  background: #e9ecef;
  padding: 2px 6px;
  border-radius: 4px;
  margin-left: 5px;
}

.type-badge {
  padding: 4px 8px;
  border-radius: 12px;
  font-size: 0.8em;
  font-weight: 600;
}

.type-badge.cars {
  background: linear-gradient(135deg, #f0f4ff 0%, #f0f4ff 100%);
  color: #3a4a6e;
  border: 1px solid #d0d9f0;
}

.type-badge.people {
  background: linear-gradient(135deg, #f0ecff 0%, #f0ecff 100%);
  color: #6d5aa7;
  border: 1px solid #c6b8f0;
}

.type-badge.items {
  background: linear-gradient(135deg, #f0fff4 0%, #f0fff4 100%);
  color: #2e7d32;
  border: 1px solid #c8e6c9;
}

.table-footer {
  padding: 6px 20px;
  border-top: 1px solid #e6e6e6;
  text-align: end;
  background: #f8fafc;
}

.items-count {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

.details-section {
  width: 60%;
  padding: 15px;
  overflow-y: auto;
  background: #fafafa;
}

.details-content {
  height: 100%;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 15px;
}

.details-title-wrapper {
  display: flex;
  flex-direction: column;
  gap: 0px;
}

.details-title {
  margin: 0;
  color: #000;
  font-size: 1.2em;
  font-weight: 600;
}

.attachment-info-title {
  display: flex;
  gap: 10px;
  align-items:center;
  flex-wrap: wrap;
}

.attachment-info-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 5px;
}

.system-name {
  font-size: 0.85em;
  color: #666;
  background: #f5f5f5;
  padding: 4px 8px;
  border-radius: 6px;
}

.attachment-type-badge {
  padding: 3px 12px;
  border-radius: 16px;
  font-size: 0.8em;
  font-weight: 500;
}

.attachment-type-badge.cars {
  background: linear-gradient(135deg, #f0f4ff 0%, #f0f4ff 100%);
  color: #3a4a6e;
  border: 1px solid #d0d9f0;
}

.attachment-type-badge.people {
  background: linear-gradient(135deg, #f0ecff 0%, #f0ecff 100%);
  color: #6d5aa7;
  border: 1px solid #c6b8f0;
}

.attachment-type-badge.items {
  background: linear-gradient(135deg, #f0fff4 0%, #f0fff4 100%);
  color: #2e7d32;
  border: 1px solid #c8e6c9;
}

.archive-badge {
  padding: 3px 8px;
  border-radius: 12px;
  font-size: 0.75em;
  font-weight: 500;
  background: #e9ecef;
  color: #666;
}

.details-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.compact-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.form-row {
  display: flex;
  gap: 16px;
}

.form-group.compact {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.detail-label {
  font-size: 0.85em;
  color: #a2a2a2;
  font-weight:400;
}

.form-input-sm {
  padding:5px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  font-size: 0.8em;
  height: 35px;
  transition: border-color 0.2s ease;
  background: #fff;
  width: 100%;
}

.form-input-sm:focus {
  border-color: #4F5BDF;
  outline: none;
}

.form-input-sm:disabled {
  background: #f8f9fa;
  color: #666;
  cursor: not-allowed;
}

.form-input-sm.has-duplicate {
  border-color: #f59e0b;
}

.form-input-sm.has-error {
  border-color: #ef4444;
}

.form-hint {
  font-size: 0.7em;
  color: #999;
  margin-top: 2px;
}

.duplicate-alert {
  margin-top: 5px;
  padding: 8px;
  background: #fffbeb;
  border: 1px solid #fde68a;
  border-radius: 6px;
  font-size: 0.8em;
}

.duplicate-alert p {
  margin: 0 0 5px 0;
  color: #92400e;
  font-weight: 500;
}

.duplicate-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 8px;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.duplicate-item:hover {
  background: #f0f0f0;
}

.duplicate-status {
  font-size: 0.75em;
  color: #666;
  font-style: italic;
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
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  background: white;
  cursor: pointer;
  font-size: 0.8em;
  height: 35px;
  transition: all 0.2s ease;
}

.select-header:hover {
  border-color: #ccc;
  background: #f8f9fa;
}

.select-header.disabled {
  background: #f8f9fa;
  cursor: not-allowed;
  opacity: 0.7;
}

.select-value {
  color: #000;
}

.select-arrow {
  width: 10px;
  height: 10px;
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
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  z-index: 1000;
  margin-top: 4px;
  overflow: hidden;
}

.select-option {
  padding: 6px 12px;
  font-size: 0.8em;
  cursor: pointer;
  transition: background-color 0.2s ease;
  color: #000;
  border-bottom: 1px solid #f0f0f0;
  height: 32px;
  display: flex;
  align-items: center;
}

.select-option:last-child {
  border-bottom: none;
}

.select-option:hover {
  background: #f0f0f0;
}

.select-option.active {
  background: #4F5BDF;
  color: white;
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

.instruction-section {
  background: #f8f9ff;
  margin-bottom: 10px;
}

.section-header-with-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  height: 23px;
}

.editor-actions {
  display: flex;
  gap: 5px;
}

.compact-btn {
  padding: 6px 12px;
  border: none;
  border-radius: 10px;
  cursor: pointer;
  font-size: 0.6em;
  font-weight: 500;
  transition: all 0.2s ease;
}

.save-btn {
  background: #4F5BDF;
  color: white;
}

.save-btn:hover {
  background: #3a45b2;
}

.cancel-btn {
  background: #6b7280;
  color: white;
}

.cancel-btn:hover {
  background: #4b5563;
}

.no-selection-message {
  width: 60%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-weight: 400;
  font-size: 14px;
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: #a2a2a2;
  width: 100%;
}

.no-results-icon {
  font-size: 3em;
  margin-bottom: 16px;
  opacity: 0.5;
}

.no-results p {
  margin: 0;
  font-size: 1.1em;
}

/* Модальные окна */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.horizontal-modal {
  width: 1050px;
  height: 400px;
  max-width: 1050px;
  display: flex;
  flex-direction: column;
  background: #fff;
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
  flex-shrink: 0;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: #000;
}

.modal-close {
  background: none;
  border: none;
  font-size: 20px;
  cursor: pointer;
  color: #999;
  padding: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.2s;
  border-radius: 50%;
}

.modal-close:hover {
  background: #f5f5f5;
  color: #000;
}

.modal-body-horizontal {
  display: flex;
  flex: 1;
  overflow: hidden;
  padding: 0;
}

.modal-main-info {
  width: 30%;
  padding: 16px;
  border-right: 1px solid #e6e6e6;
  background: #fafafa;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}

.main-fields {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-group-compact {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label-compact {
  font-size: 0.8em;
  color: #000;
  font-weight: 500;
}

.input-compact {
  padding: 8px 10px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  font-size: 0.85em;
  background: #fff;
  transition: border-color 0.2s;
  height: 32px;
}

.input-compact:focus {
  border-color: #4F5BDF;
  outline: none;
}

.input-compact.has-duplicate {
  border-color: #f59e0b;
}

.input-compact.has-error {
  border-color: #ef4444;
}

.form-error {
  font-size: 0.7em;
  color: #ef4444;
  margin-top: 4px;
}

.modal-cells-section {
  width: 70%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.cells-header-compact {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
  flex-shrink: 0;
}

.cells-title-compact {
  margin: 0;
  font-size: 1em;
  font-weight: 600;
  color: #000;
}

.cells-scroll-container {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.settings-grid {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.setting-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.setting-hint {
  font-size: 0.75em;
  color: #666;
  line-height: 1.4;
  margin-top: 4px;
}

.fields-preview-title {
  margin: 0;
  font-size: 0.9em;
  font-weight: 600;
  color: #000;
}

.preview-card {
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  padding: 12px;
  background: #f8f9fa;
  max-width: 200px;
}

.preview-header {
  margin-bottom: 8px;
}

.preview-title {
  font-size: 10px;
  font-weight: bold;
  color: #a2a2a2;
  text-transform: uppercase;
}

.preview-attachment {
  width: 125px;
  height: 25px;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 500;
  color: #000;
  display: flex;
  align-items: center;
  padding: 0 8px;
  background: white;
  margin-bottom: 8px;
}

.preview-attachment-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.preview-add-btn {
  width: 85px;
  height: 25px;
  background: rgba(79, 91, 223, 0.4);
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 500;
  color: #fff;
  cursor: pointer;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 12px 16px;
  border-top: 1px solid #e6e6e6;
  background: #fff;
  flex-shrink: 0;
}

.modal-cancel {
  padding: 8px 16px;
  background: #f8f9fa;
  color: #666;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.85em;
  font-weight: 500;
  transition: all 0.2s ease;
}

.modal-cancel:hover {
  background: #e9ecef;
}

.modal-confirm {
  padding: 8px 16px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.85em;
  font-weight: 600;
  transition: background-color 0.2s ease;
}

.modal-confirm:hover {
  background: #3a45b2;
}

/* Стили для уведомлений */
.notification {
  position: fixed;
  top: 0;
  left: 50%;
  transform: translateX(-50%) translateY(-100%);
  padding: 12px 24px;
  border-radius: 0 0 8px 8px;
  color: white;
  font-weight: 500;
  z-index: 10000;
  text-align: center;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  animation: slideDown 0.3s ease-out forwards;
  min-width: 300px;
}

.notification.success {
  background: #10b981;
}

.notification.error {
  background: #ef4444;
}

.notification.warning {
  background: #f59e0b;
}

.notification.info {
  background: #3b82f6;
}

.notification-message {
  font-size: 0.9em;
}

@keyframes slideDown {
  from {
    transform: translateX(-50%) translateY(-100%);
  }
  to {
    transform: translateX(-50%) translateY(0);
  }
}

@media (max-width: 768px) {
  .content-container {
    flex-direction: column;
    height: auto;
  }
  
  .table-section,
  .details-section,
  .no-selection-message {
    width: 100% !important;
  }
  
  .table-section.with-details {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
    height: 255px;
  }
  
  .horizontal-modal {
    height: auto;
    max-height: 80vh;
    width: 95%;
  }
  
  .modal-body-horizontal {
    flex-direction: column;
  }
  
  .modal-main-info,
  .modal-cells-section {
    width: 100%;
  }
  
  .modal-main-info {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
    padding: 12px;
  }
  
  .attachment-info-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .section-header-with-actions {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .editor-actions {
    align-self: flex-end;
  }
  
  .notification {
    left: 20px;
    right: 20px;
    transform: translateY(-100%);
    min-width: auto;
  }
  
  @keyframes slideDown {
    from {
      transform: translateY(-100%);
    }
    to {
      transform: translateY(0);
    }
  }
}
</style>