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
          v-for="section in sections"
          :key="section.value"
          class="sidebar-item"
          :class="{ 'sidebar-item--active': activeSection === section.value }"
          role="button"
          tabindex="0"
          @click="activeSection = section.value"
          @keydown.enter="activeSection = section.value"
        >
          {{ section.label }}
        </div>
      </nav>

      <div class="admin-settings__section-select">
        <BaseDropdown
          :model-value="activeSection"
          :options="sections"
          label-key="label"
          value-key="value"
          aria-label="Раздел настроек"
          @update:model-value="activeSection = $event"
        />
      </div>

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

          <!-- Информация Бюро -->
          <div
            v-else-if="activeSection === 'contacts'"
            class="settings-section"
          >
            <h3 class="section-title">
              Информация Бюро
            </h3>

            <h4 class="subsection-title">
              Контакты
            </h4>
            <p class="form-hint section-intro">
              Телефон и почта Бюро. Показываются на странице входа и в плашке блокировки.
            </p>

            <div class="form-group">
              <label
                class="form-label"
                for="bureau-phone"
              >
                Телефон
              </label>
              <input
                id="bureau-phone"
                v-model="settings.bureau_phone"
                type="text"
                class="form-input"
                placeholder="+7 (495) 123-45-67"
                maxlength="30"
              >
            </div>

            <div class="form-group">
              <label
                class="form-label"
                for="bureau-email"
              >
                Электронная почта
              </label>
              <input
                id="bureau-email"
                v-model="settings.bureau_email"
                type="email"
                class="form-input"
                placeholder="bureau@example.com"
              >
            </div>

            <button
              class="btn btn--primary"
              :disabled="saving"
              @click="saveContactsSettings"
            >
              {{ saving ? 'Сохранение...' : 'Сохранить' }}
            </button>

            <div class="bureau-schedule">
              <p
                v-if="bureauSlotsLoading && !bureauSlotsLoaded"
                class="form-hint"
              >
                Загрузка расписания...
              </p>
              <WorkScheduleTab
                v-else
                resource-url="/bureau"
                :time-slots="bureauTimeSlots"
                @update="fetchBureauSchedule"
              />
            </div>
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

          <!-- Обработка данных -->
          <div
            v-else-if="activeSection === 'data-processing'"
            class="settings-section"
          >
            <h3 class="section-title">
              Обработка данных
            </h3>
            <p class="dp-desc">
              Документ открывается по ссылке «согласие» при подаче заявки и доступен по адресу
              <code>/data-processing</code>. PDF показывается прямо на странице, DOC и DOCX —
              только для скачивания.
            </p>

            <p
              v-if="dpLoading"
              class="form-hint"
            >
              Загрузка...
            </p>

            <div
              v-else-if="dpMeta"
              class="dp-card"
            >
              <div class="dp-card__info">
                <span class="dp-card__name">{{ dpMeta.file_name }}</span>
                <span class="dp-card__meta">{{ dpExtLabel }} · загружен {{ dpUploadedLabel }}</span>
              </div>
              <div class="dp-card__actions">
                <a
                  class="btn btn--ghost"
                  href="/data-processing"
                  target="_blank"
                  rel="noopener"
                >Открыть</a>
                <button
                  class="btn btn--ghost"
                  :disabled="dpBusy"
                  @click="downloadDp"
                >
                  Скачать
                </button>
                <label
                  class="btn btn--ghost"
                  :class="{ 'btn--disabled': dpBusy }"
                >
                  {{ dpUploading ? 'Загрузка...' : 'Заменить' }}
                  <input
                    type="file"
                    accept=".pdf,.doc,.docx"
                    hidden
                    :disabled="dpBusy"
                    @change="onDpFileChange"
                  >
                </label>
                <button
                  class="btn btn--danger"
                  :disabled="dpBusy"
                  @click="deleteDp"
                >
                  {{ dpDeleting ? 'Удаление...' : 'Удалить' }}
                </button>
              </div>
            </div>

            <div
              v-else
              class="dp-upload"
            >
              <label
                class="btn btn--primary"
                :class="{ 'btn--disabled': dpUploading }"
              >
                {{ dpUploading ? 'Загрузка...' : 'Загрузить документ' }}
                <input
                  type="file"
                  accept=".pdf,.doc,.docx"
                  hidden
                  :disabled="dpUploading"
                  @change="onDpFileChange"
                >
              </label>
              <span class="form-hint">PDF, DOC или DOCX.</span>
            </div>

            <h4 class="subsection-title pdc-title">
              Текст согласия при первом входе
            </h4>
            <p class="dp-desc">
              Этот текст показывается пользователю в окне при первом входе. Кнопку подтверждения
              он сможет нажать только прокрутив текст до конца. Кнопка «Извлечь из документа»
              переносит сюда текст загруженного файла: извлечение не воспроизводит таблицы,
              теряет автонумерацию Word и путает переносы по слогам, поэтому результат нужно
              вычитать.
            </p>

            <p
              v-if="pdcLoading"
              class="form-hint"
            >
              Загрузка...
            </p>

            <template v-else>
              <div
                v-if="pdcRequired && !pdcHasText"
                class="pdc-warning"
                data-testid="pdc-empty-warning"
              >
                Запрос согласия включён, но текст пуст, поэтому согласие у пользователей не
                спрашивается. Задайте текст или выключите запрос.
              </div>

              <div class="pdc-actions">
                <button
                  class="btn btn--ghost"
                  :disabled="!dpMeta || pdcBusy"
                  data-testid="pdc-extract"
                  @click="extractPdcText"
                >
                  {{ pdcExtracting ? 'Извлечение...' : 'Извлечь из документа' }}
                </button>
                <span
                  v-if="!dpMeta"
                  class="form-hint"
                >Сначала загрузите документ</span>
              </div>

              <TextConstructor
                v-model="pdcText"
                class="pdc-editor"
                :rows="12"
                placeholder="Текст согласия на обработку персональных данных"
              />

              <div class="pdc-actions">
                <button
                  class="btn btn--primary"
                  :disabled="pdcBusy"
                  data-testid="pdc-save"
                  @click="savePdcText"
                >
                  {{ pdcSaving ? 'Сохранение...' : 'Сохранить текст' }}
                </button>
                <span class="form-hint">Редакция {{ pdcVersion }}</span>
              </div>

              <div class="form-group">
                <label class="switch-label">
                  <span class="switch-text">Запрашивать согласие при первом входе</span>
                  <span
                    class="switch"
                    :class="{ 'switch--on': pdcRequired }"
                    role="switch"
                    :aria-checked="String(pdcRequired)"
                    tabindex="0"
                    data-testid="pdc-required-switch"
                    @click="togglePdcRequired"
                    @keydown.enter="togglePdcRequired"
                    @keydown.space.prevent="togglePdcRequired"
                  >
                    <span class="switch__thumb" />
                  </span>
                </label>
                <span
                  class="form-hint"
                  data-testid="pdc-required-hint"
                >{{ pdcRequiredHint }}</span>
              </div>

              <div class="pdc-actions">
                <button
                  class="btn btn--ghost"
                  :disabled="pdcBusy"
                  data-testid="pdc-require-again"
                  @click="requirePdcAgain"
                >
                  {{ pdcRequiringAgain ? 'Обновление...' : 'Требовать согласие заново' }}
                </button>
                <span class="form-hint">
                  Поднимает редакцию: все, кто согласился с прежней, подтвердят снова.
                </span>
              </div>

              <!-- Сбор согласий: сколько человек подтвердили текущую редакцию.
                   Считается той же меркой, что и гейт, поэтому число согласившихся
                   совпадает с числом тех, кого система пускает. -->
              <div
                v-if="pdcCollection"
                class="pdc-collection"
                data-testid="pdc-collection"
              >
                <div class="pdc-collection__head">
                  <h5 class="pdc-collection__title">
                    Сбор согласий по редакции {{ pdcCollection.version }}
                  </h5>
                  <RefreshButton
                    :loading="pdcCollectionLoading"
                    data-testid="pdc-collection-refresh"
                    @refresh="fetchPdcCollection"
                  />
                </div>

                <p
                  v-if="!pdcCollection.active"
                  class="form-hint"
                  data-testid="pdc-collection-inactive"
                >
                  Запрос согласия сейчас не действует, сбор не идёт. Подтвердить согласие
                  пользователи смогут после включения.
                </p>

                <template v-else>
                <div
                  class="pdc-collection__bar"
                  role="progressbar"
                  aria-label="Доля подтвердивших согласие"
                  aria-valuemin="0"
                  aria-valuemax="100"
                  :aria-valuenow="pdcCollectionPercent"
                >
                  <div
                    class="pdc-collection__fill"
                    :style="{ transform: `scaleX(${pdcCollectionRatio})` }"
                  />
                </div>

                <p
                  class="pdc-collection__counts"
                  data-testid="pdc-collection-counts"
                >
                  Подтвердили {{ pdcCollection.accepted }} из {{ pdcCollection.total }}
                  ({{ pdcCollectionPercent }}%)
                </p>

                <template v-if="pdcCollection.pending_users.length">
                  <p class="form-hint">
                    Ещё не подтвердили ({{ pdcCollection.pending }}):
                  </p>
                  <ul
                    class="pdc-collection__list"
                    data-testid="pdc-collection-pending"
                  >
                    <li
                      v-for="person in pdcCollection.pending_users"
                      :key="person.id"
                      class="pdc-collection__item"
                    >
                      <span class="pdc-collection__name">{{ person.full_name }}</span>
                      <span class="pdc-collection__login">{{ person.username }}</span>
                      <span
                        v-if="person.organization"
                        class="pdc-collection__org"
                      >{{ person.organization }}</span>
                    </li>
                  </ul>
                  <button
                    class="btn btn--ghost"
                    data-testid="pdc-collection-export"
                    @click="exportPdcPending"
                  >
                    Выгрузить список
                  </button>
                </template>
                <p
                  v-else
                  class="form-hint"
                >
                  Подтвердили все, кого закрывает запрос согласия.
                </p>
                </template>
              </div>
            </template>
          </div>

          <!-- Напоминания согласующим -->
          <div
            v-else-if="activeSection === 'approvals'"
            class="settings-section"
          >
            <h3 class="section-title">
              Напоминания согласующим
            </h3>

            <p class="form-hint section-intro">
              Согласующему приходит уведомление, если он молчит по заявке. Напоминания
              останавливаются, как только его голос перестаёт быть нужным.
            </p>

            <div class="form-group">
              <label class="switch-label">
                <span class="switch-text">Рассылка напоминаний включена</span>
                <span
                  class="switch"
                  :class="{ 'switch--on': settings.approval_reminder_enabled }"
                  role="switch"
                  :aria-checked="String(settings.approval_reminder_enabled)"
                  tabindex="0"
                  @click="settings.approval_reminder_enabled = !settings.approval_reminder_enabled"
                  @keydown.enter="settings.approval_reminder_enabled = !settings.approval_reminder_enabled"
                  @keydown.space.prevent="settings.approval_reminder_enabled = !settings.approval_reminder_enabled"
                >
                  <span class="switch__thumb" />
                </span>
              </label>
            </div>

            <div class="form-group">
              <label
                class="form-label"
                for="reminder-first-days"
              >
                Первое напоминание через (дни)
              </label>
              <input
                id="reminder-first-days"
                v-model.number="settings.approval_reminder_first_days"
                type="number"
                class="form-input"
                :min="1"
                :max="30"
                :disabled="!settings.approval_reminder_enabled"
              >
              <span class="form-hint">От 1 до 30 дней после назначения согласующего</span>
            </div>

            <div class="form-group">
              <label
                class="form-label"
                for="reminder-repeat-days"
              >
                Повтор напоминания каждые (дни)
              </label>
              <input
                id="reminder-repeat-days"
                v-model.number="settings.approval_reminder_repeat_days"
                type="number"
                class="form-input"
                :min="1"
                :max="30"
                :disabled="!settings.approval_reminder_enabled"
              >
              <span class="form-hint">От 1 до 30 дней между повторными напоминаниями</span>
            </div>

            <button
              class="btn btn--primary"
              :disabled="saving"
              @click="saveApprovalSettings"
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
import {
  getDataProcessingMeta,
  uploadDataProcessingDoc,
  deleteDataProcessingDoc,
  downloadDataProcessingDoc,
  fetchDataProcessingBlob,
} from '@/api/dataProcessing';
import {
  getPDConsentSettings,
  savePDConsentText,
  setPDConsentRequired,
  requirePDConsentAgain,
  getPDConsentCollection,
} from '@/api/pdConsent';
import { extractDocumentHtml } from '@/utils/documentTextExtract';
import { stripHtml } from '@/utils/sanitize';
import { SkeletonTransition, SkeletonLine, SkeletonBlock } from '@/components/ui';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import TextConstructor from '@/components/TextConstructor.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';
import { useContactsStore } from '@/stores/contacts';
import { apiRequest } from '@/api/client';
import WorkScheduleTab from '@/components/WorkScheduleTab.vue';

