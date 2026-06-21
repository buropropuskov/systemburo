<template>
  <section class="admin-settings">
    <header class="admin-settings__header">
      <h2 class="admin-settings__title">
        Настройки системы
      </h2>
    </header>

    <div class="admin-settings__layout">
      <nav
        class="admin-settings__sidebar"
        role="navigation"
        aria-label="Разделы настроек"
      >
        <div
          class="sidebar-item"
          :class="{ 'sidebar-item--active': activeSection === 'upload' }"
          role="button"
          tabindex="0"
          @click="activeSection = 'upload'"
          @keydown.enter="activeSection = 'upload'"
        >
          Загрузка файлов
        </div>
        <div
          class="sidebar-item"
          :class="{ 'sidebar-item--active': activeSection === 'pagination' }"
          role="button"
          tabindex="0"
          @click="activeSection = 'pagination'"
          @keydown.enter="activeSection = 'pagination'"
        >
          Пагинация
        </div>
        <div
          class="sidebar-item"
          :class="{ 'sidebar-item--active': activeSection === 'notifications' }"
          role="button"
          tabindex="0"
          @click="activeSection = 'notifications'"
          @keydown.enter="activeSection = 'notifications'"
        >
          Уведомления
        </div>
        <div
          class="sidebar-item"
          :class="{ 'sidebar-item--active': activeSection === 'security' }"
          role="button"
          tabindex="0"
          @click="activeSection = 'security'"
          @keydown.enter="activeSection = 'security'"
        >
          Безопасность
        </div>
      </nav>

      <div class="admin-settings__content">
        <SkeletonTransition :loading="loading">
          <template #skeleton>
            <div style="display: flex; flex-direction: column; gap: 16px;">
              <SkeletonLine
                width="40%"
                height="16px"
              />
              <SkeletonBlock
                height="40px"
                radius="var(--radius-sm)"
              />
              <SkeletonBlock
                height="80px"
                radius="var(--radius-sm)"
              />
              <SkeletonBlock
                height="80px"
                radius="var(--radius-sm)"
              />
              <SkeletonLine
                width="120px"
                height="36px"
              />
            </div>
          </template>

          <div
            v-if="loadError"
            class="error-state"
          >
            <p class="error-message">
              {{ loadError }}
            </p>
            <button
              class="btn btn--primary"
              @click="fetchSettings"
            >
              Повторить
            </button>
          </div>

          <!-- Загрузка файлов -->
          <div
            v-else-if="activeSection === 'upload'"
            class="settings-section"
          >
            <h3 class="section-title">
              Загрузка файлов
            </h3>

            <div class="form-group">
              <label
                class="form-label"
                for="max-file-size"
              >
                Максимальный размер файла: <strong>{{ fileSizeMB }} МБ</strong>
              </label>
              <input
                id="max-file-size"
                type="range"
                class="form-range"
                :min="1"
                :max="50"
                :step="1"
                :value="fileSizeMB"
                @input="settings.max_file_size = $event.target.value * 1024 * 1024"
              >
              <div class="range-labels">
                <span>1 МБ</span>
                <span>50 МБ</span>
              </div>
            </div>

            <div class="form-group">
              <span class="form-label">Разрешённые типы изображений</span>
              <div class="checkbox-group">
                <label
                  v-for="imgType in availableImageTypes"
                  :key="imgType"
                  class="checkbox-label"
                >
                  <input
                    type="checkbox"
                    :value="imgType"
                    :checked="selectedImageTypes.includes(imgType)"
                    @change="toggleImageType(imgType)"
                  >
                  <span class="checkbox-text">{{ imgType }}</span>
                </label>
              </div>
            </div>

            <div class="form-group">
              <span class="form-label">Разрешённые типы документов</span>
              <div class="checkbox-group">
                <label
                  v-for="docType in availableDocTypes"
                  :key="docType"
                  class="checkbox-label"
                >
                  <input
                    type="checkbox"
                    :value="docType"
                    :checked="selectedDocTypes.includes(docType)"
                    @change="toggleDocType(docType)"
                  >
                  <span class="checkbox-text">{{ docType }}</span>
                </label>
              </div>
            </div>

            <button
              class="btn btn--primary"
              :disabled="saving"
              @click="saveUploadSettings"
            >
              {{ saving ? 'Сохранение...' : 'Сохранить' }}
            </button>
          </div>

          <!-- Пагинация -->
          <div
            v-else-if="activeSection === 'pagination'"
            class="settings-section"
          >
            <h3 class="section-title">
              Пагинация
            </h3>

            <div class="form-group">
              <label
                class="form-label"
                for="max-per-page"
              >
                Записей на странице
              </label>
              <input
                id="max-per-page"
                v-model.number="settings.max_per_page"
                type="number"
                class="form-input"
                :min="10"
                :max="500"
              >
              <span class="form-hint">От 10 до 500</span>
            </div>

            <button
              class="btn btn--primary"
              :disabled="saving"
              @click="savePaginationSettings"
            >
              {{ saving ? 'Сохранение...' : 'Сохранить' }}
            </button>
          </div>

          <!-- Уведомления -->
          <div
            v-else-if="activeSection === 'notifications'"
            class="settings-section"
          >
            <h3 class="section-title">
              Уведомления
            </h3>

            <div class="form-group">
              <label class="switch-label">
                <span class="switch-text">Уведомления включены</span>
                <span
                  class="switch"
                  :class="{ 'switch--on': settings.notifications_enabled }"
                  role="switch"
                  :aria-checked="String(settings.notifications_enabled)"
                  tabindex="0"
                  @click="settings.notifications_enabled = !settings.notifications_enabled"
                  @keydown.enter="settings.notifications_enabled = !settings.notifications_enabled"
                  @keydown.space.prevent="settings.notifications_enabled = !settings.notifications_enabled"
                >
                  <span class="switch__thumb" />
                </span>
              </label>
            </div>

            <div class="form-group">
              <label
                class="form-label"
                for="poll-interval"
              >
                Интервал опроса (секунды)
              </label>
              <input
                id="poll-interval"
                v-model.number="settings.notifications_poll_interval"
                type="number"
                class="form-input"
                :min="10"
                :max="120"
                :disabled="!settings.notifications_enabled"
              >
              <span class="form-hint">От 10 до 120 секунд</span>
            </div>

            <div class="form-group">
              <label
                class="form-label"
                for="del-duration"
              >
                Длительность уведомления об удалении (секунды)
              </label>
              <input
                id="del-duration"
                v-model.number="settings.notifications_delete_duration"
                type="number"
                class="form-input"
                :min="3"
                :max="60"
              >
              <span class="form-hint">От 3 до 60 секунд (машины и люди)</span>
            </div>

            <div class="form-group">
              <label
                class="form-label"
                for="res-duration"
              >
                Длительность уведомления о восстановлении (секунды)
              </label>
              <input
                id="res-duration"
                v-model.number="settings.notifications_restore_duration"
                type="number"
                class="form-input"
                :min="3"
                :max="60"
              >
              <span class="form-hint">От 3 до 60 секунд (машины и люди)</span>
            </div>

            <button
              class="btn btn--primary"
              :disabled="saving"
              @click="saveNotificationSettings"
            >
              {{ saving ? 'Сохранение...' : 'Сохранить' }}
            </button>
          </div>

          <!-- Безопасность: политика паролей -->
          <div
            v-else-if="activeSection === 'security'"
            class="settings-section"
          >
            <h3 class="section-title">
              Требования к паролю
            </h3>

            <div class="form-group">
              <label
                class="form-label"
                for="pw-min-length"
              >
                Минимальная длина
              </label>
              <input
                id="pw-min-length"
                v-model.number="settings.password_min_length"
                type="number"
                class="form-input"
                :min="6"
                :max="128"
              >
              <span class="form-hint">От 6 до 128 символов</span>
            </div>

            <div
              v-for="opt in passwordToggles"
              :key="opt.key"
              class="form-group"
            >
              <label class="switch-label">
                <span class="switch-text">{{ opt.label }}</span>
                <span
                  class="switch"
                  :class="{ 'switch--on': settings[opt.key] }"
                  role="switch"
                  :aria-checked="String(settings[opt.key])"
                  tabindex="0"
                  @click="settings[opt.key] = !settings[opt.key]"
                  @keydown.enter="settings[opt.key] = !settings[opt.key]"
                  @keydown.space.prevent="settings[opt.key] = !settings[opt.key]"
                >
                  <span class="switch__thumb" />
                </span>
              </label>
            </div>

            <button
              class="btn btn--primary"
              :disabled="saving"
              @click="saveSecuritySettings"
            >
              {{ saving ? 'Сохранение...' : 'Сохранить' }}
            </button>
          </div>
        </SkeletonTransition>
      </div>
    </div>

  </section>
