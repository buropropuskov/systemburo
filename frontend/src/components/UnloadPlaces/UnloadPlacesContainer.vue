<template>
  <div class="unload-places-container dashboard-card">
    <div class="management-header rt-header-inline">
      <h3 class="management-title">
        Управление местами разгрузки
      </h3>
      <div class="header-controls">
        <BaseDropdown
          class="archive-dropdown"
          :model-value="showArchive ? 'archive' : 'active'"
          :options="archiveOptions"
          label-key="label"
          value-key="value"
          @update:model-value="onArchiveModeChange"
        />
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск мест разгрузки...'"
        />
        <button
          class="add-header-button rt-btn-compact"
          aria-label="Добавить"
          @click="showAddModal = true"
        >
          <span
            class="rt-btn-icon"
            aria-hidden="true"
          >+</span>
          <span class="rt-btn-label">Добавить</span>
        </button>
        <RefreshButton
          :loading="refreshing"
          @refresh="refreshData"
        />
      </div>
    </div>

    <div
      v-if="selectedIds.length"
      class="bulk-bar"
      data-testid="unloadplaces-bulk-bar"
    >
      <span class="bulk-count">Выбрано: {{ selectedIds.length }}</span>
      <div class="bulk-actions">
        <button
          v-if="!showArchive"
          class="pill pill-danger"
          data-testid="unloadplaces-bulk-archive"
          @click="startBulkOperation('archive')"
        >
          В архив
        </button>
        <button
          v-else
          class="pill pill-restore"
          data-testid="unloadplaces-bulk-restore"
          @click="startBulkOperation('restore')"
        >
          Восстановить
        </button>
        <button
          class="pill pill-ghost bulk-clear"
          data-testid="unloadplaces-bulk-clear"
          @click="clearSelection"
        >
          Снять выбор
        </button>
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - таблица мест разгрузки -->
      <div
        class="table-section"
        :class="{'with-details': selectedPlace}"
      >
        <div class="table-container rt-table">
          <div class="table-header rt-head-row">
            <div
              class="header-col check-col"
              @click.stop
            >
              <input
                type="checkbox"
                class="bulk-check"
                :checked="allSelected"
                :indeterminate.prop="someSelected"
                aria-label="Выбрать все"
                data-testid="unloadplaces-select-all"
                @change="toggleSelectAll"
              >
            </div>
            <div
              class="header-col id-col"
              @click="sortBy('id')"
            >
              <p :class="{ 'active-sort': sortField === 'id' }">
                ID
              </p>
              <AppIcon
                name="sort"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'id',
                  'desc': sortField === 'id' && sortDirection === 'desc'
                }"
              />
            </div>
            <div
              class="header-col name-col"
              @click="sortBy('name')"
            >
              <p :class="{ 'active-sort': sortField === 'name' }">
                Наименование
              </p>
              <AppIcon
                name="sort"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'name',
                  'desc': sortField === 'name' && sortDirection === 'desc'
                }"
              />
            </div>
            <div class="header-col status-col">
              <p>Статус</p>
            </div>
          </div>

          <div class="table-body">
            <div
              v-for="(place, index) in sortedUnloadPlaces"
              :key="place.id"
              class="table-row rt-row"
              :class="{
                'selected': selectedPlace && selectedPlace.id === place.id,
                'inactive': !place.is_active
              }"
              @click="selectPlace(place)"
            >
              <div
                class="table-col check-col"
                @click.stop
              >
                <input
                  type="checkbox"
                  class="bulk-check"
                  :checked="isSelected(place.id)"
                  :aria-label="`Выбрать ${place.name}`"
                  data-testid="unloadplaces-row-check"
                  @click="onRowCheck(place, index, $event)"
                >
              </div>
              <div
                class="table-col id-col"
                data-label="ID"
              >
                <span class="cell-content id-value">{{ place.id }}</span>
              </div>
              <div
                class="table-col name-col"
                data-label="Наименование"
              >
                <span
                  class="truncate-text"
                  :title="place.name"
                >
                  {{ place.name }}
                  <span
                    v-if="!place.is_active"
                    class="inactive-badge"
                  >(архив)</span>
                </span>
              </div>
              <div
                class="table-col status-col"
                data-label="Статус"
              >
                <span
                  class="status-badge"
                  :class="getStatusClass(place)"
                >
                  {{ getStatusText(place) }}
                </span>
              </div>
            </div>
            <div
              v-if="!sortedUnloadPlaces.length"
              class="no-results"
            >
              {{ emptyText }}
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">{{ showArchive ? 'В архиве' : 'Всего мест разгрузки' }}: {{ filteredUnloadPlaces.length }}</span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали места разгрузки -->
      <div
        v-if="selectedPlace"
        class="details-section"
      >
        <div class="details-tabs">
          <div class="details-tabs__row">
            <button
              class="tab-btn"
              :class="{ 'active': activeTab === 'main' }"
              @click="activeTab = 'main'"
            >
              Основное
            </button>
            <button
              class="tab-btn"
              :class="{ 'active': activeTab === 'schedule' }"
              @click="activeTab = 'schedule'"
            >
              Расписание
            </button>
            <button
              class="tab-btn"
              :class="{ 'active': activeTab === 'warnings' }"
              @click="activeTab = 'warnings'"
            >
              Предупреждения
            </button>
            <button
              class="tab-btn"
              :class="{ 'active': activeTab === 'route' }"
              @click="activeTab = 'route'"
            >
              Местоположение и маршрут
            </button>
          </div>
        </div>

        <!-- Вкладка Основное -->
        <div
          v-if="activeTab === 'main'"
          class="tab-content"
        >
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">
                {{ selectedPlace.name }}
              </h3>
              <span
                class="current-status-badge"
                :class="getCurrentStatusClass(selectedPlace)"
              >
                {{ getCurrentStatusText(selectedPlace) }}
              </span>
            </div>
            <div class="details-header-actions">
              <span
                v-if="!selectedPlace.is_active"
                class="archive-badge"
              >В архиве</span>
              <button
                class="action-btn history-btn"
                @click="openHistory(selectedPlace)"
              >
                История
              </button>
              <button
                v-if="selectedPlace.is_active"
                class="action-btn archive-action-btn"
                @click="confirmDeletePlace(selectedPlace)"
              >
                В архив
              </button>
              <button
                v-else
                class="action-btn restore-btn"
                @click="onRestore(selectedPlace)"
              >
                Восстановить
              </button>
            </div>
          </div>
          
          <div class="details-body">
            <div class="detail-group">
              <label class="detail-label">Наименование:</label>
              <input
                v-model="selectedPlace.name"
                class="form-input"
                placeholder="Введите название места"
                autocomplete="off"
                :disabled="isArchivedView"
                @change="updatePlace(selectedPlace)"
              >
            </div>

            <div class="detail-group">
              <label class="detail-label">Описание:</label>
              <textarea
                v-model="selectedPlace.description"
                class="form-textarea"
                placeholder="Введите описание"
                rows="2"
                :disabled="isArchivedView"
                @change="updatePlace(selectedPlace)"
              />
            </div>

            <!-- Статус в виде кнопок -->
            <div class="detail-group">
              <label class="detail-label">Статус:</label>
              <div class="status-toggle">
                <button
                  class="status-btn"
                  :class="{ 'active': selectedPlace.status === 'active' }"
                  :disabled="isArchivedView"
                  @click="setPlaceStatus('active')"
                >
                  Активно
                </button>
                <button
                  class="status-btn"
                  :class="{ 'active': selectedPlace.status === 'inactive' }"
                  :disabled="isArchivedView"
                  @click="setPlaceStatus('inactive')"
                >
                  Не активно
                </button>
              </div>
            </div>

            <!-- Комментарий к статусу (только для неактивных) -->
            <div
              v-if="selectedPlace.status !== 'active'"
              class="detail-group"
            >
              <label class="detail-label">Причина:</label>
              <textarea
                v-model="selectedPlace.status_comment"
                class="form-textarea"
                placeholder="Укажите причину закрытия"
                rows="2"
                :disabled="isArchivedView"
                @change="updatePlace(selectedPlace)"
              />
            </div>

            <!-- Привязки к организациям/компаниям + «Отвязать всё» (#1379) -->
            <div class="usage-section usage-section--inline">
              <div class="usage-header">
                <div class="usage-header__text">
                  <h4 class="section-title">
                    Привязано к организациям и компаниям
                  </h4>
                  <p class="field-hint">
                    Пока место разгрузки привязано хотя бы к одной организации или
                    компании, его нельзя удалить. Отвяжите все, чтобы освободить место.
                  </p>
                </div>
                <button
                  v-if="canDetachUnloadPlace && !usageLoading && !usageError && usageHasBindings"
                  class="action-btn detach-all-btn"
                  :disabled="detaching || detachingOne"
                  @click="confirmDetachAll"
                >
                  {{ detaching ? 'Отвязываем...' : 'Отвязать всё' }}
                </button>
              </div>

              <div
                v-if="usageLoading"
                class="usage-state"
              >
                Загрузка привязок...
              </div>
              <div
                v-else-if="usageError"
                class="usage-state usage-state--error"
              >
                {{ usageError }}
              </div>
              <template v-else>
                <div class="usage-group">
                  <div class="usage-group__title">
                    Организации: {{ usage.organizations.length }}
                  </div>
                  <ul
                    v-if="usage.organizations.length"
                    class="usage-list"
                  >
                    <li
                      v-for="org in usage.organizations"
                      :key="'org-' + org.id"
                      class="usage-item"
                    >
                      <span class="usage-item__name">{{ org.name }}</span>
                      <span
                        v-if="!org.is_active"
                        class="usage-item__archived"
                      >(архив)</span>
                      <button
                        v-if="canDetachUnloadPlace"
                        class="usage-item__detach"
                        data-hint="Отвязать"
                        :disabled="detaching || detachingOne"
                        @click="confirmDetachOne('organization', org)"
                      >
                        &times;
                      </button>
                    </li>
                  </ul>
                  <p
                    v-else
                    class="usage-empty"
                  >
                    Нет привязанных организаций
                  </p>
                </div>

                <div class="usage-group">
                  <div class="usage-group__title">
                    Компании: {{ usage.companies.length }}
                  </div>
                  <ul
                    v-if="usage.companies.length"
                    class="usage-list"
                  >
                    <li
                      v-for="comp in usage.companies"
                      :key="'comp-' + comp.id"
                      class="usage-item"
                    >
                      <span class="usage-item__name">{{ comp.name }}</span>
                      <span
                        v-if="!comp.is_active"
                        class="usage-item__archived"
                      >(архив)</span>
                      <button
                        v-if="canDetachUnloadPlace"
                        class="usage-item__detach"
                        data-hint="Отвязать"
                        :disabled="detaching || detachingOne"
                        @click="confirmDetachOne('company', comp)"
                      >
                        &times;
                      </button>
                    </li>
                  </ul>
                  <p
                    v-else
                    class="usage-empty"
                  >
                    Нет привязанных компаний
                  </p>
                </div>
              </template>
            </div>
          </div>
        </div>

        <!-- Вкладка Расписание -->
        <div
          v-if="activeTab === 'schedule'"
          class="tab-content"
        >
          <WorkScheduleTab
            :resource-url="'/unload-places/' + selectedPlace.id"
            :time-slots="selectedPlace.time_slots"
            :readonly="isArchivedView"
            @update="refreshSelectedPlace"
          />
        </div>

        <!-- Вкладка Предупреждения -->
        <div
          v-if="activeTab === 'warnings'"
          class="tab-content"
        >
          <div class="warnings-section">
            <h4 class="section-title">
              Свободное предупреждение
            </h4>
            <p class="field-hint">
              Показывается заявителю всегда при добавлении машины/человека с этим
              местом.
            </p>
            <textarea
              v-model="selectedPlace.warning"
              class="form-textarea"
              placeholder="Например: въезд только по предварительной записи"
              rows="2"
              :disabled="isArchivedView"
              @change="updatePlace(selectedPlace)"
            />
          </div>

          <div class="warnings-section">
            <WarningWindowsEditor
              :resource-url="'/unload-places/' + selectedPlace.id"
              :windows="selectedPlace.warning_windows || []"
              :readonly="isArchivedView"
              @update="refreshSelectedPlace"
            />
          </div>
        </div>

        <!-- Вкладка Маршрут -->
        <div
          v-if="activeTab === 'route'"
          class="tab-content"
        >
          <div class="route-section">
            <h4 class="section-title">
              Ссылка на локацию на карте
            </h4>
            <p class="field-hint">
              Видна водителю в карточке места при подаче заявки - откроется
              в Яндекс/Google Maps.
            </p>
            <div class="map-link-group">
              <input
                v-model="selectedPlace.map_link"
                class="form-input"
                placeholder="https://maps.google.com/..."
                autocomplete="off"
                :disabled="isArchivedView"
                @change="updatePlace(selectedPlace)"
              >
              <a 
                v-if="selectedPlace.map_link" 
                :href="selectedPlace.map_link" 
                target="_blank" 
                class="map-link-btn"
              >
                Открыть карту
              </a>
            </div>
          </div>

          <div class="route-section">
            <div class="photos-header">
              <h4 class="section-title">
                Изображение(-я)
              </h4>
              <label
                v-if="!isArchivedView"
                class="upload-photo-btn"
              >
                + Загрузить
                <input
                  type="file"
                  accept="image/*"
                  multiple
                  style="display: none"
                  @change="uploadPhotos"
                >
              </label>
            </div>
            <p class="field-hint">
              Снимки места разгрузки - схема проезда, КПП, парковка. Видит
              водитель в карточке места при подаче заявки.
            </p>

            <!-- Drag&drop zone (как в TableConstructorPhotoSection). -->
            <label
              v-if="!isArchivedView"
              class="photo-dropzone"
              :class="{ 'photo-dropzone--active': isDraggingPhoto }"
              @dragenter.prevent="isDraggingPhoto = true"
              @dragover.prevent="isDraggingPhoto = true"
              @dragleave.prevent="isDraggingPhoto = false"
              @drop.prevent="onPhotoDrop"
            >
              <input
                type="file"
                accept="image/*"
                multiple
                class="photo-dropzone__input"
                @change="uploadPhotos"
              >
              <svg
                width="32"
                height="32"
                viewBox="0 0 24 24"
                fill="none"
                class="photo-dropzone__icon"
              >
                <path
                  d="M12 4v12m0 0l-4-4m4 4l4-4M4 20h16"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              <div class="photo-dropzone__text">
                <strong>Перетащите фотографии сюда</strong>
                <span>или нажмите, чтобы выбрать из обзора</span>
              </div>
            </label>

            <div class="photos-grid">
              <div
                v-for="photo in selectedPlace.photos"
                :key="photo.id"
                class="photo-item"
                :class="{ 'main-photo': photo.is_main }"
              >
                <div
                  class="photo-preview"
                  @click="viewPhoto(photo)"
                >
                  <img
                    :src="photo.photo_url"
                    :alt="photo.file_name"
                  >
                </div>
                <div
                  v-if="!isArchivedView"
                  class="photo-actions"
                >
                  <button
                    v-if="!photo.is_main"
                    class="photo-main-btn"
                    title="Сделать главной"
                    @click="setMainPhoto(photo)"
                  >
                    ★
                  </button>
                  <span
                    v-else
                    class="photo-main-badge"
                    title="Главная фотография"
                  >★</span>
                  <button
                    class="photo-delete-btn"
                    title="Удалить"
                    @click="deletePhoto(photo)"
                  >
                    <AppIcon
                      name="trashcan"
                      class="action-icon-small"
                    />
                  </button>
                </div>
              </div>
              <div
                v-if="!selectedPlace.photos || selectedPlace.photos.length === 0"
                class="no-photos"
              >
                <p>Фотографии не загружены</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-else
        class="no-selection-message"
      >
        <p>Выберите место разгрузки для просмотра</p>
      </div>
    </div>


    <!-- Модальное окно добавления места -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showAddModal"
          class="modal-overlay"
          @mousedown="onAddOverlayMousedown"
          @mouseup="onAddOverlayMouseup"
        >
          <div
            class="modal-content"
            @mousedown.stop
            @click.stop
          >
            <div class="modal-header">
              <h3 class="modal-title">
                Добавить место разгрузки
              </h3>
              <button
                class="modal-close"
                @click="closeModal"
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
          
            <div class="modal-body">
              <div class="input-group">
                <label class="input-label">Наименование *</label>
                <input
                  ref="nameInput"
                  v-model="newPlaceName"
                  placeholder="Введите название места"
                  class="modal-input"
                  @keyup.enter="addPlace"
                >
              </div>
            
              <div class="input-group">
                <label class="input-label">Описание</label>
                <textarea
                  v-model="newPlaceDescription"
                  placeholder="Введите описание (необязательно)"
                  class="modal-textarea"
                  rows="3"
                />
              </div>

              <div class="input-group">
                <label class="input-label">Предупреждение</label>
                <textarea
                  v-model="newPlaceWarning"
                  placeholder="Показывается заявителю всегда (необязательно)"
                  class="modal-textarea"
                  rows="2"
                />
              </div>
            </div>
          
            <div class="modal-footer">
              <button
                class="modal-btn modal-btn--cancel"
                @click="closeModal"
              >
                Отмена
              </button>
              <button 
                class="modal-btn modal-btn--confirm" 
                :disabled="!newPlaceName.trim()"
                :class="{'modal-btn--disabled': !newPlaceName.trim()}"
                @click="addPlace"
              >
                Добавить
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <!-- Модальное окно просмотра фото -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showPhotoModal"
          class="modal-overlay"
          @mousedown="onPhotoOverlayMousedown"
          @mouseup="onPhotoOverlayMouseup"
        >
          <div
            class="modal-content photo-view-modal"
            @mousedown.stop
            @click.stop
          >
            <div class="modal-header">
              <h3 class="modal-title">
                {{ viewingPhoto?.file_name }}
              </h3>
              <button
                class="modal-close"
                @click="showPhotoModal = false"
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
            <div class="modal-body photo-view-body">
              <img
                :src="viewingPhoto?.photo_url"
                class="full-photo"
                alt="Full size"
              >
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <ConfirmationModal
      :show="!!deleteConfirmPlace"
      title="Архивация места разгрузки"
      :message="deleteConfirmPlace ? `Архивировать место разгрузки «${deleteConfirmPlace.name}»? Его можно будет восстановить из архива.` : ''"
      confirm-text="В архив"
      cancel-text="Отмена"
      @confirm="performDeletePlace"
      @cancel="deleteConfirmPlace = null"
    />

    <ConfirmationModal
      :show="!!deleteConfirmPhoto"
      title="Удаление фотографии"
      message="Удалить эту фотографию?"
      confirm-text="Удалить"
      cancel-text="Отмена"
      @confirm="performDeletePhoto"
      @cancel="deleteConfirmPhoto = null"
    />

    <ConfirmationModal
      :show="bulkConfirmVisible"
      :title="bulkConfirmTitle"
      :message="bulkConfirmMessage"
      :confirm-text="bulkConfirmText"
      cancel-text="Отмена"
      :confirm-button-style="bulkConfirmButtonStyle"
      @confirm="applyBulkArchiveRestore"
      @cancel="cancelBulkConfirm"
    />

    <ConfirmationModal
      :show="detachConfirmVisible"
      title="Отвязать все организации и компании"
      :message="detachConfirmMessage"
      confirm-text="Отвязать всё"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="performDetachAll"
      @cancel="detachConfirmVisible = false"
    />

    <ConfirmationModal
      :show="!!detachOneTarget"
      title="Отвязать привязку"
      :message="detachOneConfirmMessage"
      confirm-text="Отвязать"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="performDetachOne"
      @cancel="detachOneTarget = null"
    />

    <UnloadPlaceHistoryModal
      v-if="historyPlace"
      :unload-place="historyPlace"
      :current-user-name="currentUserName"
      @close="historyPlace = null"
    />
  </div>
</template>

<script>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { apiRequest } from '@/api/client'
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants'
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import { useOverlayClose } from '@/composables/useOverlayClose';
import RefreshButton from '../RefreshButton.vue';
import SearchComponent from '../SearchComponent.vue';
import ConfirmationModal from '../ConfirmationModal.vue';
import BaseDropdown from '../ui/BaseDropdown.vue';
import WorkScheduleTab from '../WorkScheduleTab.vue';
import WarningWindowsEditor from '../WarningWindowsEditor.vue';
import UnloadPlaceHistoryModal from './UnloadPlaceHistoryModal.vue';
import { bulkArchiveUnloadPlaces, bulkRestoreUnloadPlaces, getUnloadPlaceUsage, detachAllUnloadPlace, detachOrganizationFromUnloadPlace, detachCompanyFromUnloadPlace } from '@/api/unload-places';
import AppIcon from '@/components/icons/AppIcon.vue';
import { fetchCurrentUserName } from '@/utils/currentUserName';
import { openFromSearchLink } from '@/mixins/openFromSearchLink'

export default {
  mixins: [openFromSearchLink((vm) => vm.unloadPlaces, 'selectPlace')],
  components: {
    SearchComponent,
    RefreshButton,
    ConfirmationModal,
    BaseDropdown,
    WorkScheduleTab,
    WarningWindowsEditor,
    UnloadPlaceHistoryModal,
    AppIcon,
  },
  setup() {
    // Колбэк закрытия присваивается в created (нужен доступ к this).
    const addOverlay = { close: () => {} };
    const photoOverlay = { close: () => {} };
    const add = useOverlayClose(() => addOverlay.close());
    const photo = useOverlayClose(() => photoOverlay.close());
    return {
      onAddOverlayMousedown: add.onOverlayMousedown,
      onAddOverlayMouseup: add.onOverlayMouseup,
      onPhotoOverlayMousedown: photo.onOverlayMousedown,
      onPhotoOverlayMouseup: photo.onOverlayMouseup,
      addOverlay,
      photoOverlay
    };
  },
  data() {
    return {
      searchQuery: '',
      showArchive: false,
      archiveOptions: [
        { label: 'Активные', value: 'active' },
        { label: 'Архив', value: 'archive' },
      ],
      newPlaceName: '',
      newPlaceDescription: '',
      newPlaceWarning: '',
      unloadPlaces: [],
      showAddModal: false,
      showPhotoModal: false,
      selectedPlace: null,
      viewingPhoto: null,
      sortField: null,
      sortDirection: 'asc',
      activeTab: 'main',
      isDraggingPhoto: false,
      refreshing: false,
      deleteConfirmPlace: null,
      deleteConfirmPhoto: null,
      historyPlace: null,
      currentUserName: '',
      // Групповой выбор (по id). lastSelectedId - якорь shift-диапазона.
      selectedIds: [],
      lastSelectedId: null,
      pendingBulkOp: null,
      bulkConfirmVisible: false,
      bulkSubmitting: false,
      // Привязки (блок на вкладке «Основное»): организации/компании, держащие место разгрузки.
      usage: { organizations: [], companies: [] },
      usageLoading: false,
      usageError: '',
      usageSeq: 0,
      detaching: false,
      detachConfirmVisible: false,
      // Точечная отвязка: { kind: 'organization'|'company', id, name } | null.
      detachOneTarget: null,
      detachingOne: false,
    };
  },
  computed: {
    isArchivedView() {
      return !!this.selectedPlace && !this.selectedPlace.is_active;
    },
    usageHasBindings() {
      return this.usage.organizations.length > 0 || this.usage.companies.length > 0;
    },
    // Зеркалит BE-гейт detach-all: отвязка закрыта тем же page.admin.directories,
    // что открывает экран (#1982).
    canDetachUnloadPlace() {
      return usePermissionsStore().hasPermission('page.admin.directories');
    },
    detachConfirmMessage() {
      if (!this.selectedPlace) return '';
      const o = this.usage.organizations.length;
      const c = this.usage.companies.length;
      return `Отвязать место разгрузки «${this.selectedPlace.name}» от всех организаций (${o}) и компаний (${c})? Это освободит место, чтобы его можно было удалить.`;
    },
    detachOneConfirmMessage() {
      if (!this.detachOneTarget || !this.selectedPlace) return '';
      const kind = this.detachOneTarget.kind === 'organization' ? 'организацию' : 'компанию';
      return `Отвязать ${kind} «${this.detachOneTarget.name}» от места разгрузки «${this.selectedPlace.name}»?`;
    },
    allSelected() {
      return this.sortedUnloadPlaces.length > 0 && this.selectedIds.length === this.sortedUnloadPlaces.length;
    },
    someSelected() {
      return this.selectedIds.length > 0 && !this.allSelected;
    },
    bulkConfirmTitle() {
      return this.pendingBulkOp === 'restore' ? 'Восстановление мест разгрузки' : 'Архивация мест разгрузки';
    },
    bulkConfirmMessage() {
      const n = this.selectedIds.length;
      return this.pendingBulkOp === 'restore'
        ? `Восстановить выбранные места разгрузки (${n})?`
        : `Архивировать выбранные места разгрузки (${n})? Их можно будет восстановить из архива.`;
    },
    bulkConfirmText() {
      return this.pendingBulkOp === 'restore' ? 'Восстановить' : 'В архив';
    },
    bulkConfirmButtonStyle() {
      return this.pendingBulkOp === 'restore'
        ? { background: '#10b981', borderColor: '#10b981' }
        : { background: '#c62828', borderColor: '#c62828' };
    },
    emptyText() {
      if (this.searchQuery) return 'Ничего не найдено по фильтру';
      return this.showArchive ? 'В архиве пусто' : 'Места разгрузки пока нет';
    },
    filteredUnloadPlaces() {
      // Тянем активные и архивные одним запросом, режим фильтрует на клиенте.
      const byMode = this.unloadPlaces.filter(place =>
        this.showArchive ? !place.is_active : place.is_active
      );
      const variants = buildSearchVariants(this.searchQuery);
      if (!variants.length) return byMode;
      return byMode.filter(place => matchesSearch(`${place.name} ${place.id}`, variants));
    },
    sortedUnloadPlaces() {
      const places = [...this.filteredUnloadPlaces];
      
      if (!this.sortField) {
        return places.sort((a, b) => a.name.localeCompare(b.name));
      }
      
      return places.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'id':
            valueA = a.id;
            valueB = b.id;
            break;
          case 'name':
            valueA = a.name;
            valueB = b.name;
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
    }
  },
  watch: {
    showAddModal(newVal) {
      this.syncBodyScroll();
      if (newVal) {
        this.$nextTick(() => {
          this.$refs.nameInput?.focus();
        });
      }
    },
    showPhotoModal() {
      this.syncBodyScroll();
    },
    // Список фильтруется по режиму архив/поиск - выпавшие из вида id снимаем.
    sortedUnloadPlaces() {
      this.pruneSelection();
    },
    // Привязки показываются на вкладке «Основное» - грузим при смене места
    // (id меняется), а не по правке полей того же места (id тот же).
    'selectedPlace.id'(id) {
      if (id) this.loadUsage();
    }
  },
  created() {
    this.addOverlay.close = () => { this.closeModal(); };
    this.photoOverlay.close = () => { this.showPhotoModal = false; };
  },
  mounted() {
    this.refreshData();
    this.fetchCurrentUser();
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeydown);
    releaseBodyScrollLock(this);
  },
  methods: {
    // Скролл body блокируется, пока открыта ЛЮБАЯ из модалок (не залипает при
    // закрытии одной, если вдруг открыта другая).
    syncBodyScroll() {
      setBodyScrollLock(this, this.showAddModal || this.showPhotoModal);
    },

    onKeydown(e) {
      if (e.key !== 'Escape') return;
      if (this.showPhotoModal) {
        this.showPhotoModal = false;
      } else if (this.showAddModal) {
        this.closeModal();
      }
    },

    async refreshData() {
      this.refreshing = true;
      try {
        await this.fetchUnloadPlaces();
      } finally {
        this.refreshing = false;
      }
    },
    
    async fetchUnloadPlaces() {
      try {
        const response = await apiRequest("/unload-places?include_archived=true", {
        });
        if (response.ok) {
          const data = await response.json();
          this.unloadPlaces = data.map(place => ({
            ...place,
            originalName: place.name,
            originalDescription: place.description,
            originalWarning: place.warning,
            originalMapLink: place.map_link,
            originalStatus: place.status,
            originalStatusComment: place.status_comment
          }));
          this.pruneSelection();
          this.openFromSearchLink();
        }
      } catch (error) {
        console.error("Error fetching unload places:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'места разгрузки', type: 'error' });
      }
    },
    
    async refreshSelectedPlace() {
  if (!this.selectedPlace) return;
  
  try {
    const response = await apiRequest(`/unload-places/${this.selectedPlace.id}`, {
    });
    if (response.ok) {
      const data = await response.json();
      
      // Исправляем URL фотографий
      if (data.photos) {
        data.photos = data.photos.map(photo => ({
          ...photo,
          photo_url: photo.photo_url
        }));
      }
      
      this.selectedPlace = {
        ...data,
        originalName: data.name,
        originalDescription: data.description,
        originalWarning: data.warning,
        originalMapLink: data.map_link,
        originalStatus: data.status,
        originalStatusComment: data.status_comment
      };
      
      // Обновляем в общем списке
      const index = this.unloadPlaces.findIndex(p => p.id === data.id);
      if (index !== -1) {
        this.unloadPlaces[index] = { ...this.selectedPlace };
      }
    }
  } catch (error) {
    console.error("Error refreshing place:", error);
    useDeletionsStore().notify({ prefix: 'Не удалось обновить ', bold: 'данные места разгрузки', type: 'error' });
  }
},
    
    async addPlace() {
      if (!this.newPlaceName.trim()) {
        useDeletionsStore().notify({ prefix: 'Не удалось добавить: ', bold: 'введите название места', type: 'error' });
        return;
      }
      const name = this.newPlaceName.trim();

      try {
        const response = await apiRequest("/unload-places", {
          method: "POST",
          body: JSON.stringify({
            name: this.newPlaceName,
            description: this.newPlaceDescription || null,
            warning: this.newPlaceWarning || null,
            status: 'active',
            status_comment: null
          }),
        });

        if (response.ok) {
          const result = await response.json();
          this.newPlaceName = '';
          this.newPlaceDescription = '';
          this.newPlaceWarning = '';
          this.showAddModal = false;
          await this.refreshData();
          
          const newPlace = this.unloadPlaces.find(p => p.id === result.id);
          if (newPlace) {
            this.selectPlace(newPlace);
          }
          
          useDeletionsStore().notify({ prefix: 'Место разгрузки ', bold: name, suffix: ' создано' });
        } else {
          const err = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось добавить место разгрузки: ', bold: err.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Error adding unload place:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось добавить место разгрузки: ', bold: 'ошибка сети', type: 'error' });
      }
    },
    
    async updatePlace(place) {
      // Архивную запись редактировать нельзя - бэкенд PUT её не блокирует,
      // поэтому страхуемся тут (в UI поля и так disabled).
      if (!place.is_active) return;

      const hasChanges =
        place.name !== place.originalName ||
        place.description !== place.originalDescription ||
        place.warning !== place.originalWarning ||
        place.map_link !== place.originalMapLink ||
        place.status !== place.originalStatus ||
        place.status_comment !== place.originalStatusComment;

      if (!hasChanges) return;

      try {
        const response = await apiRequest(`/unload-places/${place.id}`, {
          method: "PUT",
          body: JSON.stringify({
            name: place.name,
            description: place.description,
            warning: place.warning,
            map_link: place.map_link,
            status: place.status,
            status_comment: place.status_comment
          }),
        });

        if (response.ok) {
          place.originalName = place.name;
          place.originalDescription = place.description;
          place.originalWarning = place.warning;
          place.originalMapLink = place.map_link;
          place.originalStatus = place.status;
          place.originalStatusComment = place.status_comment;
          
          const index = this.unloadPlaces.findIndex(p => p.id === place.id);
          if (index !== -1) {
            this.unloadPlaces[index] = { ...place };
          }
          
          useDeletionsStore().notify({ prefix: 'Изменения сохранены в ', bold: place.name });
        } else {
          const err = await response.json();
          this.revertPlaceChanges(place);
          useDeletionsStore().notify({ prefix: 'Не удалось сохранить: ', bold: err.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Error updating unload place:", error);
        this.revertPlaceChanges(place);
        useDeletionsStore().notify({ prefix: 'Не удалось сохранить: ', bold: 'ошибка сети', type: 'error' });
      }
    },
    
    revertPlaceChanges(place) {
      place.name = place.originalName;
      place.description = place.originalDescription;
      place.warning = place.originalWarning;
      place.map_link = place.originalMapLink;
      place.status = place.originalStatus;
      place.status_comment = place.originalStatusComment;
    },
    
    setPlaceStatus(status) {
      if (!this.selectedPlace) return;
      this.selectedPlace.status = status;
      if (status === 'active') {
        this.selectedPlace.status_comment = null;
      }
      this.updatePlace(this.selectedPlace);
    },
    
    confirmDeletePlace(place) {
      this.deleteConfirmPlace = place;
    },

    async performDeletePlace() {
      const place = this.deleteConfirmPlace;
      this.deleteConfirmPlace = null;
      if (!place) return;

      try {
        const response = await apiRequest(`/unload-places/${place.id}`, {
          method: "DELETE",
        });

        if (response.ok) {
          this.selectedPlace = null;
          this.activeTab = 'main';
          await this.refreshData();
          useDeletionsStore().notify({ prefix: 'Место разгрузки ', bold: place.name, suffix: ' архивировано' });
        } else {
          const error = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось архивировать: ', bold: error.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Error archiving unload place:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось архивировать: ', bold: 'ошибка сети', type: 'error' });
      }
    },
    
    onArchiveModeChange(value) {
      this.showArchive = value === 'archive';
      this.selectedPlace = null;
      this.activeTab = 'main';
      this.clearSelection();
    },

    async onRestore(place) {
      try {
        const response = await apiRequest(`/unload-places/${place.id}/restore`, {
          method: "POST",
        });

        if (response.ok) {
          this.selectedPlace = null;
          this.activeTab = 'main';
          await this.refreshData();
          useDeletionsStore().notify({ prefix: 'Место разгрузки ', bold: place.name, suffix: ' восстановлено из архива' });
        } else {
          const err = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось восстановить: ', bold: err.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Error restoring unload place:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось восстановить: ', bold: 'ошибка сети', type: 'error' });
      }
    },

    openHistory(place) {
      this.historyPlace = place;
    },

    // seq-guard: быстрое переключение мест не даст устаревшему ответу затереть
    // актуальные привязки (last-resolve-wins иначе показал бы чужое место).
    async loadUsage() {
      if (!this.selectedPlace) return;
      const seq = ++this.usageSeq;
      this.usageLoading = true;
      this.usageError = '';
      // Гасим привязки предыдущего места сразу: пока грузятся новые, кнопка
      // «Отвязать всё» и текст подтверждения не должны показывать чужие цифры.
      this.usage = { organizations: [], companies: [] };
      try {
        const data = await getUnloadPlaceUsage(this.selectedPlace.id);
        if (seq !== this.usageSeq) return;
        this.usage = {
          organizations: data?.organizations || [],
          companies: data?.companies || [],
        };
      } catch (err) {
        if (seq !== this.usageSeq) return;
        this.usage = { organizations: [], companies: [] };
        this.usageError = err instanceof TypeError
          ? 'Не удалось загрузить привязки (ошибка сети)'
          : (err.message || 'Не удалось загрузить привязки');
      } finally {
        if (seq === this.usageSeq) this.usageLoading = false;
      }
    },

    confirmDetachAll() {
      this.detachConfirmVisible = true;
    },

    async performDetachAll() {
      this.detachConfirmVisible = false;
      const place = this.selectedPlace;
      if (!place) return;
      this.detaching = true;
      try {
        const res = await detachAllUnloadPlace(place.id);
        const orgN = res?.organizations_detached || 0;
        const compN = res?.companies_detached || 0;
        // Перезагружаем привязки только если пользователь не ушёл на другое место,
        // пока летел запрос (иначе затрём usage чужого места).
        if (this.selectedPlace && this.selectedPlace.id === place.id) {
          await this.loadUsage();
        }
        useDeletionsStore().notify({
          prefix: 'Место разгрузки ',
          bold: place.name,
          suffix: ` отвязано от организаций (${orgN}) и компаний (${compN})`,
        });
      } catch (err) {
        const msg = err instanceof TypeError ? 'ошибка сети' : (err.message || 'ошибка');
        useDeletionsStore().notify({ prefix: 'Не удалось отвязать: ', bold: msg, type: 'error' });
      } finally {
        this.detaching = false;
      }
    },

    confirmDetachOne(kind, item) {
      this.detachOneTarget = { kind, id: item.id, name: item.name };
    },

    async performDetachOne() {
      const target = this.detachOneTarget;
      const place = this.selectedPlace;
      this.detachOneTarget = null;
      if (!target || !place) return;
      this.detachingOne = true;
      try {
        if (target.kind === 'organization') {
          await detachOrganizationFromUnloadPlace(place.id, target.id);
        } else {
          await detachCompanyFromUnloadPlace(place.id, target.id);
        }
        // Перезагружаем привязки, только если не ушли на другое место.
        if (this.selectedPlace && this.selectedPlace.id === place.id) {
          await this.loadUsage();
        }
        useDeletionsStore().notify({
          prefix: target.kind === 'organization' ? 'Организация ' : 'Компания ',
          bold: target.name,
          suffix: ' отвязана от места разгрузки',
        });
      } catch (err) {
        const msg = err instanceof TypeError ? 'ошибка сети' : (err.message || 'ошибка');
        useDeletionsStore().notify({ prefix: 'Не удалось отвязать: ', bold: msg, type: 'error' });
      } finally {
        this.detachingOne = false;
      }
    },

    async fetchCurrentUser() {
      this.currentUserName = await fetchCurrentUserName();
    },

    selectPlace(place) {
      this.selectedPlace = { ...place };
      this.activeTab = 'main';
    },
    
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'asc';
      }
    },
    
    closeModal() {
      this.showAddModal = false;
      this.newPlaceName = '';
      this.newPlaceDescription = '';
      this.newPlaceWarning = '';
    },
    
    // В методе uploadPhotos, после успешной загрузки, нужно обработать photo_url
async uploadPhotos(event) {
  const files = event.target.files;
  if (!files || files.length === 0) return;
  await this.uploadPhotoFiles(files);
  event.target.value = '';
},

onPhotoDrop(event) {
  this.isDraggingPhoto = false;
  const files = event.dataTransfer?.files;
  if (files && files.length) this.uploadPhotoFiles(files);
},

async uploadPhotoFiles(files) {
  if (!this.selectedPlace) return;
  const formData = new FormData();
  for (let i = 0; i < files.length; i++) {
    if (!files[i].type || files[i].type.startsWith('image/')) {
      formData.append('photos', files[i]);
    }
  }
  try {
    const response = await apiRequest(`/unload-places/${this.selectedPlace.id}/photos`, {
      method: "POST",
      body: formData,
      headers: {},
    });
    if (response.ok) {
      await this.refreshSelectedPlace();
      if (this.selectedPlace && this.selectedPlace.photos) {
        this.selectedPlace.photos = this.selectedPlace.photos.map(photo => ({
          ...photo,
          photo_url: photo.photo_url
        }));
      }
      useDeletionsStore().notify({ prefix: 'Фотографии загружены в ', bold: this.selectedPlace.name });
    } else {
      const err = await response.json();
      useDeletionsStore().notify({ prefix: 'Не удалось загрузить фото: ', bold: err.message || 'ошибка', type: 'error' });
    }
  } catch (error) {
    console.error("Error uploading photos:", error);
    useDeletionsStore().notify({ prefix: 'Не удалось загрузить фото: ', bold: 'ошибка сети', type: 'error' });
  }
},
    
    deletePhoto(photo) {
      this.deleteConfirmPhoto = photo;
    },

    async performDeletePhoto() {
      const photo = this.deleteConfirmPhoto;
      this.deleteConfirmPhoto = null;
      if (!photo || !this.selectedPlace) return;

      try {
        const response = await apiRequest(`/unload-places/${this.selectedPlace.id}/photos/${photo.id}`,
          {
            method: "DELETE",
          }
        );

        if (response.ok) {
          await this.refreshSelectedPlace();
          useDeletionsStore().notify({ prefix: 'Фотография удалена из ', bold: this.selectedPlace.name });
        } else {
          const err = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось удалить фото: ', bold: err.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Error deleting photo:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось удалить фото: ', bold: 'ошибка сети', type: 'error' });
      }
    },
    
    async setMainPhoto(photo) {
      try {
        const response = await apiRequest(`/unload-places/${this.selectedPlace.id}/photos/${photo.id}/main`,
          {
            method: "POST",
          }
        );
        
        if (response.ok) {
          await this.refreshSelectedPlace();
          useDeletionsStore().notify({ prefix: 'Главная фотография установлена для ', bold: this.selectedPlace.name });
        } else {
          const err = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось установить главное фото: ', bold: err.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Error setting main photo:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось установить главное фото: ', bold: 'ошибка сети', type: 'error' });
      }
    },
    
    viewPhoto(photo) {
      this.viewingPhoto = photo;
      this.showPhotoModal = true;
    },
    
    // Вспомогательные методы
    getStatusClass(place) {
      if (place.status !== 'active') {
        return 'status-inactive';
      }
      return place.current_status === 'open' ? 'status-open' : 'status-closed';
    },
    
    getStatusText(place) {
      if (place.status !== 'active') {
        return 'Неактивно';
      }
      return place.current_status === 'open' ? 'Открыто' : 'Закрыто';
    },
    
    getCurrentStatusClass(place) {
      if (place.status !== 'active') {
        return 'status-inactive-badge';
      }
      return place.current_status === 'open' ? 'status-open-badge' : 'status-closed-badge';
    },
    
    getCurrentStatusText(place) {
      if (place.status !== 'active') {
        return 'Неактивно';
      }
      return place.current_status === 'open' ? 'Открыто сейчас' : 'Закрыто сейчас';
    },

    // --- Групповой выбор ---
    isSelected(id) {
      return this.selectedIds.includes(id);
    },
    toggleSelect(id) {
      const i = this.selectedIds.indexOf(id);
      if (i === -1) this.selectedIds.push(id);
      else this.selectedIds.splice(i, 1);
    },
    // onRowCheck: обычный клик - toggle; shift-клик - диапазон от якоря до текущей.
    onRowCheck(place, index, event) {
      if (event.shiftKey && window.getSelection) window.getSelection().removeAllRanges();
      if (event.shiftKey && this.lastSelectedId != null && this.lastSelectedId !== place.id) {
        const list = this.sortedUnloadPlaces;
        const anchor = list.findIndex(p => p.id === this.lastSelectedId);
        if (anchor !== -1) {
          const [from, to] = anchor < index ? [anchor, index] : [index, anchor];
          const target = !this.isSelected(place.id);
          for (let i = from; i <= to; i++) {
            const id = list[i].id;
            const sel = this.isSelected(id);
            if (target && !sel) this.selectedIds.push(id);
            else if (!target && sel) this.selectedIds.splice(this.selectedIds.indexOf(id), 1);
          }
          this.lastSelectedId = place.id;
          return;
        }
      }
      this.toggleSelect(place.id);
      this.lastSelectedId = place.id;
    },
    toggleSelectAll() {
      this.selectedIds = this.allSelected ? [] : this.sortedUnloadPlaces.map(p => p.id);
      this.lastSelectedId = null;
    },
    clearSelection() {
      this.selectedIds = [];
      this.lastSelectedId = null;
      this.pendingBulkOp = null;
    },
    pruneSelection() {
      if (!this.selectedIds.length) return;
      const visible = new Set(this.sortedUnloadPlaces.map(p => p.id));
      const pruned = this.selectedIds.filter(id => visible.has(id));
      if (pruned.length !== this.selectedIds.length) this.selectedIds = pruned;
    },
    startBulkOperation(operation) {
      this.pendingBulkOp = operation;
      this.bulkConfirmVisible = true;
    },
    cancelBulkConfirm() {
      if (this.bulkSubmitting) return;
      this.bulkConfirmVisible = false;
      this.pendingBulkOp = null;
    },
    async applyBulkArchiveRestore() {
      const ids = [...this.selectedIds];
      const op = this.pendingBulkOp;
      if (this.bulkSubmitting) return;
      if (!ids.length || (op !== 'archive' && op !== 'restore')) {
        this.bulkConfirmVisible = false;
        this.pendingBulkOp = null;
        return;
      }
      this.bulkSubmitting = true;
      let result;
      try {
        result = op === 'archive' ? await bulkArchiveUnloadPlaces(ids) : await bulkRestoreUnloadPlaces(ids);
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить групповую операцию', type: 'error' });
        this.bulkSubmitting = false;
        return;
      }
      this.bulkSubmitting = false;
      if (this.handleBulkResult(op, result, ids.length)) {
        this.bulkConfirmVisible = false;
        this.pendingBulkOp = null;
      }
    },
    // Разбор BulkOpResult: полный успех -> notify, частичный -> ui.warning с
    // перечнем непрошедших. false при ошибке-envelope (держим модалку для повтора).
    handleBulkResult(op, result, total) {
      if (!result || typeof result.success_count !== 'number') {
        useDeletionsStore().notify({ prefix: result?.message || 'Не удалось выполнить групповую операцию', type: 'error' });
        return false;
      }
      const label = op === 'restore' ? 'Восстановлено' : 'Архивировано';
      if (result.error_count > 0) {
        const failed = (result.errors || []).map(e => e.name || `#${e.id}`).join(', ');
        useDeletionsStore().notify({ prefix: 'Выполнено ', bold: `${result.success_count} из ${total}`, suffix: `. Не удалось: ${failed}`, type: 'warning' });
      } else {
        useDeletionsStore().notify({ prefix: `${label}: `, bold: String(result.success_count) });
      }
      this.clearSelection();
      this.refreshData();
      return true;
    },
  }
};
</script>