export default {
  name: 'AdminSettings',
  components: {
    SkeletonTransition,
    SkeletonLine,
    SkeletonBlock,
    WorkScheduleTab,
    BaseDropdown,
    TextConstructor,
    RefreshButton,
  },
  data() {
    return {
      activeSection: 'upload',
      sections: [
        { value: 'upload', label: 'Загрузка файлов' },
        { value: 'pagination', label: 'Пагинация' },
        { value: 'notifications', label: 'Уведомления' },
        { value: 'security', label: 'Безопасность' },
        { value: 'data-processing', label: 'Обработка данных' },
        { value: 'contacts', label: 'Информация Бюро' },
        { value: 'approvals', label: 'Напоминания' },
      ],
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
        bureau_phone: '',
        bureau_email: '',
        approval_reminder_enabled: true,
        approval_reminder_first_days: 3,
        approval_reminder_repeat_days: 3,
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
      dpMeta: null,
      dpLoaded: false,
      dpLoading: false,
      dpUploading: false,
      dpDeleting: false,
      pdcText: '',
      pdcLoaded: false,
      pdcVersion: 1,
      pdcRequired: false,
      pdcLoading: false,
      pdcSaving: false,
      pdcExtracting: false,
      pdcRequiringAgain: false,
      pdcTogglingRequired: false,
      // Сводка сбора согласий: грузится вместе с секцией и после каждого действия,
      // которое меняет состав согласившихся (подъём редакции).
      pdcCollection: null,
      pdcCollectionLoading: false,
      bureauTimeSlots: [],
      bureauSlotsLoaded: false,
      bureauSlotsLoading: false,
      bureauSeq: 0,
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
    dpBusy() {
      return this.dpUploading || this.dpDeleting;
    },
    dpExtLabel() {
      const ext = (this.dpMeta?.ext || '').replace('.', '').toUpperCase();
      return ext || 'Документ';
    },
    dpUploadedLabel() {
      if (!this.dpMeta?.uploaded_at) return '';
      const d = new Date(this.dpMeta.uploaded_at);
      if (Number.isNaN(d.getTime())) return '';
      return d.toLocaleDateString('ru-RU');
    },
    /**
     * Подпись под тумблером обязана следовать его состоянию: статичный текст
     * «пока выключено» при включённом запросе читается как вранье.
     */
    pdcRequiredHint() {
      return this.pdcRequired
        ? 'Включено: согласие при входе запрашивается.'
        : 'Выключено: согласие при входе не запрашивается.';
    },
    /** Доля подтвердивших, 0..1. Пустая система - считаем сбор завершённым. */
    pdcCollectionRatio() {
      const c = this.pdcCollection;
      if (!c || !c.total) return 1;
      return Math.min(1, Math.max(0, c.accepted / c.total));
    },
    pdcCollectionPercent() {
      return Math.round(this.pdcCollectionRatio * 100);
    },
    pdcBusy() {
      return this.pdcSaving || this.pdcExtracting || this.pdcRequiringAgain || this.pdcTogglingRequired;
    },
    /**
     * Есть ли в тексте видимое содержимое: редактор на пустом документе отдаёт
     * "<p></p>". Нужно только для предупреждения администратору - запрет включения
     * с пустым текстом держит сервер.
     */
    pdcHasText() {
      const html = this.pdcText || '';
      if (/<img/i.test(html)) return true;
      return stripHtml(html) !== '';
    },
  },
  watch: {
    activeSection(section) {
      if (section === 'data-processing') {
        if (!this.dpLoaded) this.fetchDataProcessingDoc();
        if (!this.pdcLoaded) this.fetchPdConsentSettings();
      }
      if (section === 'contacts' && !this.bureauSlotsLoaded) {
        this.fetchBureauSchedule();
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
          case 'contacts.bureau_phone':
            this.settings.bureau_phone = item.value || '';
            break;
          case 'contacts.bureau_email':
            this.settings.bureau_email = item.value || '';
            break;
          case 'approval.reminder_enabled':
            this.settings.approval_reminder_enabled = item.value === 'true';
            break;
          case 'approval.reminder_first_days':
            this.settings.approval_reminder_first_days = Number(item.value) || 3;
            break;
          case 'approval.reminder_repeat_days':
            this.settings.approval_reminder_repeat_days = Number(item.value) || 3;
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

    async saveContactsSettings() {
      const phone = (this.settings.bureau_phone || '').trim();
      const email = (this.settings.bureau_email || '').trim();
      if (!phone && !email) {
        useDeletionsStore().notify({ prefix: 'Укажите телефон или почту Бюро', type: 'error' });
        return;
      }
      if (email && !/^\S+@\S+\.\S+$/.test(email)) {
        useDeletionsStore().notify({ prefix: 'Некорректный адрес электронной почты', type: 'error' });
        return;
      }
      if (phone && (phone.length < 5 || phone.length > 30)) {
        useDeletionsStore().notify({ prefix: 'Телефон: от 5 до 30 символов', type: 'error' });
        return;
      }
      this.saving = true;
      try {
        if (phone) await updateSetting('contacts.bureau_phone', phone);
        if (email) await updateSetting('contacts.bureau_email', email);
        useContactsStore().fetch(true);
        useDeletionsStore().notify({ prefix: 'Контакты Бюро сохранены' });
      } catch (error) {
        console.error('Ошибка сохранения:', error);
        useDeletionsStore().notify({ prefix: 'Ошибка сохранения настроек', type: 'error' });
      } finally {
        this.saving = false;
      }
    },

    async fetchBureauSchedule() {
      // seq-токен: быстрые @update от тумблеров 24/7 шлют параллельные fetch'и,
      // в стор пишет только последний - иначе устаревший ответ затрёт актуальный.
      const seq = ++this.bureauSeq;
      this.bureauSlotsLoading = true;
      try {
        const response = await apiRequest('/bureau/time-slots');
        if (!response.ok) {
          throw new Error('Не удалось загрузить расписание');
        }
        const data = await response.json();
        if (seq !== this.bureauSeq) return;
        this.bureauTimeSlots = Array.isArray(data) ? data : [];
        this.bureauSlotsLoaded = true;
      } catch (error) {
        if (seq !== this.bureauSeq) return;
        console.error('Ошибка загрузки расписания Бюро:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить расписание Бюро', type: 'error' });
      } finally {
        if (seq === this.bureauSeq) this.bureauSlotsLoading = false;
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

    async saveApprovalSettings() {
      const first = this.settings.approval_reminder_first_days;
      const repeat = this.settings.approval_reminder_repeat_days;
      if (!Number.isInteger(first) || first < 1 || first > 30) {
        useDeletionsStore().notify({ prefix: 'Первое напоминание: от 1 до 30 дней', type: 'error' });
        return;
      }
      if (!Number.isInteger(repeat) || repeat < 1 || repeat > 30) {
        useDeletionsStore().notify({ prefix: 'Повтор напоминания: от 1 до 30 дней', type: 'error' });
        return;
      }
      this.saving = true;
      try {
        await updateSetting('approval.reminder_enabled', String(this.settings.approval_reminder_enabled));
        await updateSetting('approval.reminder_first_days', String(first));
        await updateSetting('approval.reminder_repeat_days', String(repeat));
        useDeletionsStore().notify({ prefix: 'Настройки напоминаний сохранены' });
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

    async fetchDataProcessingDoc() {
      this.dpLoading = true;
      try {
        this.dpMeta = await getDataProcessingMeta();
        this.dpLoaded = true;
      } catch (error) {
        console.error('Ошибка загрузки документа согласия:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить документ', type: 'error' });
      } finally {
        this.dpLoading = false;
      }
    },

    async onDpFileChange(event) {
      const file = event.target.files?.[0];
      // Сбрасываем input, иначе повторный выбор того же файла не вызовет change.
      event.target.value = '';
      if (!file) return;
      this.dpUploading = true;
      try {
        this.dpMeta = await uploadDataProcessingDoc(file);
        this.dpLoaded = true;
        useDeletionsStore().notify({ prefix: 'Документ ', bold: this.dpMeta.file_name, suffix: ' загружен' });
      } catch (error) {
        useDeletionsStore().notify({ prefix: error?.message || 'Ошибка загрузки документа', type: 'error' });
      } finally {
        this.dpUploading = false;
      }
    },

    async downloadDp() {
      if (!this.dpMeta) return;
      try {
        await downloadDataProcessingDoc(this.dpMeta.file_name);
      } catch (error) {
        console.error('Ошибка скачивания документа согласия:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось скачать документ', type: 'error' });
      }
    },

    async deleteDp() {
      if (!this.dpMeta) return;
      const ok = await useUiStore().confirm({
        message: `Удалить документ «${this.dpMeta.file_name}»? Пользователи перестанут видеть его при подаче заявки.`,
      });
      if (!ok) return;
      const name = this.dpMeta.file_name;
      this.dpDeleting = true;
      try {
        await deleteDataProcessingDoc();
        this.dpMeta = null;
        useDeletionsStore().notify({ prefix: 'Документ ', bold: name, suffix: ' удалён' });
      } catch (error) {
        console.error('Ошибка удаления документа согласия:', error);
        useDeletionsStore().notify({ prefix: 'Ошибка удаления документа', type: 'error' });
      } finally {
        this.dpDeleting = false;
      }
    },

    applyPdConsentSettings(settings) {
      this.pdcText = settings?.text || '';
      this.pdcVersion = settings?.version || 1;
      this.pdcRequired = Boolean(settings?.required);
    },

    async fetchPdConsentSettings() {
      this.pdcLoading = true;
      try {
        this.applyPdConsentSettings(await getPDConsentSettings());
        this.pdcLoaded = true;
        this.fetchPdcCollection();
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось загрузить текст согласия',
          type: 'error',
        });
      } finally {
        this.pdcLoading = false;
      }
    },

    /**
     * Сводку тянем отдельно от настроек: она считается по всем учётным записям и
     * не должна задерживать показ самого текста согласия. Ошибку не показываем
     * тостом - блок просто не рисуется, текст и тумблер при этом работают.
     */
    async fetchPdcCollection() {
      this.pdcCollectionLoading = true;
      try {
        this.pdcCollection = await getPDConsentCollection();
      } catch (error) {
        console.error('Не удалось загрузить сводку по сбору согласий:', error);
        this.pdcCollection = null;
      } finally {
        this.pdcCollectionLoading = false;
      }
    },

    /**
     * Выгружает список не подтвердивших: администратору с ним идти напоминать, а
     * пятнадцать человек из интерфейса не перепишешь. Формат тот же, что у прочих
     * выгрузок системы.
     */
    async exportPdcPending() {
      const rows = this.pdcCollection?.pending_users || [];
      if (!rows.length) return;
      try {
        const ExcelJS = (await import('exceljs')).default;
        const workbook = new ExcelJS.Workbook();
        const sheet = workbook.addWorksheet('Не подтвердили');

        const headerRow = sheet.addRow(['ФИО', 'Логин', 'Организация']);
        headerRow.height = 25;
        headerRow.eachCell((cell) => {
          cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } };
          cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
          cell.alignment = { vertical: 'middle', horizontal: 'center' };
        });
        rows.forEach((person) => {
          sheet.addRow([person.full_name, person.username, person.organization]);
        });
        sheet.columns.forEach((column) => { column.width = 32; });

        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], {
          type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
        });
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `Soglasie_ne_podtverdili_red${this.pdcCollection.version}.xlsx`;
        document.body.appendChild(link);
        link.click();
        link.remove();
        window.URL.revokeObjectURL(url);
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось выгрузить список',
          type: 'error',
        });
      }
    },

    async extractPdcText() {
      if (!this.dpMeta) return;
      this.pdcExtracting = true;
      try {
        const blob = await fetchDataProcessingBlob();
        const html = await extractDocumentHtml(blob, this.dpMeta.ext);
        if (!html) {
          useDeletionsStore().notify({
            prefix: 'В документе не нашлось текста. Возможно, это сканированные страницы.',
            type: 'warning',
          });
          return;
        }
        this.pdcText = html;
        useDeletionsStore().notify({
          prefix: 'Текст извлечён. Проверьте нумерацию и переносы, затем сохраните.',
          type: 'info',
        });
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось извлечь текст из документа',
          type: 'error',
        });
      } finally {
        this.pdcExtracting = false;
      }
    },

    async savePdcText() {
      this.pdcSaving = true;
      try {
        this.applyPdConsentSettings(await savePDConsentText(this.pdcText));
        this.pdcLoaded = true;
        useDeletionsStore().notify({ prefix: 'Текст согласия сохранён' });
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось сохранить текст согласия',
          type: 'error',
        });
      } finally {
        this.pdcSaving = false;
      }
    },

    async togglePdcRequired() {
      if (this.pdcBusy) return;
      const next = !this.pdcRequired;
      if (next) {
        const ok = await useUiStore().confirm({
          message: 'Включить запрос согласия при первом входе? Снять его можно тем же тумблером.',
        });
        if (!ok) return;
      }
      this.pdcTogglingRequired = true;
      try {
        this.applyPdConsentSettings(await setPDConsentRequired(next));
        useDeletionsStore().notify({
          prefix: next ? 'Согласие запрашивается при входе' : 'Запрос согласия выключен',
        });
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось изменить настройку',
          type: 'error',
        });
      } finally {
        this.pdcTogglingRequired = false;
      }
    },

    async requirePdcAgain() {
      const ok = await useUiStore().confirm({
        message: 'Требовать согласие заново? Все, кто соглашался с прежней редакцией текста,'
          + ' подтвердят его при следующем входе.',
      });
      if (!ok) return;
      this.pdcRequiringAgain = true;
      try {
        this.applyPdConsentSettings(await requirePDConsentAgain());
        // Состав согласившихся только что обнулился - сводка обязана это показать.
        this.fetchPdcCollection();
        useDeletionsStore().notify({ prefix: `Редакция согласия поднята до ${this.pdcVersion}` });
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось запросить согласие заново',
          type: 'error',
        });
      } finally {
        this.pdcRequiringAgain = false;
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
  color: var(--text);
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
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  overflow: hidden;
}

