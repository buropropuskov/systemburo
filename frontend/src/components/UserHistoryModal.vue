<template>
  <Teleport to="body">
    <transition
      name="modal-fade"
      @after-leave="onAfterLeave"
    >
      <div
        v-if="visible"
        class="modal-overlay"
        data-testid="user-history-modal"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="user-history-modal"
          @mousedown.stop
        >
          <div class="modal-header">
            <h3>История учётной записи «{{ formatLogin(user.username) }}»</h3>
            <div class="header-actions">
              <button
                class="export-btn"
                :disabled="filteredHistory.length === 0 || isExporting"
                @click="exportToExcel"
              >
                <AppIcon
                  v-if="!isExporting"
                  name="export"
                  class="export-icon"
                />
                <span v-if="!isExporting">Экспорт</span>
                <div
                  v-else
                  class="export-loader"
                />
              </button>
              <button
                class="close-btn"
                aria-label="Закрыть"
                @click="requestClose"
              >
                ×
              </button>
            </div>
          </div>

          <div class="history-filters">
            <div class="filter-row">
              <div class="search-filter">
                <span class="filter-label">Поиск:</span>
                <input
                  v-model="searchQuery"
                  type="text"
                  class="search-input"
                  placeholder="Поиск по пользователю, действию..."
                >
              </div>

              <div class="user-filter">
                <span class="filter-label">Кто:</span>
                <div
                  class="custom-select"
                  @click="toggleUserDropdown"
                >
                  <div class="select-trigger">
                    <span class="selected-value">{{ selectedUserName }}</span>
                    <AppIcon
                      name="arrow"
                      class="select-arrow"
                      :class="{ 'arrow-open': userDropdownOpen }"
                    />
                  </div>
                  <transition name="fade">
                    <div
                      v-if="userDropdownOpen"
                      class="select-dropdown"
                    >
                      <div
                        class="select-option"
                        :class="{ 'selected': selectedActorId === null }"
                        @click.stop="selectUser(null)"
                      >
                        Все пользователи
                      </div>
                      <div
                        v-for="actor in uniqueUsers"
                        :key="actor.id"
                        class="select-option"
                        :class="{ 'selected': selectedActorId === actor.id }"
                        @click.stop="selectUser(actor.id)"
                      >
                        {{ actor.name }}
                      </div>
                    </div>
                  </transition>
                </div>
              </div>

              <div class="date-filter">
                <span class="filter-label">Период:</span>
                <input
                  v-model="dateFrom"
                  type="date"
                  class="date-input"
                >
                <span class="date-separator">-</span>
                <input
                  v-model="dateTo"
                  type="date"
                  class="date-input"
                >
              </div>

              <div class="sort-filter">
                <span class="filter-label">Сортировка:</span>
                <button
                  class="sort-btn"
                  @click="toggleSortOrder"
                >
                  <AppIcon
                    name="sort"
                    class="sort-icon"
                    :class="{ 'sort-asc': sortOrder === 'asc' }"
                  />
                  <span>{{ sortOrder === 'desc' ? 'Сначала новые' : 'Сначала старые' }}</span>
                </button>
              </div>
            </div>
          </div>

          <div class="modal-content">
            <div
              v-if="loading"
              class="history-loading"
            >
              <LoaderSpinner label="Загрузка истории..." />
            </div>

            <div
              v-else-if="filteredHistory.length === 0"
              class="history-empty"
            >
              История пуста
            </div>

            <div
              v-else
              class="history-timeline"
            >
              <template
                v-for="group in historyGroupedByDate"
                :key="group.date"
              >
                <div class="history-date-separator">
                  {{ group.date }}
                </div>
                <div
                  v-for="(item, i) in group.items"
                  :key="item.id"
                  class="history-item"
                >
                  <div
                    class="timeline-dot"
                    :class="getActionClass(item.action_type)"
                  />
                  <div
                    v-if="i < group.items.length - 1"
                    class="timeline-line"
                  />

                  <div class="history-content">
                    <div class="history-header">
                      <span class="user-name">{{ item.actor_name || 'Система' }}</span>
                      <span class="action-time">{{ formatDateTime(item.created_at) }}</span>
                    </div>

                    <div class="action-text">
                      {{ getActionText(item) }}
                    </div>

                    <div
                      v-if="getActionComment(item)"
                      class="action-comment"
                    >
                      {{ getActionComment(item) }}
                    </div>
                  </div>
                </div>
              </template>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { ref } from 'vue';