<style scoped>
.unload-places-container {
  background: var(--surface);
  border-radius: 16px;
  border: 1px solid var(--border);
  overflow: hidden;
  width: 100%;
  height: 550px;
  position: relative; /* контекст для оверлей-панели .bulk-bar поверх шапки */
}

/* Панель групповых операций - оверлей поверх .management-header (не reflow,
   список не прыгает при выборе - урок #510). Высота = высоте шапки (50px). */
.bulk-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 6;
  display: flex;
  align-items: center;
  gap: 14px;
  height: 50px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  background: var(--accent-tint-solid);
  overflow-x: auto;
  overflow-y: hidden;
}
.bulk-count {
  font-size: 14px;
  font-weight: 600;
  color: var(--accent-text);
  white-space: nowrap;
}
.bulk-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
  margin-left: auto;
}
.bulk-actions .pill {
  flex: 0 0 auto;
  white-space: nowrap;
}
.pill {
  display: inline-flex;
  align-items: center;
  height: 30px;
  padding: 0 14px;
  border-radius: 50px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  border: none;
  font-family: inherit;
  white-space: nowrap;
  transition: background 0.2s, border-color 0.2s;
}
.pill-ghost {
  background: var(--surface);
  color: var(--accent-text);
  border: 1px solid var(--accent);
}
.pill-ghost:hover {
  background: var(--accent-tint);
}
.bulk-clear {
  color: var(--text-muted);
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
}
.bulk-clear:hover {
  background: var(--surface-2);
}
.pill-danger {
  background: var(--surface);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}