/* Мобильный селектор раздела вместо тесной полосы табов (#1208). */
.admin-settings__section-select {
  display: none;
}

.sidebar-item {
  padding: 10px 16px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  border-left: 3px solid transparent;
  font-family: 'Montserrat', sans-serif;
}

.sidebar-item:hover {
  background: var(--surface-2);
}

.sidebar-item--active {
  background: var(--accent-tint);
  color: var(--accent-text);
  border-left-color: var(--accent-text);
  font-weight: 600;
}

.sidebar-item + .sidebar-item {
  border-top: 1px solid var(--border);
}

.admin-settings__content {
  flex: 1;
  min-width: 0;
  background: var(--surface);
  border: 1px solid var(--border);
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
  color: var(--text);
  margin: 0 0 16px 0;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
}

.form-group {
  margin-bottom: 16px;
}

.form-label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--text);
  margin-bottom: 6px;
  font-family: 'Montserrat', sans-serif;
}

.form-input {
  width: 140px;
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  font-size: 13px;
  font-family: 'Montserrat', sans-serif;
  color: var(--text);
  transition: border-color 0.2s;
  outline: none;
}

.form-input:focus {
  border-color: var(--accent);
}

.form-input:disabled {
  background: var(--surface-2);
  color: var(--text-muted);
  cursor: not-allowed;
}