import { formatLogin } from '@/utils/formatName';
import { apiRequest } from '@/api/client';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { useDeletionsStore } from '@/stores/deletions';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
import ExcelJS from 'exceljs';
import { formatMoscow, formatMoscowDateTime } from '@/utils/serverTime';

const ACTION_TEXTS = {
  created: 'Учётная запись создана',
  updated: 'Изменены данные',
  type_changed: 'Изменён тип пользователя',
  org_changed: 'Изменена организация',
  company_changed: 'Изменена компания',
  password_reset: 'Сброшен пароль',
  archived: 'Учётная запись архивирована',
  restored: 'Учётная запись восстановлена из архива',
  banned: 'Заблокирован',
  unbanned: 'Разблокирован',
  consent_granted: 'Дал согласие на обработку персональных данных',
  consent_revoked: 'Отозвал согласие на обработку персональных данных',
  impersonate_start: 'Вход в систему от имени работника',
  impersonate_stop: 'Выход из режима работы от имени работника',
};

const ACTION_DOT_CLASS = {
  created: 'dot-create',
  updated: 'dot-update',
  type_changed: 'dot-update',
  org_changed: 'dot-update',
  company_changed: 'dot-update',
  password_reset: 'dot-update',
  archived: 'dot-deactivate',
  restored: 'dot-activate',
  banned: 'dot-deactivate',
  unbanned: 'dot-activate',
  consent_granted: 'dot-activate',
  consent_revoked: 'dot-deactivate',
  // Вход под чужой учётной записью - событие того же веса, что блокировка:
  // нейтральная точка прятала бы его в ленте среди правок телефона.
  impersonate_start: 'dot-deactivate',
  impersonate_stop: 'dot-activate',
};

// Читаемые лейблы для полей в details (updated/created).
const FIELD_LABELS = {
  username: 'Логин',
  last_name: 'Фамилия',
  first_name: 'Имя',
  middle_name: 'Отчество',
  position: 'Должность',
  email: 'Email',
  phone: 'Телефон',
};