.pill-danger:hover {
  background: var(--danger-bg);
  border-color: var(--danger);
}
.pill-restore {
  background: var(--success);
  color: var(--fill-text);
}
.pill-restore:hover {
  background: color-mix(in srgb, var(--success) 85%, var(--text));
}
.check-col {
  width: 8%;
  min-width: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 8px;
  cursor: default;
}
.bulk-check {
  width: 15px;
  height: 15px;
  cursor: pointer;
  accent-color: var(--accent-text);
  margin: 0;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  height: 50px;
}

.management-title {
  font-size: 1.2em;
  margin: 0;
  font-weight: 600;
  color: var(--text);
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.archive-dropdown {
  min-width: 130px;
}

.add-header-button {
  padding: 8px 16px;
  background: var(--accent);
  color: var(--accent-contrast);
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
  background: var(--accent-hover);
}

.content-container {
  display: flex;
  height: 500px;
  width: 100%;
}

/* Левая часть - таблица */
.table-section {
  width: 35%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border);
  background: var(--surface);
}

.table-section.with-details {
  width: 35%;
}

.table-container {
  background: var(--surface);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  height: 43px;
  align-items: center;
}

.header-col {
  padding: 0 8px;
  font-size: 14px;
  color: var(--text-muted);
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
  color: var(--text);
}

