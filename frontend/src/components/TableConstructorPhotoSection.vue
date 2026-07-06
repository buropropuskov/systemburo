<template>
  <div class="location-section">
    <div class="photos-header">
      <h4 class="section-title">
        Фотографии места
      </h4>
      <label class="upload-photo-btn">
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

    <!-- Drag&drop zone - можно перетащить файлы из проводника или кликнуть. -->
    <label
      class="photo-dropzone"
      :class="{ 'photo-dropzone--active': isDragging }"
      @dragenter.prevent="isDragging = true"
      @dragover.prevent="isDragging = true"
      @dragleave.prevent="isDragging = false"
      @drop.prevent="onDrop"
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
        v-for="photo in photos"
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
        <div class="photo-actions">
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
            <img
              src="@/assets/icons/trashcan.png"
              class="action-icon-small"
            >
          </button>
        </div>
      </div>
      <div
        v-if="!photos || photos.length === 0"
        class="no-photos"
      >
        <p>Фотографии не загружены</p>
      </div>
    </div>

    <!-- Модальное окно просмотра фото -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showPhotoModal"
          class="modal-overlay"
          @click.self="showPhotoModal = false"
        >
          <div class="modal-content photo-view-modal">
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
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useDeletionsStore } from '@/stores/deletions'
import { useUiStore } from '@/stores/ui'

export default {
  name: 'TableConstructorPhotoSection',
  props: {
    tableId: { type: Number, required: true },
    photos: { type: Array, default: () => [] }
  },
  emits: ['photos-changed'],
  data() {
    return {
      showPhotoModal: false,
      viewingPhoto: null,
      isDragging: false,
    };
  },
  methods: {
    async uploadPhotos(event) {
      const files = event.target.files;
      if (!files || files.length === 0) return;
      await this.uploadFiles(files);
      // Сбрасываем input, чтобы можно было повторно выбрать те же файлы.
      event.target.value = '';
    },

    onDrop(event) {
      this.isDragging = false;
      const files = event.dataTransfer?.files;
      if (files && files.length) this.uploadFiles(files);
    },

    /**
     * Достаёт текст ошибки бэка из ответа apiRequest. wrapJsonUnwrap на !success
     * кладёт его в message (в envelope ключ - error); сырой response.text() дал бы
     * JSON целиком в уведомление.
     * @param {Response} response
     * @param {string} fallback
     * @returns {Promise<string>}
     */
    async errorMessage(response, fallback) {
      try {
        const body = await response.json();
        return (body && body.message) || fallback;
      } catch {
        return fallback;
      }
    },

    async uploadFiles(files) {
      const formData = new FormData();
      for (let i = 0; i < files.length; i++) {
        // Принимаем только изображения - drag из проводника может тащить любое.
        if (!files[i].type || files[i].type.startsWith('image/')) {
          formData.append('photos', files[i]);
        }
      }
      try {
        const response = await apiRequest(`/system-tables/${this.tableId}/photos`, {
          method: 'POST',
          body: formData,
          headers: {},
        });
        if (response.ok) {
          this.$emit('photos-changed');
          this.dispatchNotification('Фотографии успешно загружены', 'success');
        } else {
          this.dispatchNotification(await this.errorMessage(response, 'Ошибка при загрузке фото'), 'error');
        }
      } catch (error) {
        console.error('Error uploading photos:', error);
        this.dispatchNotification('Ошибка сети', 'error');
      }
    },

    async deletePhoto(photo) {
      const ok = await useUiStore().confirm({
        title: 'Удалить фотографию?',
        message: `Фотография «${photo.file_name}» будет удалена без возможности восстановления.`,
        confirmText: 'Удалить',
        cancelText: 'Отмена',
        danger: true,
      });
      if (!ok) return;

      try {
        const response = await apiRequest(`/system-tables/${this.tableId}/photos/${photo.id}`, {
          method: 'DELETE',
        });

        if (response.ok) {
          this.$emit('photos-changed');
          this.dispatchNotification('Фотография удалена', 'success');
        } else {
          this.dispatchNotification(await this.errorMessage(response, 'Ошибка при удалении фото'), 'error');
        }
      } catch (error) {
        console.error('Error deleting photo:', error);
        this.dispatchNotification('Ошибка сети', 'error');
      }
    },

    async setMainPhoto(photo) {
      try {
        const response = await apiRequest(`/system-tables/${this.tableId}/photos/${photo.id}/main`, {
          method: 'POST',
        });

        if (response.ok) {
          this.$emit('photos-changed');
          this.dispatchNotification('Главная фотография установлена', 'success');
        } else {
          this.dispatchNotification(await this.errorMessage(response, 'Ошибка при установке главной фотографии'), 'error');
        }
      } catch (error) {
        console.error('Error setting main photo:', error);
        this.dispatchNotification('Ошибка сети', 'error');
      }
    },

    viewPhoto(photo) {
      this.viewingPhoto = photo;
      this.showPhotoModal = true;
    },

    dispatchNotification(message, type) {
      const opts = type === 'error'
        ? { prefix: '', bold: message, type: 'error' }
        : { prefix: '', bold: message };
      useDeletionsStore().notify(opts);
    }
  }
}
</script>

