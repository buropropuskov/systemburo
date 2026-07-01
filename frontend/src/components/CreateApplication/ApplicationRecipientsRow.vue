<template>
  <div
    ref="root"
    class="recipients-row"
  >
    <span class="recipients-label">Получатели:</span>

    <div class="recipients-chips">
      <span
        v-for="chip in visibleChips"
        :key="chip.key"
        class="recipient-chip"
        :class="{ 'is-approver': chip.isApprover }"
        :data-hint="chip.name"
      >
        <span class="recipient-chip__name">{{ shortName(chip.name) }}</span>
        <button
          v-if="chip.removable"
          class="recipient-chip__remove"
          title="Убрать читателя"
          @click="removeReader(chip.userId)"
        >
          ×
        </button>
      </span>

      <span
        v-if="!allChips.length"
        class="recipients-empty"
      >Нет получателей</span>

      <!-- Переполнение: остальные получатели в выпадающем списке -->
      <div
        v-if="overflowChips.length"
        ref="overflowRef"
        class="recipients-extra"
      >
        <button
          class="recipients-extra__btn"
          type="button"
          @click="toggleOverflow"
        >
          +{{ overflowChips.length }}
          <span
            class="recipients-chevron"
            :class="{ open: showOverflow }"
          >▾</span>
        </button>
        <transition name="rdrop">
          <div
            v-if="showOverflow"
            class="recipients-popover recipients-popover--overflow"
          >
            <span
              v-for="chip in overflowChips"
              :key="chip.key"
              class="recipient-chip"
              :class="{ 'is-approver': chip.isApprover }"
            >
              <span class="recipient-chip__name">{{ chip.name }}</span>
              <button
                v-if="chip.removable"
                class="recipient-chip__remove"
                title="Убрать читателя"
                @click="removeReader(chip.userId)"
              >
                ×
              </button>
            </span>
          </div>
        </transition>
      </div>

      <!-- Добавить читателя (только Руководители) -->
      <div
        ref="addRef"
        class="recipients-add"
      >
        <button
          class="recipients-add__btn"
          type="button"
          @click="toggleAdd"
        >
          + получатель
        </button>
        <transition name="rdrop">
          <div
            v-if="showAdd"
            class="recipients-popover recipients-popover--add"
          >
            <input
              v-model="search"
              class="recipients-search"
              type="text"
              placeholder="Поиск"
            >
            <div class="recipients-add-list">
              <button
                v-for="u in availableManagers"
                :key="u.userId"
                class="recipients-add-item"
                type="button"
                @click="addReader(u)"
              >
                <span class="recipients-add-item__name">{{ u.name }}</span>
                <span
                  v-if="u.position"
                  class="recipients-add-item__pos"
                >{{ u.position }}</span>
              </button>
              <div
                v-if="!availableManagers.length"
                class="recipients-add-empty"
              >
                Пользователей нет
              </div>
            </div>
          </div>
        </transition>
      </div>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { formatShortName } from '@/utils/formatName'

const MAX_VISIBLE = 4

export default {
  name: 'ApplicationRecipientsRow',
  props: {
    /** Дефолтные согласующие организации/компании - показываются, удалить нельзя. */
    approvers: {
      type: Array,
      default: () => []
    },
    /** Добавленные читатели (read-only доступ). v-model. */
    readers: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:readers'],
  data() {
    return {
      managerUsers: [],
      search: '',
      showAdd: false,
      showOverflow: false
    }
  },
  computed: {
    allChips() {
      const approverChips = this.approvers.map(a => ({
        key: `a-${a.user_id}`,
        userId: a.user_id,
        name: a.name,
        isApprover: true,
        removable: false
      }))
      const readerChips = this.readers.map(r => ({
        key: `r-${r.user_id}`,
        userId: r.user_id,
        name: r.name,
        isApprover: false,
        removable: true
      }))
      return [...approverChips, ...readerChips]
    },
    visibleChips() {
      return this.allChips.slice(0, MAX_VISIBLE)
    },
    overflowChips() {
      return this.allChips.slice(MAX_VISIBLE)
    },
    availableManagers() {
      const taken = new Set([
        ...this.approvers.map(a => a.user_id),
        ...this.readers.map(r => r.user_id)
      ])
      const q = this.search.trim().toLowerCase()
      return this.managerUsers
        .filter(u => !taken.has(u.userId))
        .filter(u => !q || u.name.toLowerCase().includes(q) || (u.position || '').toLowerCase().includes(q))
    }
  },
  mounted() {
    this.fetchManagers()
    document.addEventListener('mousedown', this.handleOutside)
  },
  beforeUnmount() {
    document.removeEventListener('mousedown', this.handleOutside)
  },
  methods: {
    async fetchManagers() {
      try {
        const response = await apiRequest('/users/all', {})
        if (!response.ok) return
        const users = await response.json()
        this.managerUsers = (users || [])
          .filter(u => u.user_type === 'Руководитель')
          .map(u => ({
            userId: u.id,
            name: this.displayName(u),
            username: u.username,
            position: u.position || ''
          }))
      } catch (error) {
        console.error('Ошибка загрузки руководителей:', error)
      }
    },

    displayName(u) {
      const names = [u.last_name, u.first_name, u.middle_name].filter(Boolean)
      return names.length ? names.join(' ') : u.username
    },

    // Полное ФИО-строку "Фамилия Имя Отчество" -> "Фамилия И.О." (общий формат, formatName).
    // Имена в чипах приходят строкой, поэтому разбираем на части перед сокращением.
    shortName(full) {
      const parts = String(full || '').trim().split(/\s+/)
      if (parts.length < 2) return full || ''
      return formatShortName({
        last_name: parts[0],
        first_name: parts[1],
        middle_name: parts.slice(2).join(' ')
      })
    },

    addReader(user) {
      if (this.readers.some(r => r.user_id === user.userId)) return
      this.$emit('update:readers', [
        ...this.readers,
        { user_id: user.userId, name: user.name, username: user.username }
      ])
      this.search = ''
      this.showAdd = false
    },

    removeReader(userId) {
      this.$emit('update:readers', this.readers.filter(r => r.user_id !== userId))
    },

    toggleAdd() {
      this.showAdd = !this.showAdd
      if (this.showAdd) this.showOverflow = false
    },

    toggleOverflow() {
      this.showOverflow = !this.showOverflow
      if (this.showOverflow) this.showAdd = false
    },

    // Клик вне конкретного дропдауна закрывает именно его - в т.ч. клик по свободному
    // месту при открытом "+ получатель".
    handleOutside(event) {
      const addEl = this.$refs.addRef
      if (this.showAdd && addEl && !addEl.contains(event.target)) {
        this.showAdd = false
      }
      const overflowEl = this.$refs.overflowRef
      if (this.showOverflow && overflowEl && !overflowEl.contains(event.target)) {
        this.showOverflow = false
      }
    }
  }
}
</script>

<style scoped>
.recipients-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.recipients-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: #64748b;
  flex-shrink: 0;
}

