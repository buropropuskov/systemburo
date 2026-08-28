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
        data-testid="ob-admin-settings"
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

            <button
              class="btn btn--primary"
              :disabled="saving"
              @click="saveUploadSettings"
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

            <h3 class="section-title rotation-title">
              Срок действия паролей
            </h3>

            <div
              class="rotation-status"
              data-testid="rotation-status"
            >
              <p>
                Ближайшая проверка: <b>{{ nextRotationRunText }}</b>.
                Срок действия пароля вышел у работников: <b>{{ rotationStatus.expired }}</b>.
                Пароли им не меняются и письмами не рассылаются: каждый входит своим
                прежним паролем, а система просит задать новый и до этого никуда не пускает.
              </p>
              <p>
                Учётных записей с почтой: <b>{{ rotationStatus.eligible }}</b>,
                без почты: <b>{{ rotationStatus.without_email }}</b>.
                <span v-if="rotationStatus.without_email > 0">
                  Проверки сроков это не касается, но предупредить их заранее не получится -
                  адреса проставляет бюро.
                </span>
              </p>
              <p
                v-if="!rotationStatus.mail_configured"
                class="rotation-warning"
              >
                Почта не настроена. Проверке сроков она не нужна, но предупреждения о скором
                истечении отправлять нечем, и обновить пароль всем работникам нельзя - новый
                пароль доставить некуда. Параметры SMTP задаются в файле параметров.
              </p>
              <div class="rotation-actions">
                <button
                  class="btn btn--secondary"
                  :disabled="testingMail"
                  data-testid="mail-test-button"
                  @click="sendTestMail"
                >
                  {{ testingMail ? 'Отправляем...' : 'Отправить проверочное письмо' }}
                </button>
                <button
                  v-if="rotationStatus.mail_configured"
                  class="btn btn--danger"
                  :disabled="rotationRunning"
                  data-testid="rotation-run-button"
                  @click="askRunRotation"
                >
                  {{ rotationRunning ? 'Меняем пароли...' : 'Обновить пароль всем работникам' }}
                </button>
              </div>
            </div>

            <BaseModal
              :show="confirmRotation"
              title="Сменить пароли всем работникам?"
              width="440px"
              content-testid="rotation-confirm"
              @close="confirmRotation = false"
            >
              <p>
                Пароли сменятся у <b>{{ rotationStatus.eligible }}</b> работников с указанным
                адресом почты. Каждому уйдёт письмо с новым паролем.
              </p>
              <p>
                Все текущие сессии будут завершены - людям придётся войти заново.
                <span v-if="rotationStatus.without_email > 0">
                  Работников без почты ({{ rotationStatus.without_email }}) действие не затронет.
                </span>
              </p>
              <template #actions>
                <button
                  class="btn btn--secondary"
                  @click="confirmRotation = false"
                >
                  Отмена
                </button>
                <button
                  class="btn btn--danger"
                  data-testid="rotation-confirm-button"
                  @click="runRotationNow"
                >
                  Сменить пароли
                </button>
              </template>
            </BaseModal>

            <div class="form-group">
              <label class="switch-label">
                <span class="switch-text">Требовать смену пароля по истечении срока</span>
                <span
                  class="switch"
                  :class="{ 'switch--on': settings.password_rotation_enabled }"
                  role="switch"
                  :aria-checked="String(settings.password_rotation_enabled)"
                  tabindex="0"
                  data-testid="rotation-toggle"
                  @click="toggleRotation"
                  @keydown.enter="toggleRotation"
                  @keydown.space.prevent="toggleRotation"
                >
                  <span class="switch__thumb" />
                </span>
              </label>
              <span class="form-hint">
                Раз в сутки система отбирает работников с истёкшим сроком и просит их
                задать новый пароль при следующем входе.
              </span>
            </div>

            <div class="form-group">
              <label
                class="form-label"
                for="pw-rotation-days"
              >
                Срок действия пароля, суток
              </label>
              <input
                id="pw-rotation-days"
                v-model.number="settings.password_rotation_days"
                type="number"
                class="form-input"
                :min="30"
                :max="120"
                :disabled="!settings.password_rotation_enabled"
              >
              <span class="form-hint">
                От 30 до 120 суток. Верхняя граница - требование приказа ФСТЭК России N 21
                для информационных систем персональных данных.
              </span>
            </div>

            <div class="form-group">
              <label
                class="form-label"
                for="pw-rotation-notify"
              >
                Предупреждать заранее, суток
              </label>
              <input
                id="pw-rotation-notify"
                v-model.number="settings.password_rotation_notify_days_before"
                type="number"
                class="form-input"
                :min="0"
                :max="30"
                :disabled="!settings.password_rotation_enabled"
              >
              <span class="form-hint">0 - не предупреждать</span>
            </div>

            <div class="form-group">
              <label class="switch-label">
                <span class="switch-text">Требовать сменить пароль при первом входе</span>
                <span
                  class="switch"
                  :class="{ 'switch--on': settings.password_force_change_on_next_login, 'switch--disabled': !settings.password_rotation_enabled }"
                  role="switch"
                  :aria-checked="String(settings.password_force_change_on_next_login)"
                  tabindex="0"
                  data-testid="force-change-toggle"
                  @click="toggleForceChange"
                  @keydown.enter="toggleForceChange"
                  @keydown.space.prevent="toggleForceChange"
                >
                  <span class="switch__thumb" />
                </span>
              </label>
              <span class="form-hint">
                Относится к паролям, которые придумывает сама система: обновление всем
                работникам и сброс из карточки. Такой пароль уходит письмом открытым
                текстом, и обязательная смена при входе ограничивает срок его жизни в
                почтовом ящике. По истечении срока смена требуется всегда.
              </span>
            </div>

            <button
              class="btn btn--primary"
              :disabled="saving"
              @click="saveSecuritySettings"
            >
              {{ saving ? 'Сохранение...' : 'Сохранить' }}
            </button>
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
import { SkeletonTransition, SkeletonLine, SkeletonBlock, BaseModal } from '@/components/ui';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { useDeletionsStore } from '@/stores/deletions';
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
    BaseModal,
  },
  data() {
    return {
      activeSection: 'upload',
      sections: [
        { value: 'upload', label: 'Загрузка файлов' },
        { value: 'notifications', label: 'Уведомления' },
        { value: 'security', label: 'Безопасность' },
        { value: 'contacts', label: 'Информация Бюро' },
        { value: 'approvals', label: 'Напоминания' },
      ],
      loading: false,
      saving: false,
      testingMail: false,
      rotationRunning: false,
      confirmRotation: false,
      // Состояние сроков действия паролей. Пока не загрузилось - почта считается
      // ненастроенной: это запирающая сторона, кнопка обновления паролей до ответа
      // сервера не показывается.
      rotationStatus: {
        mail_configured: false,
        enabled: false,
        rotation_days: 90,
        eligible: 0,
        without_email: 0,
        expired: 0,
        expiring_soon: 0,
        next_run_at: null,
      },
      loadError: null,
      settings: {
        max_file_size: 10 * 1024 * 1024,
        notifications_delete_duration: 10,
        notifications_restore_duration: 5,
        password_min_length: 8,
        password_require_letter: true,
        password_require_uppercase: false,
        password_require_lowercase: false,
        password_require_digit: true,
        password_require_special: false,
        password_rotation_enabled: false,
        password_rotation_days: 90,
        password_rotation_notify_days_before: 7,
        password_force_change_on_next_login: true,
        bureau_phone: '',
        bureau_email: '',
        approval_reminder_enabled: true,
        approval_reminder_first_days: 3,
        approval_reminder_repeat_days: 3,
      },
      passwordToggles: [
        { key: 'password_require_letter', label: 'Требовать букву' },
        { key: 'password_require_uppercase', label: 'Требовать заглавную букву' },
        { key: 'password_require_lowercase', label: 'Требовать строчную букву' },
        { key: 'password_require_digit', label: 'Требовать цифру' },
        { key: 'password_require_special', label: 'Требовать спецсимвол' },
      ],
      bureauTimeSlots: [],
      bureauSlotsLoaded: false,
      bureauSlotsLoading: false,
      bureauSeq: 0,
    };
  },
  computed: {
    /** Дата ближайшей проверки сроков, человеческим языком. */
    nextRotationRunText() {
      const raw = this.rotationStatus.next_run_at;
      if (!raw) return 'после включения';
      const at = new Date(raw);
      if (Number.isNaN(at.getTime())) return 'после включения';
      return at.toLocaleString('ru-RU', {
        day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit',
      });
    },
    fileSizeMB() {
      return Math.round(this.settings.max_file_size / (1024 * 1024));
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
    this.fetchRotationStatus();
  },
  methods: {
    /**
     * Состояние сроков: у скольких работников пароль истёк, скольким уйдёт
     * предупреждение и настроена ли почта. Без этих чисел администратор включает
     * проверку вслепую, поэтому блок грузится вместе с экраном, а не по нажатию.
     */
    async fetchRotationStatus() {
      try {
        const response = await apiRequest('/settings/password-rotation/status');
        if (!response.ok) return;
        const json = await response.json();
        this.rotationStatus = json.data ?? json;
      } catch (error) {
        // Состояние справочное: сбой оставляет блок в исходном виде и не мешает
        // править остальные настройки.
        console.error('Не удалось загрузить состояние плановой смены паролей:', error);
      }
    },

    /**
     * Ручной прогон: спрашиваем подтверждение с числом затрагиваемых учётных
     * записей. Действие обрывает сессии всей организации, поэтому кнопка не
     * должна срабатывать с одного клика.
     */
    askRunRotation() {
      this.confirmRotation = true;
    },

    async runRotationNow() {
      this.confirmRotation = false;
      this.rotationRunning = true;
      try {
        const response = await apiRequest('/settings/password-rotation/run', { method: 'POST' });
        if (response.ok) {
          useDeletionsStore().notify({
            bold: 'Смена паролей запущена',
            suffix: ': письма встают в очередь и уходят по мере отправки',
          });
        } else {
          const errorData = await response.json().catch(() => ({}));
          useDeletionsStore().notify({
            prefix: 'Не удалось запустить смену: ',
            bold: errorData.message || 'ошибка',
            type: 'error',
          });
        }
      } catch (error) {
        console.error('Ошибка сети при запуске смены паролей:', error);
        useDeletionsStore().notify({ bold: 'Нет связи с сервером', type: 'error' });
      } finally {
        this.rotationRunning = false;
        await this.fetchRotationStatus();
      }
    },

    /**
     * Проверка сроков включается независимо от почты: она ничего не рассылает, а
     * лишь требует сменить пароль на входе. Запрет без почты стоит у ручного
     * обновления, где придуманный пароль надо кому-то выслать.
     */
    toggleRotation() {
      this.settings.password_rotation_enabled = !this.settings.password_rotation_enabled;
    },

    toggleForceChange() {
      if (!this.settings.password_rotation_enabled) return;
      this.settings.password_force_change_on_next_login = !this.settings.password_force_change_on_next_login;
    },

    /**
     * Проверочное письмо: единственный способ убедиться, что чужой почтовый сервер
     * принимает письма от системы, до того как от него зависят пароли работников.
     */
    async sendTestMail() {
      const to = window.prompt('На какой адрес отправить проверочное письмо?');
      if (!to) return;
      this.testingMail = true;
      try {
        const response = await apiRequest('/settings/mail/test', {
          method: 'POST',
          body: JSON.stringify({ to }),
        });
        if (response.ok) {
          useDeletionsStore().notify({ prefix: 'Проверочное письмо отправлено на ', bold: to });
          await this.fetchRotationStatus();
        } else {
          const errorData = await response.json().catch(() => ({}));
          useDeletionsStore().notify({
            prefix: 'Письмо не ушло: ',
            bold: errorData.message || 'ошибка',
            type: 'error',
          });
        }
      } catch (error) {
        console.error('Ошибка сети при отправке проверочного письма:', error);
        useDeletionsStore().notify({ bold: 'Нет связи с сервером', type: 'error' });
      } finally {
        this.testingMail = false;
      }
    },

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
          case 'password.rotation_enabled':
            this.settings.password_rotation_enabled = item.value === 'true';
            break;
          case 'password.rotation_days':
            this.settings.password_rotation_days = Number(item.value) || 90;
            break;
          case 'password.rotation_notify_days_before':
            this.settings.password_rotation_notify_days_before = Number(item.value) || 0;
            break;
          case 'password.force_change_on_next_login':
            this.settings.password_force_change_on_next_login = item.value === 'true';
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



    async saveUploadSettings() {
      this.saving = true;
      try {
        await updateSetting('upload.max_file_size', String(this.settings.max_file_size));
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


    async saveNotificationSettings() {
      const del = this.settings.notifications_delete_duration;
      const res = this.settings.notifications_restore_duration;
      if (del < 3 || del > 60 || res < 3 || res > 60) {
        useDeletionsStore().notify({ prefix: 'Длительность уведомлений: от 3 до 60 секунд', type: 'error' });
        return;
      }
      this.saving = true;
      try {
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
        await updateSetting('password.rotation_enabled', String(this.settings.password_rotation_enabled));
        await updateSetting('password.rotation_days', String(this.settings.password_rotation_days));
        await updateSetting('password.rotation_notify_days_before', String(this.settings.password_rotation_notify_days_before));
        await updateSetting('password.force_change_on_next_login', String(this.settings.password_force_change_on_next_login));
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

/* Блок состояния сроков: справочные числа перед тем, как включать проверку.
   Тревожная подложка только у предупреждения о почте. */
.rotation-title {
  margin-top: 28px;
}

.rotation-status {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
  font-size: 14px;
}

.rotation-status p {
  margin: 0;
}

.rotation-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.rotation-warning {
  padding: 10px 12px;
  border-radius: var(--radius-md);
  background: var(--warning-bg);
  color: var(--warning-text);
}

.switch--disabled {
  opacity: 0.5;
  cursor: not-allowed;
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