export default {
  name: 'UserHistoryModal',
  components: { LoaderSpinner, AppIcon },
  props: {
    user: { type: Object, required: true },
    organizations: { type: Array, default: () => [] },
    companies: { type: Array, default: () => [] },
    userTypes: { type: Array, default: () => [] },
    currentUserName: { type: String, default: '' },
  },
  emits: ['close'],
  setup(_, { emit }) {
    // Анимация закрытия: показ управляем внутренним visible (enter по mounted,
    // leave по requestClose); emit('close') шлём только ПОСЛЕ leave-перехода
    // (@after-leave), иначе родитель размонтирует мгновенно и анимация не проиграется.
    const visible = ref(false);
    const requestClose = () => { visible.value = false; };
    const onAfterLeave = () => emit('close');
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(requestClose);
    return { visible, requestClose, onAfterLeave, onOverlayMousedown, onOverlayMouseup };
  },
  data() {
    return {
      loading: false,
      history: [],
      sortOrder: 'desc',
      searchQuery: '',
      selectedActorId: null,
      dateFrom: '',
      dateTo: '',
      userDropdownOpen: false,
      isExporting: false,
    };
  },
  computed: {
    uniqueUsers() {
      const users = new Map();
      this.history.forEach((item) => {
        if (item.actor_user_id && !users.has(item.actor_user_id)) {
          users.set(item.actor_user_id, {
            id: item.actor_user_id,
            name: item.actor_name || 'Система',
          });
        }
      });
      return Array.from(users.values()).sort((a, b) => a.name.localeCompare(b.name));
    },

    selectedUserName() {
      if (this.selectedActorId === null) return 'Все пользователи';
      const actor = this.uniqueUsers.find((u) => u.id === this.selectedActorId);
      return actor ? actor.name : 'Все пользователи';
    },

    filteredHistory() {
      let filtered = [...this.history];

      if (this.searchQuery && this.searchQuery.trim() !== '') {
        const query = this.searchQuery.toLowerCase().trim();
        filtered = filtered.filter((item) => {
          const actorName = (item.actor_name || '').toLowerCase();
          const actionText = this.getActionText(item).toLowerCase();
          const comment = this.getActionComment(item).toLowerCase();
          return actorName.includes(query)
            || actionText.includes(query)
            || comment.includes(query);
        });
      }

      if (this.selectedActorId) {
        filtered = filtered.filter((item) => item.actor_user_id === this.selectedActorId);
      }

      if (this.dateFrom) {
        const fromDate = new Date(this.dateFrom);
        fromDate.setHours(0, 0, 0, 0);
        filtered = filtered.filter((item) => new Date(item.created_at) >= fromDate);
      }

      if (this.dateTo) {
        const toDate = new Date(this.dateTo);
        toDate.setHours(23, 59, 59, 999);
        filtered = filtered.filter((item) => new Date(item.created_at) <= toDate);
      }

      return filtered.sort((a, b) => {
        const timeA = new Date(a.created_at).getTime();
        const timeB = new Date(b.created_at).getTime();
        return this.sortOrder === 'desc' ? timeB - timeA : timeA - timeB;
      });
    },

    historyGroupedByDate() {
      const groups = [];
      const dateMap = new Map();
      for (const item of this.filteredHistory) {
        const dateKey = formatMoscow(new Date(item.created_at), { day: 'numeric', month: 'long', year: 'numeric' });
        if (!dateMap.has(dateKey)) {
          dateMap.set(dateKey, []);
          groups.push({ date: dateKey, items: dateMap.get(dateKey) });
        }
        dateMap.get(dateKey).push(item);
      }
      return groups;
    },

    formattedCurrentDateTime() {
      return formatMoscowDateTime();
    },

    currentUserDisplayName() {
      if (!this.currentUserName) return 'Пользователь';
      const parts = this.currentUserName.split(' ').filter((p) => p && p !== 'null' && p !== 'undefined');
      return parts.length > 0 ? parts.join(' ') : 'Пользователь';
    },

    safeUserName() {
      const name = this.user.username || `id_${this.user.id}`;
      return name.replace(/[\\/:"*?<>|]/g, '_').replace(/\s+/g, '_');
    },

    exportData() {
      return this.filteredHistory.map((item) => ({
        'Дата и время': this.formatDateTime(item.created_at),
        'Пользователь': item.actor_name || 'Система',
        'Действие': this.getActionText(item),
        'Детали': this.getActionComment(item),
        'Тип действия': item.action_type,
        'ID записи': item.id,
      }));
    },
  },
  mounted() {
    this.visible = true;
    this.loadHistory();
    document.addEventListener('click', this.handleClickOutside);
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    document.removeEventListener('click', this.handleClickOutside);
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    formatLogin,

    onKeydown(e) {
      if (e.key === 'Escape') this.requestClose();
    },
    handleClickOutside(event) {
      // Контент в Teleport -> ищем дропдаун в body (this.$el под Teleport = якорь-коммент).
      const select = document.querySelector('.user-history-modal .custom-select');
      if (select && !select.contains(event.target)) {
        this.userDropdownOpen = false;
      }
    },

    async loadHistory() {
      this.loading = true;
      try {
        const response = await apiRequest(`/users/${encodeURIComponent(this.user.username)}/history`);
        if (response.ok) {
          const data = await response.json();
          this.history = Array.isArray(data) ? data : [];
        } else {
          useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'историю пользователя', type: 'error' });
        }
      } catch (error) {
        console.error('Error loading user history:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'историю пользователя', type: 'error' });
      } finally {
        this.loading = false;
      }
    },

    getActionClass(actionType) {
      return ACTION_DOT_CLASS[actionType] || 'dot-default';
    },

    getActionText(item) {
      return ACTION_TEXTS[item.action_type] || item.action_type;
    },

    orgName(id) {
      if (id === null || id === undefined) return 'Не выбрано';
      const org = this.organizations.find((o) => o.id === id);
      return org ? org.name : `#${id}`;
    },

    companyName(id) {
      if (id === null || id === undefined) return 'Не выбрано';
      const company = this.companies.find((c) => c.id === id);
      return company ? company.name : `#${id}`;
    },

    typeName(id) {
      if (id === null || id === undefined) return '-';
      const type = this.userTypes.find((t) => t.id === id);
      return type ? type.name : `#${id}`;
    },

    /**
     * Читаемые детали для timeline. ID организации/компании/типа резолвим в имена
     * через переданные справочники; пустые/null поля не показываем.
     */
    getActionComment(item) {
      const d = item.details;
      if (!d || typeof d !== 'object') return '';

      switch (item.action_type) {
        case 'impersonate_start': {
          // В details лежат логины обеих сторон и срок действия доступа. Без
          // разбора запись показывалась голым заголовком, и главное - до какого
          // момента действовал чужой доступ - на экран не попадало.
          const parts = [];
          if (d.actor_username) parts.push(`Администратор: ${formatLogin(d.actor_username)}`);
          if (d.expires_at) parts.push(`Доступ до ${this.formatDateTime(d.expires_at)}`);
          return parts.join(' / ');
        }
        case 'impersonate_stop':
          return d.actor_username ? `Администратор: ${formatLogin(d.actor_username)}` : '';
        case 'created': {
          const parts = [];
          if (d.username) parts.push(`Логин: ${d.username}`);
          if (d.type_id !== undefined && d.type_id !== null) parts.push(`Тип: ${this.typeName(d.type_id)}`);
          return parts.join(' / ');
        }
        case 'updated': {
          const parts = [];
          for (const [key, raw] of Object.entries(d)) {
            const label = FIELD_LABELS[key] || key;
            if (this.isDiff(raw)) {
              parts.push(`${label}: ${this.fieldOrDash(raw.old)} → ${this.fieldOrDash(raw.new)}`);
            } else if (raw !== null && raw !== undefined && raw !== '') {
              // старый формат записей (снапшот значения без old/new)
              parts.push(`${label}: ${this.formatFieldValue(raw)}`);
            }
          }
          return parts.join(' / ');
        }
        case 'type_changed':
          if (this.isDiff(d)) return `${this.typeName(d.old)} → ${this.typeName(d.new)}`;
          return `Новый тип: ${this.typeName(d.type_id)}`;
        case 'org_changed':
          if (this.isDiff(d)) return `${this.orgName(d.old)} → ${this.orgName(d.new)}`;
          return `Новая организация: ${this.orgName(d.organization_id)}`;
        case 'company_changed':
          if (this.isDiff(d)) return `${this.companyName(d.old)} → ${this.companyName(d.new)}`;
          return `Новая компания: ${this.companyName(d.company_id)}`;
        case 'banned':
          return d.reason ? `Причина: «${d.reason}»` : '';
        case 'unbanned': {
          // Сколько пробыл в блокировке (от banned_at снимка до момента разбана)
          // и по какой причине был заблокирован -- чтобы запись была информативной.
          const dur = d.banned_at ? this.formatBanDuration(d.banned_at, item.created_at) : '';
          if (dur && d.reason) return `Был в блокировке: ${dur}, причина: «${d.reason}»`;
          if (dur) return `Был в блокировке: ${dur}`;
          if (d.reason) return `Причина блокировки: «${d.reason}»`;
          return '';
        }
        // Редакция - главное в записи о согласии: по ней видно, с каким текстом
        // человек согласился, если текст с тех пор переиздавали.
        case 'consent_granted':
          return d.version ? `Редакция ${d.version}` : '';
        case 'password_reset':
        case 'archived':
        case 'restored':
        case 'consent_revoked':
        default:
          return '';
      }
    },

    /**
     * Длительность блокировки человекочитаемо (до двух старших единиц):
     * "2 дн. 3 ч.", "5 ч. 12 мин.", "8 мин.", "меньше минуты".
     */
    formatBanDuration(fromIso, toIso) {
      const from = new Date(fromIso).getTime();
      const to = new Date(toIso).getTime();
      if (!Number.isFinite(from) || !Number.isFinite(to) || to <= from) return '';
      let mins = Math.floor((to - from) / 60000);
      if (mins < 1) return 'меньше минуты';
      const days = Math.floor(mins / 1440);
      mins -= days * 1440;
      const hours = Math.floor(mins / 60);
      mins -= hours * 60;
      const parts = [];
      if (days) parts.push(`${days} дн.`);
      if (hours) parts.push(`${hours} ч.`);
      if (mins && !days) parts.push(`${mins} мин.`);
      return parts.join(' ') || 'меньше минуты';
    },

    isDiff(v) {
      return v !== null && typeof v === 'object' && ('old' in v || 'new' in v);
    },

    fieldOrDash(value) {
      if (value === null || value === undefined || value === '') return '—';
      return this.formatFieldValue(value);
    },

    formatFieldValue(value) {
      if (typeof value === 'boolean') return value ? 'да' : 'нет';
      const s = String(value);
      return s.length > 80 ? `${s.slice(0, 80)}...` : s;
    },

    formatDateTime(s) {
      if (!s) return '';
      return formatMoscowDateTime(new Date(s));
    },

    toggleSortOrder() {
      this.sortOrder = this.sortOrder === 'desc' ? 'asc' : 'desc';
    },

    toggleUserDropdown() {
      this.userDropdownOpen = !this.userDropdownOpen;
    },

    selectUser(actorId) {
      this.selectedActorId = actorId;
      this.userDropdownOpen = false;
    },

    async exportToExcel() {
      if (this.filteredHistory.length === 0) return;
      this.isExporting = true;
      try {
        const workbook = new ExcelJS.Workbook();
        const worksheet = workbook.addWorksheet(`Istoriya_${this.safeUserName}`);

        const headers = ['Дата и время', 'Пользователь', 'Действие', 'Детали', 'Тип действия', 'ID записи'];
        const headerRow = worksheet.addRow(headers);
        headerRow.height = 25;
        headerRow.eachCell((cell) => {
          cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } };
          cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
          cell.alignment = { vertical: 'middle', horizontal: 'center' };
          cell.border = {
            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            right: { style: 'thin', color: { argb: 'FFE6E6E6' } },
          };
        });

        this.exportData.forEach((item, index) => {
          const row = worksheet.addRow([
            item['Дата и время'],
            item['Пользователь'],
            item['Действие'],
            item['Детали'],
            item['Тип действия'],
            item['ID записи'],
          ]);
          row.height = 20;
          const fillColor = index % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF';
          row.eachCell((cell) => {
            cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: fillColor } };
            cell.font = { name: 'Verdana', size: 9, color: { argb: 'FF333333' } };
            cell.alignment = { vertical: 'middle', wrapText: true };
            cell.border = {
              top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              right: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            };
          });
        });

        const lastDataRow = this.exportData.length;
        const cols = headers.length;
        for (let row = 1; row <= lastDataRow + 1; row++) {
          const rightCell = worksheet.getCell(row, cols);
          rightCell.border = { ...rightCell.border, right: { style: 'medium', color: { argb: 'FF000000' } } };
          const leftCell = worksheet.getCell(row, 1);
          leftCell.border = { ...leftCell.border, left: { style: 'medium', color: { argb: 'FF000000' } } };
        }
        for (let col = 1; col <= cols; col++) {
          const topCell = worksheet.getCell(1, col);
          topCell.border = { ...topCell.border, top: { style: 'medium', color: { argb: 'FF000000' } } };
          const bottomCell = worksheet.getCell(lastDataRow + 1, col);
          bottomCell.border = { ...bottomCell.border, bottom: { style: 'medium', color: { argb: 'FF000000' } } };
        }

        worksheet.addRow([]);
        const infoRow1 = worksheet.addRow(['Отчёт сформировал:', this.currentUserDisplayName]);
        const infoRow2 = worksheet.addRow(['Дата формирования:', this.formattedCurrentDateTime]);
        [infoRow1, infoRow2].forEach((row) => {
          row.eachCell((cell) => {
            cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
            cell.alignment = { vertical: 'middle' };
            cell.border = {
              top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              right: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            };
          });
        });

        worksheet.columns = [
          { width: 22 },
          { width: 30 },
          { width: 30 },
          { width: 60 },
          { width: 22 },
          { width: 12 },
        ];

        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.download = `Istoriya_polzovatelya_${this.safeUserName}_${this.formattedCurrentDateTime.replace(/[.:,]/g, '-')}.xlsx`;
        a.href = url;
        a.click();
        window.URL.revokeObjectURL(url);
      } catch (error) {
        console.error('Error exporting history to Excel:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'выгрузить историю в Excel', type: 'error' });
      } finally {
        this.isExporting = false;
      }
    },
  },
};
</script>