</template>

<script>
import { getSettings, updateSetting } from '@/api/settings';
import { SkeletonTransition, SkeletonLine, SkeletonBlock } from '@/components/ui';
import { useDeletionsStore } from '@/stores/deletions';

export default {
  name: 'AdminSettings',
  components: {
    SkeletonTransition,
    SkeletonLine,
    SkeletonBlock,
  },
  data() {
    return {
      activeSection: 'upload',
      loading: false,
      saving: false,
      loadError: null,
      settings: {
        max_file_size: 10 * 1024 * 1024,
        allowed_image_types: '',
        allowed_doc_types: '',
        max_per_page: 50,
        notifications_enabled: false,
        notifications_poll_interval: 30,
        notifications_delete_duration: 10,
        notifications_restore_duration: 5,
        password_min_length: 8,
        password_require_letter: true,
        password_require_uppercase: false,
        password_require_lowercase: false,
        password_require_digit: true,
        password_require_special: false,
      },
      availableImageTypes: ['image/jpeg', 'image/png', 'image/gif', 'image/webp', 'image/svg+xml'],
      availableDocTypes: ['application/pdf', 'application/msword', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'text/plain'],
      passwordToggles: [
        { key: 'password_require_letter', label: 'Требовать букву' },
        { key: 'password_require_uppercase', label: 'Требовать заглавную букву' },
        { key: 'password_require_lowercase', label: 'Требовать строчную букву' },
        { key: 'password_require_digit', label: 'Требовать цифру' },
        { key: 'password_require_special', label: 'Требовать спецсимвол' },
      ],
    };
  },
  computed: {
    fileSizeMB() {
      return Math.round(this.settings.max_file_size / (1024 * 1024));
    },
    selectedImageTypes() {
      if (!this.settings.allowed_image_types) return [];
      try {
        const parsed = JSON.parse(this.settings.allowed_image_types);
        return Array.isArray(parsed) ? parsed : [];
      } catch {
        return this.settings.allowed_image_types.split(',').map(t => t.trim()).filter(Boolean);
      }
    },
    selectedDocTypes() {
      if (!this.settings.allowed_doc_types) return [];
      try {
        const parsed = JSON.parse(this.settings.allowed_doc_types);
        return Array.isArray(parsed) ? parsed : [];
      } catch {
        return this.settings.allowed_doc_types.split(',').map(t => t.trim()).filter(Boolean);
      }
    },
  },
  mounted() {
    this.fetchSettings();
  },
  methods: {
    async fetchSettings() {
      this.loading = true;
      this.loadError = null;
      try {
        const data = await getSettings();
        const settingsArray = Array.isArray(data) ? data : (data.data || []);
        this.mapSettingsFromArray(settingsArray);
      } catch (error) {
        console.error('Ошибка загрузки настроек:', error);
        this.loadError = 'Не удалось загрузить настройки';
      } finally {
        this.loading = false;
      }
    },

    mapSettingsFromArray(arr) {
      for (const item of arr) {
        switch (item.key) {
          case 'upload.max_file_size':
            this.settings.max_file_size = Number(item.value) || 10 * 1024 * 1024;
            break;
          case 'upload.allowed_image_types':
            this.settings.allowed_image_types = item.value || '';
            break;
          case 'upload.allowed_doc_types':
            this.settings.allowed_doc_types = item.value || '';
            break;
          case 'pagination.max_per_page':
            this.settings.max_per_page = Number(item.value) || 50;
            break;
          case 'notifications.enabled':
            this.settings.notifications_enabled = item.value === 'true';
            break;
          case 'notifications.poll_interval':
            this.settings.notifications_poll_interval = Number(item.value) || 30;
            break;
          case 'notifications.delete_duration':
            this.settings.notifications_delete_duration = Number(item.value) || 10;
            break;
          case 'notifications.restore_duration':
            this.settings.notifications_restore_duration = Number(item.value) || 5;
            break;
          case 'password.min_length':
            this.settings.password_min_length = Number(item.value) || 8;
            break;
          case 'password.require_letter':
            this.settings.password_require_letter = item.value === 'true';
            break;
          case 'password.require_uppercase':
            this.settings.password_require_uppercase = item.value === 'true';
            break;
          case 'password.require_lowercase':
            this.settings.password_require_lowercase = item.value === 'true';
            break;
          case 'password.require_digit':
            this.settings.password_require_digit = item.value === 'true';
            break;
          case 'password.require_special':
            this.settings.password_require_special = item.value === 'true';
            break;
        }
      }
    },

    toggleImageType(type) {
      const types = this.selectedImageTypes.slice();
      const idx = types.indexOf(type);
      if (idx >= 0) {
        types.splice(idx, 1);
      } else {
        types.push(type);
      }
      this.settings.allowed_image_types = JSON.stringify(types);
    },

    toggleDocType(type) {
      const types = this.selectedDocTypes.slice();
      const idx = types.indexOf(type);
      if (idx >= 0) {
        types.splice(idx, 1);
      } else {
        types.push(type);
      }
      this.settings.allowed_doc_types = JSON.stringify(types);
    },

    async saveUploadSettings() {
      this.saving = true;
      try {
        await updateSetting('upload.max_file_size', String(this.settings.max_file_size));
        await updateSetting('upload.allowed_image_types', JSON.stringify(this.selectedImageTypes));
        await updateSetting('upload.allowed_doc_types', JSON.stringify(this.selectedDocTypes));
        useDeletionsStore().notify({ prefix: 'Настройки загрузки сохранены' });
      } catch (error) {
        console.error('Ошибка сохранения:', error);
        useDeletionsStore().notify({ prefix: 'Ошибка сохранения настроек', type: 'error' });
      } finally {
        this.saving = false;
      }
    },

    async savePaginationSettings() {
      const value = this.settings.max_per_page;
      if (value < 10 || value > 500) {
        useDeletionsStore().notify({ prefix: 'Значение должно быть от 10 до 500', type: 'error' });
        return;
      }
      this.saving = true;
      try {
        await updateSetting('pagination.max_per_page', String(value));
        useDeletionsStore().notify({ prefix: 'Настройки пагинации сохранены' });
      } catch (error) {
        console.error('Ошибка сохранения:', error);
        useDeletionsStore().notify({ prefix: 'Ошибка сохранения настроек', type: 'error' });
      } finally {
        this.saving = false;
      }
    },

    async saveNotificationSettings() {
      const interval = this.settings.notifications_poll_interval;
      if (interval < 10 || interval > 120) {
        useDeletionsStore().notify({ prefix: 'Интервал должен быть от 10 до 120 секунд', type: 'error' });
        return;
      }
      const del = this.settings.notifications_delete_duration;
      const res = this.settings.notifications_restore_duration;
      if (del < 3 || del > 60 || res < 3 || res > 60) {
        useDeletionsStore().notify({ prefix: 'Длительность уведомлений: от 3 до 60 секунд', type: 'error' });
        return;
      }
      this.saving = true;
      try {
        await updateSetting('notifications.enabled', String(this.settings.notifications_enabled));
        await updateSetting('notifications.poll_interval', String(interval));
        await updateSetting('notifications.delete_duration', String(del));
        await updateSetting('notifications.restore_duration', String(res));
        // Сразу применяем к стору уведомлений, иначе новые длительности
        // вступят в силу только после полной перезагрузки страницы.
        useDeletionsStore().setDurations(del, res);
        useDeletionsStore().notify({ prefix: 'Настройки уведомлений сохранены' });
      } catch (error) {
        console.error('Ошибка сохранения:', error);
        useDeletionsStore().notify({ prefix: 'Ошибка сохранения настроек', type: 'error' });
      } finally {
        this.saving = false;
      }
    },

    async saveSecuritySettings() {
      const len = this.settings.password_min_length;
      if (len < 6 || len > 128) {
        useDeletionsStore().notify({ prefix: 'Минимальная длина: от 6 до 128', type: 'error' });
        return;
      }
      this.saving = true;
      try {
        await updateSetting('password.min_length', String(len));
        await updateSetting('password.require_letter', String(this.settings.password_require_letter));
        await updateSetting('password.require_uppercase', String(this.settings.password_require_uppercase));
        await updateSetting('password.require_lowercase', String(this.settings.password_require_lowercase));
        await updateSetting('password.require_digit', String(this.settings.password_require_digit));
        await updateSetting('password.require_special', String(this.settings.password_require_special));
        useDeletionsStore().notify({ prefix: 'Требования к паролю сохранены' });
      } catch (error) {
        console.error('Ошибка сохранения:', error);
        useDeletionsStore().notify({ prefix: 'Ошибка сохранения настроек', type: 'error' });
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
.admin-settings {
  padding: 12px;
}

.admin-settings__header {
  padding-bottom: 8px;
  margin-bottom: 12px;
}

.admin-settings__title {
  font-size: 16px;
  font-weight: 600;
  color: #000;
  margin: 0;
}

.admin-settings__layout {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.admin-settings__sidebar {
  width: 200px;
  flex-shrink: 0;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  overflow: hidden;
}

.sidebar-item {
  padding: 10px 16px;
  font-size: 13px;
  font-weight: 500;
  color: #333;
  cursor: pointer;
  transition: all 0.2s ease;
  border-left: 3px solid transparent;
  font-family: 'Montserrat', sans-serif;
}

.sidebar-item:hover {
  background: #f5f5f5;
}

.sidebar-item--active {
  background: #f0f1ff;
  color: #4F5BDF;
  border-left-color: #4F5BDF;
  font-weight: 600;
}

.sidebar-item + .sidebar-item {
  border-top: 1px solid #f0f0f0;
}

.admin-settings__content {
  flex: 1;
  min-width: 0;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  padding: 16px 20px;
}

.settings-section {
  animation: fadeIn 0.2s ease;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: #333;
  margin: 0 0 16px 0;
  padding-bottom: 8px;
  border-bottom: 1px solid #e6e6e6;
}

.form-group {
  margin-bottom: 16px;
}

.form-label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: #333;
  margin-bottom: 6px;
  font-family: 'Montserrat', sans-serif;
}

.form-input {
  width: 140px;
  padding: 6px 10px;
  border: 1px solid #e6e6e6;
  border-radius: var(--radius-md);
  font-size: 13px;
  font-family: 'Montserrat', sans-serif;
  color: #333;
  transition: border-color 0.2s;
  outline: none;
}

.form-input:focus {
  border-color: #4F5BDF;
}

.form-input:disabled {
  background: #f9f9f9;
  color: #999;
  cursor: not-allowed;
}

.form-hint {
  display: block;
  font-size: 11px;
  color: #888;
  margin-top: 4px;
}

.form-range {
  width: 100%;
  max-width: 300px;
  margin-top: 4px;
  accent-color: #4F5BDF;
  cursor: pointer;
}

.range-labels {
  display: flex;
  justify-content: space-between;
  max-width: 300px;
  font-size: 11px;
  color: #888;
  margin-top: 2px;
}

.checkbox-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  padding: 4px 10px;
  border: 1px solid #e6e6e6;
  border-radius: 50px;
  transition: all 0.2s;
  font-size: 12px;
}

.checkbox-label:hover {
  border-color: #4F5BDF;
  background: #fafaff;
}

.checkbox-label input[type="checkbox"] {
  accent-color: #4F5BDF;
  cursor: pointer;
}

.checkbox-text {
  font-family: 'Montserrat', sans-serif;
  font-size: 12px;
  color: #333;
  white-space: nowrap;
}

/* Switch */
.switch-label {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
}

.switch-text {
  font-size: 12px;
  font-weight: 500;
  color: #333;
  font-family: 'Montserrat', sans-serif;
}

.switch {
  position: relative;
  width: 40px;
  height: 22px;
  background: #ccc;
  border-radius: 11px;
  transition: background 0.2s ease;
  flex-shrink: 0;
  outline: none;
}

.switch:focus-visible {
  box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.3);
}

.switch--on {
  background: #4F5BDF;
}

.switch__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 18px;
  height: 18px;
  background: #fff;
  border-radius: 50%;
  transition: transform 0.2s ease;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
}

.switch--on .switch__thumb {
  transform: translateX(18px);
}

/* Кнопки */
.btn {
  padding: 6px 20px;
  border: 1px solid transparent;
  border-radius: 50px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  font-family: 'Montserrat', sans-serif;
  transition: all 0.2s;
  height: 32px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 100px;
}

.btn--primary {
  background: #4F5BDF;
  color: #fff;
  border-color: #4F5BDF;
}

.btn--primary:hover:not(:disabled) {
  background: #3d49c7;
}

.btn--primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Состояния загрузки и ошибки */
.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.spinner {
  width: 24px;
  height: 24px;
  border: 2px solid #e6e6e6;
  border-top-color: #4F5BDF;
  border-radius: 50%;
  animation: spinner-rotate 1s linear infinite;
}

@keyframes spinner-rotate {
  to { transform: rotate(360deg); }
}

.error-state {
  text-align: center;
  padding: 40px 20px;
}

.error-message {
  color: #dc3545;
  font-size: 13px;
  margin: 0 0 12px 0;
}

@media (max-width: 768px) {
  .admin-settings__layout {
    flex-direction: column;
  }

  .admin-settings__sidebar {
    width: 100%;
    display: flex;
    border-radius: 50px;
  }

  .sidebar-item {
    flex: 1;
    text-align: center;
    border-left: none;
    border-bottom: 3px solid transparent;
    padding: 8px 12px;
    font-size: 12px;
  }

  .sidebar-item--active {
    border-left-color: transparent;
    border-bottom-color: #4F5BDF;
  }

  .sidebar-item + .sidebar-item {
    border-top: none;
    border-left: 1px solid #f0f0f0;
  }

  .admin-settings__content {
    border-radius: 20px;
  }

  .form-range {
    max-width: 100%;
  }

  .range-labels {
    max-width: 100%;
  }

  /* Длинные MIME-типы (application/vnd.openxmlformats...) переполняли flex-row */
  .checkbox-label {
    max-width: 100%;
  }

  .checkbox-text {
    white-space: normal;
    overflow-wrap: anywhere;
    word-break: break-word;
  }
}

@media (max-width: 480px) {
  .admin-settings__content {
    padding: 16px;
  }

  .sidebar-item {
    font-size: 11px;
    padding: 8px 6px;
  }
}
</style>