.form-hint {
  display: block;
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 4px;
}

.section-intro {
  margin: 0 0 16px;
  font-size: 13px;
}

.subsection-title {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.bureau-schedule {
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid var(--border);
}

.form-range {
  width: 100%;
  max-width: 300px;
  margin-top: 4px;
  accent-color: var(--accent-text);
  cursor: pointer;
}

.range-labels {
  display: flex;
  justify-content: space-between;
  max-width: 300px;
  font-size: 11px;
  color: var(--text-muted);
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
  border: 1px solid var(--border);
  border-radius: 50px;
  transition: all 0.2s;
  font-size: 12px;
}

.checkbox-label:hover {
  border-color: var(--accent);
  background: var(--accent-tint);
}

.checkbox-label input[type="checkbox"] {
  accent-color: var(--accent-text);
  cursor: pointer;
}

.checkbox-text {
  font-family: 'Montserrat', sans-serif;
  font-size: 12px;
  color: var(--text);
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
  color: var(--text);
  font-family: 'Montserrat', sans-serif;
}

.switch {
  position: relative;
  width: 40px;
  height: 22px;
  background: var(--border);
  border-radius: 11px;
  transition: background 0.2s ease;
  flex-shrink: 0;
  outline: none;
}

.switch:focus-visible {
  box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.3);
}