<style scoped>
.history-date-separator {
  font-size: 11px;
  font-weight: 600;
  color: var(--accent-text);
  padding: 8px 0 4px;
  margin-bottom: 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  letter-spacing: 0.02em;
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--overlay);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 12000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

/* Анимация открытия/закрытия (паттерн BaseModal): overlay fade + контент scale */
.modal-fade-enter-active {
  transition: opacity 0.25s ease;
}

.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .user-history-modal {
  animation: modal-scale-in 0.25s ease;
}

.modal-fade-leave-active .user-history-modal {
  animation: modal-scale-out 0.2s ease;
}

@keyframes modal-scale-in {
  from { transform: scale(0.95); opacity: 0; }
  to { transform: scale(1); opacity: 1; }
}

@keyframes modal-scale-out {
  from { transform: scale(1); opacity: 1; }
  to { transform: scale(0.95); opacity: 0; }
}

.user-history-modal {
  background: var(--surface);
  border-radius: 45px;
  width: 900px;
  max-width: 95%;
  max-height: calc(var(--app-vh, 1vh) * 80);
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 30px var(--shadow-drop);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 25px;
  border-bottom: 1px solid var(--border);
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--text);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.export-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 6px 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  height: 32px;
}

.export-btn:hover:not(:disabled) {
  background: var(--surface-2);
  border-color: var(--accent);
}