<style scoped>
.location-section {
  margin-bottom: 24px;
}

.section-title {
  margin: 0 0 12px 0;
  font-size: 14px;
  font-weight: 600;
  color: #333;
}

.photos-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.upload-photo-btn {
  padding: 4px 12px;
  background: #f0f3ff;
  color: #4F5BDF;
  border: 1px solid #4F5BDF;
  border-radius: 20px;
  cursor: pointer;
  font-size: 12px;
  transition: background-color 0.2s ease;
}

.upload-photo-btn:hover {
  background: #4F5BDF;
  color: white;
}

/* Drop-zone для перетаскивания файлов из проводника. Также служит обычной
   кнопкой обзора (label + input file). */
.photo-dropzone {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 20px;
  margin-bottom: 12px;
  border: 2px dashed #c0c4d8;
  border-radius: 50px;
  background: #fafbff;
  color: #6b7280;
  cursor: pointer;
  text-align: center;
  transition: border-color 0.2s ease, background 0.2s ease, color 0.2s ease;
}

.photo-dropzone:hover {
  border-color: #4F5BDF;
  background: #f0f3ff;
  color: #4F5BDF;
}

.photo-dropzone--active {
  border-color: #4F5BDF;
  background: #e6ebff;
  color: #4F5BDF;
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
  color: #333;
  font-weight: 600;
}

.photo-dropzone:hover .photo-dropzone__text strong,
.photo-dropzone--active .photo-dropzone__text strong {
  color: #4F5BDF;
}

.photo-dropzone__text span {
  font-size: 11px;
}

.photos-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 12px;
  max-height: 250px;
  overflow-y: auto;
  padding: 4px;
}

.photos-grid::-webkit-scrollbar {
  width: 6px;
}

.photos-grid::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 3px;
}

.photos-grid::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 3px;
}

.photos-grid::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

.photo-item {
  position: relative;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  overflow: hidden;
  aspect-ratio: 1;
  background: #f8f9fa;
}

.photo-item.main-photo {
  border: 2px solid #4F5BDF;
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
  background: #4F5BDF;
  color: white;
}

.photo-main-badge {
  background: #4F5BDF;
  color: white;
  cursor: default;
}

.photo-delete-btn:hover {
  background: #c62828;
}

.photo-delete-btn:hover .action-icon-small {
  filter: brightness(0) invert(1);
}

.action-icon-small {
  width: 14px;
  height: 14px;
}

.no-photos {
  grid-column: 1 / -1;
  text-align: center;
  padding: 20px;
  color: #a2a2a2;
  background: #f8f9fa;
  border: 1px dashed #e6e6e6;
  border-radius: 25px;
  font-size: 15px;
}

/* Модальное окно */
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
  z-index: 10000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  animation: overlayAppear 0.3s ease-out;
}

@keyframes overlayAppear {
  from {
    background: rgba(0, 0, 0, 0);
    backdrop-filter: blur(0px);
  }
  to {
    background: rgba(0, 0, 0, 0.5);
    backdrop-filter: blur(0.1px);
  }
}

.modal-content {
  background: #fff;
  border-radius: 12px;
  padding: 0;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
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
  padding: 16px 20px;
  border-bottom: 1px solid #f0f0f0;
}

.modal-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #333;
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
  background-color: #f5f5f5;
}

.photo-view-body {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  background: #f0f0f0;
}

.full-photo {
  max-width: 100%;
  max-height: 70vh;
  object-fit: contain;
}

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
  background: rgba(0, 0, 0, 0);
  backdrop-filter: blur(0px);
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
  opacity: 0;
  transform: scale(0.8) translateY(-20px);
}
</style>