.switch--on {
  background: var(--accent);
}

.switch__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 18px;
  height: 18px;
  background: var(--surface);
  border-radius: 50%;
  transition: transform 0.2s ease;
  box-shadow: 0 1px 3px var(--shadow-drop);
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
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.btn--primary:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn--primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn--ghost {
  background: var(--surface);
  color: var(--accent-text);
  border-color: var(--accent);
  text-decoration: none;
}

.btn--ghost:hover:not(:disabled):not(.btn--disabled) {
  background: var(--accent-tint);
}

.btn--danger {
  background: var(--surface);
  color: var(--danger-text);
  border-color: var(--danger);
}

.btn--danger:hover:not(:disabled) {
  background: var(--danger-bg);
}

.btn--danger:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn--disabled {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}

/* Обработка данных */
.dp-desc {
  font-size: 12px;
  color: var(--text);
  line-height: 1.5;
  margin: 0 0 16px 0;
}

.dp-desc code {
  background: var(--accent-tint);
  color: var(--accent-text);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  font-size: 11px;
}

.dp-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
}

.dp-card__info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.dp-card__name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  word-break: break-word;
}

.dp-card__meta {
  font-size: 11px;
  color: var(--text-muted);
}

.dp-card__actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.dp-upload {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.dp-upload .form-hint {
  margin-top: 0;
}

/* Текст согласия при первом входе */
.pdc-title {
  margin-top: 28px;
  padding-top: 20px;
  border-top: 1px solid var(--border);
}

.pdc-warning {
  margin-bottom: 16px;
  padding: 12px 16px;
  border: 1px solid var(--danger);
  border-radius: var(--radius-md);
  background: var(--danger-bg);
  color: var(--danger-text);
  font-size: 13px;
  line-height: 1.5;
}

.pdc-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin: 12px 0;
}