.export-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.export-icon {
  width: 14px;
  height: 14px;
}

.export-loader {
  width: 16px;
  height: 16px;
  border: 2px solid var(--border);
  border-top: 2px solid var(--accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  color: var(--text-muted);
  cursor: pointer;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: all 0.2s ease;
}

.close-btn:hover {
  background: var(--surface-2);
  color: var(--text);
}

.history-filters {
  padding: 15px 25px;
  border-bottom: 1px solid var(--border);
  background-color: var(--surface-2);
}

.filter-row {
  display: flex;
  gap: 15px;
  align-items: center;
  flex-wrap: wrap;
}

.search-filter,
.user-filter,
.date-filter,
.sort-filter {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-label {
  font-size: 12px;
  color: var(--text-muted);
  white-space: nowrap;
}

.search-input {
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 12px;
  width: 200px;
  height: 32px;
  transition: all 0.2s ease;
}

.search-input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.1);
}

.custom-select {
  position: relative;
  width: 200px;
  cursor: pointer;
}

.select-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  transition: all 0.2s ease;
  height: 32px;
}

.select-trigger:hover {
  border-color: var(--accent);
  background: var(--surface-2);
}

.selected-value {
  font-size: 12px;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.select-arrow {
  width: 8px;
  height: 8px;
  transition: transform 0.2s ease;
}

.select-arrow.arrow-open {
  transform: rotate(90deg);
}

.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from, .fade-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

.select-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  max-height: 300px;
  overflow-y: auto;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 15px;
  box-shadow: 0 4px 12px var(--shadow-drop);
  z-index: 1000;
}