.header-col:hover .sort-icon {
  color: var(--text);
}

.sort-icon {
  color: var(--text-muted);
  width: 12px;
  height: 12px;
  transition: .2s;
}

.sort-icon.sorted {
  color: var(--text);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: var(--text) !important;
  font-weight: 600 !important;
}

.id-col {
  width: 18%;
  min-width: 54px;
}

.name-col {
  width: 51%;
  min-width: 140px;
}

.status-col {
  width: 23%;
  min-width: 74px;
}

.table-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.table-row {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  align-items: center;
  transition: background-color 0.2s ease;
  cursor: pointer;
  height: 42px;
  font-size: 14px;
}

.table-row:hover {
  background-color: var(--surface-2);
}

.table-row.selected {
  background-color: var(--accent-tint);
}

.table-row.inactive {
  background: var(--surface-2);
  color: var(--text-muted);
}

.table-row.inactive .id-value {
  color: var(--text-muted);
}

.inactive-badge {
  margin-left: 6px;
  font-size: 0.75em;
  color: var(--text-muted);
  font-style: italic;
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
  color: var(--text);
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.status-badge {
  display: inline-block;
  padding: 4px 10px;
  border-radius: 20px;
  font-size: 11px;
  font-weight: 500;
  min-width: 70px;
  text-align: center;
}

.status-open {
  background-color: var(--success-bg);
  color: var(--success-text);
  border: 1px solid var(--success);
}

.status-closed {
  background-color: var(--warning-bg);
  color: var(--warning-text);
  border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.status-inactive {
  background-color: var(--danger-bg);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.table-footer {
  padding: 6px 20px;
  border-top: 1px solid var(--border);
  text-align: right;
  background: var(--accent-tint);
}

.items-count {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

/* Правая часть - детали */
.details-section {
  width: 65%;
  display: flex;
  flex-direction: column;
  background: var(--surface);
  overflow: hidden;
}

.details-tabs {
  display: flex;
  flex-direction: column;
  gap: 6px;
  border-bottom: 1px solid var(--border);
  background: var(--surface-2);
  padding: 10px 16px;
}

.details-tabs__row {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}

.tab-btn {
  padding: 8px 18px;
  background: var(--surface);
  border: 1px solid transparent;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted);
  transition: color 0.2s ease, background 0.2s ease, border-color 0.2s ease;
  border-radius: 50px;
  white-space: nowrap;
  flex-shrink: 0;
}

.tab-btn:hover {
  color: var(--accent-text);
  background: var(--accent-tint);
}

.tab-btn.active {
  color: var(--accent-text);
  border-color: var(--accent);
  background: var(--surface);
}

.tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: var(--surface);
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.details-title-wrapper {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.details-title {
  margin: 0;
  color: var(--text);
  font-size: 1.2em;
  font-weight: 600;
}

.current-status-badge {
  display: inline-block;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
}

.status-open-badge {
  background-color: var(--success-bg);
  color: var(--success-text);
  border: 1px solid var(--success);
}

.status-closed-badge {
  background-color: var(--warning-bg);
  color: var(--warning-text);
  border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.status-inactive-badge {
  background-color: var(--danger-bg);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.details-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.archive-badge {
  background: var(--text-muted);
  color: var(--surface);
  padding: 4px 10px;
  border-radius: 50px;
  font-size: 0.75em;
  font-weight: 500;
  white-space: nowrap;
}

.action-btn {
  padding: 8px 16px;
  border: none;
  border-radius: 30px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  transition: background 0.2s, border-color 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
  white-space: nowrap;
}

.history-btn {
  background: var(--surface);
  color: var(--accent-text);
  border: 1px solid var(--accent);
}

.history-btn:hover {
  background: var(--accent-tint);
}

.archive-action-btn {
  background: var(--surface);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.archive-action-btn:hover {
  background: var(--danger-bg);
  border-color: var(--danger);
}

.restore-btn {
  background: var(--success);
  color: var(--fill-text);
}

.restore-btn:hover {
  background: color-mix(in srgb, var(--success) 85%, var(--text));
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.detail-label {
  font-size: 0.85em;
  color: var(--text-muted);
  font-weight:400;
}

.form-input {
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 14px;
  width: 100%;
  transition: border-color 0.2s ease;
  background: var(--surface);
}

.form-input:focus {
  border-color: var(--accent);
  outline: none;
}

.form-input:disabled,
.form-textarea:disabled {
  background: var(--accent-tint);
  color: var(--text-muted);
  cursor: not-allowed;
}

.status-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.form-textarea {
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 14px;
  width: 100%;
  transition: border-color 0.2s ease;
  background: var(--surface);
  resize: vertical;
  font-family: inherit;
}

.form-textarea:focus {
  border-color: var(--accent);
  outline: none;
}

/* Статус в виде кнопок */
.status-toggle {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.status-btn {
  padding: 6px 16px;
  border: 1px solid var(--border);
  background: var(--surface);
  border-radius: 30px;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
  color: var(--text-muted);
}

.status-btn:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent-text);
}

.status-btn.active {
  background: var(--accent);
  border-color: var(--accent);
  color: var(--accent-contrast);
}

/* Стили для маршрута */
.route-section {
  margin-bottom: 24px;
}

.warnings-section {
  margin-bottom: 24px;
}

.section-title {
  margin: 0 0 12px 0;
  font-size: 1em;
  font-weight: 600;
  color: var(--text);
}

.map-link-group {
  display: flex;
  gap: 12px;
  align-items: center;
}

.map-link-group .form-input {
  flex: 1;
}

.map-link-btn {
  padding: 8px 16px;
  background: var(--accent-tint);
  color: var(--accent-text);
  text-decoration: none;
  border-radius: 30px;
  font-size: 13px;
  white-space: nowrap;
  transition: background-color 0.2s ease;
  border: 1px solid var(--accent);
}

.map-link-btn:hover {
  background: var(--accent);
  color: var(--accent-contrast);
}

.photos-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.photos-header .section-title {
  margin: 0;
}

.upload-photo-btn {
  padding: 4px 12px;
  background: var(--accent-tint);
  color: var(--accent-text);
  border: 1px solid var(--accent);
  border-radius: 20px;
  cursor: pointer;
  font-size: 12px;
  transition: background-color 0.2s ease;
}

.upload-photo-btn:hover {
  background: var(--accent);
  color: var(--accent-contrast);
}

/* Подсказка под полем настройки. */
.field-hint {
  margin: 4px 0 8px;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}

/* Блок привязок на вкладке «Основное». */
.usage-section {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* Отделяем блок привязок от секций выше на вкладке «Основное». */
.usage-section--inline {
  margin-top: 24px;
  padding-top: 24px;
  border-top: 1px solid var(--border);
}

.usage-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.usage-header__text {
  flex: 1;
  min-width: 0;
}

.usage-header .section-title {
  margin: 0 0 4px 0;
}

.usage-header .field-hint {
  margin: 0;
}

.detach-all-btn {
  background: var(--surface);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
  white-space: nowrap;
}

.detach-all-btn:hover:not(:disabled) {
  background: var(--danger-bg);
  border-color: var(--danger);
}

.detach-all-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.usage-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.usage-group__title {
  font-size: 0.9em;
  font-weight: 600;
  color: var(--text);
}

.usage-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.usage-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: var(--accent-tint);
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 14px;
}

.usage-item__name {
  color: var(--text);
}

.usage-item__archived {
  color: var(--text-muted);
  font-size: 0.8em;
  font-weight: 500;
}

/* Крестик «Отвязать» на строке привязки (виден админу). */
.usage-item__detach {
  margin-left: auto;
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-size: 20px;
  line-height: 1;
  border-radius: 8px;
  cursor: pointer;
  transition: color 0.15s, background 0.15s;
  position: relative;
}

.usage-item__detach:hover:not(:disabled) {
  color: var(--danger-text);
  background: var(--danger-bg);
}

.usage-item__detach:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Всплывающая подсказка #333 как у прочих hint проекта (не native title). */
.usage-item__detach::after {
  content: attr(data-hint);
  position: absolute;
  bottom: calc(100% + 6px);
  right: 0;
  background: var(--hint-bg);
  color: var(--hint-text);
  font-size: 12px;
  white-space: nowrap;
  padding: 4px 8px;
  border-radius: 6px;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.15s;
  z-index: 1;
}

.usage-item__detach:hover:not(:disabled)::after {
  opacity: 1;
}

.usage-empty {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
}

.usage-state {
  font-size: 14px;
  color: var(--text-muted);
}

.usage-state--error {
  color: var(--danger-text);
}

/* Drag&drop zone (как в TableConstructorPhotoSection). */
.photo-dropzone {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 20px;
  margin-bottom: 12px;
  border: 2px dashed var(--accent);
  border-radius: 50px;
  background: var(--accent-tint);
  color: var(--text-muted);
  cursor: pointer;
  text-align: center;
  transition: border-color 0.2s ease, background 0.2s ease, color 0.2s ease;
}

.photo-dropzone:hover {
  border-color: var(--accent);
  background: color-mix(in srgb, var(--accent) 18%, var(--surface));
  color: var(--accent-text);
}

.photo-dropzone--active {
  border-color: var(--accent);
  background: var(--accent-tint);
  color: var(--accent-text);
}

.photo-dropzone__input {
  display: none;
}

.photo-dropzone__icon {
  color: inherit;
}

.photo-dropzone__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 13px;
  line-height: 1.4;
}

.photo-dropzone__text strong {
  color: var(--text);
  font-weight: 600;
}

.photo-dropzone:hover .photo-dropzone__text strong,
.photo-dropzone--active .photo-dropzone__text strong {
  color: var(--accent-text);
}

.photo-dropzone__text span {
  font-size: 11px;
}

.photos-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
  gap: 10px;
  max-height: 250px;
  overflow-y: auto;
  padding: 4px;
}

.photo-item {
  position: relative;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
  aspect-ratio: 1;
  background: var(--surface-2);
  
}

.photo-item.main-photo {
  border: 2px solid var(--accent);
}

.photo-preview {
  width: 100%;
  height: 100%;
  cursor: pointer;
}

.photo-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.photo-actions {
  position: absolute;
  top: 4px;
  right: 4px;
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.photo-item:hover .photo-actions {
  opacity: 1;
}

.photo-main-btn,
.photo-delete-btn,
.photo-main-badge {
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  transition: background-color 0.2s ease;
  background: rgba(255, 255, 255, 0.9);
}

.photo-main-btn:hover {
  background: var(--accent);
  color: var(--accent-contrast);
}

.photo-main-badge {
  background: var(--accent);
  color: var(--accent-contrast);
  cursor: default;
}

.photo-delete-btn:hover {
  background: var(--danger);
}

.photo-delete-btn:hover .action-icon-small {
  color: var(--fill-text);
}

.action-icon-small {
  /* Значок мельче 16px: общая обводка 1.7 садится в волосок, здесь плотнее. */
  stroke-width: 2.2;
  color: var(--text);
  width: 14px;
  height: 14px;
}

.no-photos {
  grid-column: 1 / -1;
  text-align: center;
  padding: 20px;
  color: var(--text-muted);
  background: var(--surface-2);
  border: 1px dashed var(--border);
  border-radius: 25px;
  font-size: 15px;
}

.no-selection-message {
  width: 65%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-weight: 400;
  font-size: 14px;
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-muted);
  width: 100%;
}

/* Стили для модальных окон */
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
  border-radius: 30px;
  padding: 0;
  width: 420px;
  max-width: 90vw;
  box-shadow: 0 20px 60px var(--shadow-drop);
  animation: modalAppear 0.3s ease-out;
}

.modal-content.photo-view-modal {
  width: 800px;
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
  padding: 20px 24px 16px;
  border-bottom: 1px solid var(--border);
}

.modal-title {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: var(--text);
}

.modal-close {
  background: none;
  border: none;
  cursor: pointer;
  padding: 6px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s ease;
}

.modal-close:hover {
  background-color: var(--surface-2);
}

.modal-body {
  padding: 20px 24px;
  max-height: calc(var(--app-vh, 1vh) * 60);
  overflow-y: auto;
}

.photo-view-body {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  background: var(--border);
}

.full-photo {
  max-width: 100%;
  max-height: calc(var(--app-vh, 1vh) * 70);
  object-fit: contain;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 16px;
}

.input-label {
  font-size: 0.85em;
  font-weight: 500;
  color: var(--text);
  margin-bottom: 2px;
}

.modal-input,
.modal-textarea {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 0.9em;
  transition: border-color 0.2s ease;
  background: var(--surface);
  font-family: inherit;
}

.modal-input:focus,
.modal-textarea:focus {
  border-color: var(--accent);
  outline: none;
}

.modal-textarea {
  resize: vertical;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px 20px;
  border-top: 1px solid var(--border);
}

.modal-btn {
  padding: 8px 20px;
  border: none;
  border-radius: 999px;
  cursor: pointer;
  font-size: 0.85em;
  font-weight: 500;
  transition: background-color 0.2s ease;
  min-width: 80px;
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

.modal-btn--confirm:hover:not(.modal-btn--disabled) {
  background: var(--accent-hover);
}

.modal-btn--disabled {
  background: var(--border);
  cursor: not-allowed;
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

/* Скроллбары */
.table-body::-webkit-scrollbar,
.photos-grid::-webkit-scrollbar,
.modal-body::-webkit-scrollbar {
  width: 6px;
}

.table-body::-webkit-scrollbar-track,
.photos-grid::-webkit-scrollbar-track,
.modal-body::-webkit-scrollbar-track {
  background: var(--surface-2);
  border-radius: 3px;
}

.table-body::-webkit-scrollbar-thumb,
.photos-grid::-webkit-scrollbar-thumb,
.modal-body::-webkit-scrollbar-thumb {
  background: var(--border);
  border-radius: 3px;
}

.table-body::-webkit-scrollbar-thumb:hover,
.photos-grid::-webkit-scrollbar-thumb:hover,
.modal-body::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
}

@media (max-width: 968px) {
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
    border-bottom: 1px solid var(--border);
    height: 255px;
  }
  
  .details-section {
    height: 400px;
  }
  
  .details-title-wrapper {
    flex-direction: column;
    align-items: flex-start;
  }
  
  .map-link-group {
    flex-direction: column;
  }
  
  .map-link-btn {
    width: 100%;
    text-align: center;
  }
  
  .modal-content {
    width: 95%;
    max-height: 80vh;
  }
}

@media (max-width: 767.98px) {
  /* Направление/высоту шапки берёт на себя глобальный .rt-header-inline
     (responsive-tables.css, !important - перебивает scoped-специфичность). */
  .management-header {
    padding: 10px var(--gutter, 16px);
  }

  .header-controls {
    flex-wrap: wrap;
    row-gap: 8px;
  }

  .archive-dropdown {
    min-width: 92px;
  }

  :deep(.search) {
    width: 110px;
  }

  /* Bulk-панель на мобилке - в потоке (не оверлей поверх шапки), кнопки
     переносятся, чекбокс-колонка держит тач-таргет 44px. */
  .bulk-bar {
    position: static;
    height: auto;
    padding: 12px 16px;
    overflow-x: visible;
  }

  .bulk-actions {
    flex-wrap: wrap;
  }

  .check-col {
    min-height: 44px;
  }
}
</style>