.pdc-actions .form-hint {
  margin-top: 0;
}

.pdc-editor {
  margin-bottom: 4px;
}

.pdc-collection {
  margin-top: 20px;
  padding: 16px 18px;
  border: 1px solid var(--color-border, var(--border));
  border-radius: var(--radius-md, 15px);
  background: var(--surface);
}

.pdc-collection__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.pdc-collection__title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text, var(--text));
}

.pdc-collection__bar {
  height: 6px;
  border-radius: 999px;
  overflow: hidden;
  background: var(--accent-tint);
}

.pdc-collection__fill {
  height: 100%;
  background: var(--accent);
  transform: scaleX(0);
  transform-origin: left center;
  transition: transform 0.2s ease-out;
}

.pdc-collection__counts {
  margin: 8px 0 12px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text, var(--text));
}

.pdc-collection__list {
  margin: 8px 0 12px;
  padding: 0;
  list-style: none;
  max-height: 220px;
  overflow-y: auto;
}

.pdc-collection__item {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
  padding: 6px 0;
  border-bottom: 1px solid var(--color-border, var(--border));
  font-size: 13px;
}

.pdc-collection__item:last-child {
  border-bottom: none;
}

.pdc-collection__name {
  font-weight: 500;
  color: var(--color-text, var(--text));
}

.pdc-collection__login,
.pdc-collection__org {
  font-size: 12px;
  color: var(--color-text-muted, var(--text-muted));
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
  border: 2px solid var(--border);
  border-top-color: var(--accent-text);
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
  color: var(--danger-text);
  font-size: 13px;
  margin: 0 0 12px 0;
}

@media (max-width: 768px) {
  .admin-settings__layout {
    flex-direction: column;
  }

  .admin-settings__sidebar {
    display: none;
  }

  .admin-settings__section-select {
    display: block;
    width: 100%;
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
}
</style>