.select-option {
  padding: 10px 14px;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.select-option:hover {
  background-color: var(--accent-tint);
}

.select-option.selected {
  background-color: var(--accent-tint);
  font-weight: 500;
}

.date-input {
  padding: 6px 8px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 12px;
  width: 120px;
  height: 32px;
}

.date-separator {
  color: var(--text-muted);
  font-size: 12px;
}

.sort-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  height: 32px;
  width: 170px;
}

.sort-btn:hover {
  background: var(--surface-2);
  border-color: var(--accent);
}

.sort-icon {
  color: var(--text-muted);
  width: 14px;
  height: 14px;
  transition: transform 0.2s ease;
}

.sort-icon.sort-asc {
  transform: rotate(180deg);
}

.modal-content {
  padding: 20px 25px;
  overflow-y: auto;
  max-height: calc(var(--app-vh, 1vh) * 80 - 180px);
  position: relative;
}

.history-loading,
.history-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  color: var(--text-muted);
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.history-timeline {
  position: relative;
  padding-left: 20px;
  min-height: 100px;
}

.history-item {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  position: relative;
}

.history-item:last-child {
  margin-bottom: 0;
}

.timeline-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
  margin-top: 4px;
  z-index: 1;
  position: relative;
}

.timeline-line {
  position: absolute;
  left: 4px;
  top: 18px;
  width: 2px;
  height: calc(100% + 2px);
  background: var(--border);
}

.dot-create { background: var(--accent-text); }
.dot-update { background: #f59e0b; }
.dot-activate { background: #10b981; }
.dot-deactivate { background: #6b7280; }
.dot-default { background: #9ca3af; }

.history-content {
  flex: 1;
  min-width: 0;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 4px;
}

.user-name {
  font-weight: 500;
  color: var(--text);
  font-size: 13px;
}

.action-time {
  color: var(--text-muted);
  font-size: 11px;
}

.action-text {
  color: var(--text-muted);
  font-size: 12px;
  margin-bottom: 2px;
}

.action-comment {
  font-size: 11px;
  color: var(--text-muted);
  font-style: italic;
  margin-top: 4px;
  padding-left: 6px;
  border-left: 2px solid var(--border);
  word-break: break-word;
}

@media (max-width: 768px) {
  .filter-row {
    flex-direction: column;
    align-items: flex-start;
  }
  .search-filter,
  .user-filter,
  .date-filter,
  .sort-filter {
    width: 100%;
  }
  .custom-select,
  .search-input,
  .date-input,
  .sort-btn {
    width: 100%;
  }
  .date-input {
    width: calc(50% - 20px);
  }
}
</style>