.recipients-chips {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.recipient-chip {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 999px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
  font-size: 0.78rem;
  color: #1e293b;
  white-space: nowrap;
}

/* Согласующих выделяем синим border (без текста роли). */
.recipient-chip.is-approver {
  border-color: #4F5BDF;
}

.recipient-chip__name {
  font-weight: 500;
}

/* Полное ФИО во всплывающей подсказке под чипом (системный стиль #333, как .tag-hint). */
.recipient-chip[data-hint]::after {
  content: attr(data-hint);
  position: absolute;
  top: calc(100% + 6px);
  left: 50%;
  transform: translateX(-50%);
  width: max-content;
  max-width: 240px;
  background: #333;
  color: #fff;
  padding: 5px 9px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 400;
  line-height: 1.3;
  text-align: center;
  white-space: normal;
  z-index: 60;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.15s;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.recipient-chip[data-hint]::before {
  content: '';
  position: absolute;
  top: calc(100% + 1px);
  left: 50%;
  transform: translateX(-50%);
  border: 5px solid transparent;
  border-bottom-color: #333;
  z-index: 61;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.15s;
}

.recipient-chip[data-hint]:hover::after,
.recipient-chip[data-hint]:hover::before {
  opacity: 1;
}

.recipient-chip__remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border: none;
  border-radius: 50%;
  background: none;
  color: #94a3b8;
  font-size: 0.95rem;
  line-height: 1;
  cursor: pointer;
  transition: all 0.15s ease;
}

.recipient-chip__remove:hover {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
}

.recipients-empty {
  font-size: 0.75rem;
  color: #94a3b8;
  font-style: italic;
}

.recipients-extra,
.recipients-add {
  position: relative;
  display: inline-flex;
}

.recipients-extra__btn,
.recipients-add__btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 12px;
  border-radius: 999px;
  border: 1px dashed #cbd5e1;
  background: #fff;
  font-size: 0.76rem;
  font-weight: 500;
  color: #4F5BDF;
  cursor: pointer;
  transition: all 0.15s ease;
}

.recipients-extra__btn:hover,
.recipients-add__btn:hover {
  border-color: #4F5BDF;
  background: rgba(79, 91, 223, 0.06);
}

.recipients-chevron {
  font-size: 0.7rem;
  transition: transform 0.2s ease;
}

.recipients-chevron.open {
  transform: rotate(180deg);
}

.recipients-popover {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  z-index: 50;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
  padding: 10px;
}

.recipients-popover--overflow {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 200px;
  max-height: 260px;
  overflow-y: auto;
}

.recipients-popover--add {
  width: 240px;
}

.recipients-search {
  width: 100%;
  box-sizing: border-box;
  padding: 6px 10px;
  border: 1px solid #e2e8f0;
  border-radius: var(--radius-md, 15px);
  font-size: 0.78rem;
  margin-bottom: 8px;
  outline: none;
}

.recipients-search:focus {
  border-color: #4F5BDF;
}

.recipients-add-list {
  max-height: 220px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.recipients-add-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 1px;
  width: 100%;
  text-align: left;
  padding: 6px 8px;
  border: none;
  border-radius: 8px;
  background: none;
  cursor: pointer;
  transition: background 0.15s ease;
}

.recipients-add-item:hover {
  background: #f1f5f9;
}

.recipients-add-item__name {
  font-size: 0.78rem;
  color: #1e293b;
  font-weight: 500;
}

.recipients-add-item__pos {
  font-size: 0.65rem;
  color: #94a3b8;
}

.recipients-add-empty {
  padding: 10px 8px;
  text-align: center;
  color: #94a3b8;
  font-size: 0.72rem;
  font-style: italic;
}

/* Плавное раскрытие выпадающих списков (transform + opacity). */
.rdrop-enter-active,
.rdrop-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.rdrop-enter-from,
.rdrop-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
