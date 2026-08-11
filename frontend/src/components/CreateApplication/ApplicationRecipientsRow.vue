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
        :class="{ 'is-approver': chip.isApprover, 'is-hinted': hintedChip === chip.key }"
        :data-hint="chip.name"
        @click="revealName(chip)"
      >
        <span class="recipient-chip__name">{{ shortName(chip.name) }}</span>
        <button
          v-if="chip.removable"
          class="recipient-chip__remove"
          title="Убрать читателя"
          @click.stop="removeReader(chip.userId)"
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
          {{ isNarrow ? `Ещё ${overflowChips.length}` : `+${overflowChips.length}` }}
          <svg
            class="recipients-chevron"
            :class="{ open: showOverflow }"
            width="10"
            height="10"
            viewBox="0 0 10 10"
            fill="none"
            aria-hidden="true"
          >
            <path
              d="M2 3.5L5 6.5L8 3.5"
              stroke="currentColor"
              stroke-width="1.6"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
        </button>
        <transition name="rdrop">
          <div
            v-if="showOverflow"
            class="recipients-popover recipients-popover--overflow"
            :style="popoverStyle"
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

      <!-- Добавить читателя. Кнопки нет, когда добавлять некого: коллег и
           руководителей не нашлось или список не пришёл. -->
      <div
        v-if="canAddRecipients"
        ref="addRef"
        class="recipients-add"
      >
        <button
          class="recipients-add__btn"
          type="button"
          :aria-label="isNarrow ? 'Добавить получателя' : null"
          @click="toggleAdd"
        >
          {{ isNarrow ? '+' : '+ получатель' }}
        </button>
        <transition name="rdrop">
          <div
            v-if="showAdd"
            class="recipients-popover recipients-popover--add"
            :style="popoverStyle"
          >
            <input
              v-model="search"
              class="recipients-search"
              type="text"
              placeholder="Поиск"
            >
            <div class="recipients-add-list">
              <button
                v-for="u in availableCandidates"
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
                <span
                  v-if="u.pdHidden"
                  class="recipients-add-item__masked"
                >ФИО скрыто до согласия на обработку данных</span>
              </button>
              <div
                v-if="!availableCandidates.length"
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
import { getViewportZoom } from '@/utils/viewportScale'

const MAX_VISIBLE = 4
// На телефоне в строку помещается один получатель, остальные - под кнопкой «Ещё N».
const MAX_VISIBLE_NARROW = 1

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
      candidateUsers: [],
      search: '',
      showAdd: false,
      showOverflow: false,
      isNarrow: false,
      popoverStyle: null,
      hintedChip: null
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
    maxVisible() {
      return this.isNarrow ? MAX_VISIBLE_NARROW : MAX_VISIBLE
    },
    visibleChips() {
      return this.allChips.slice(0, this.maxVisible)
    },
    overflowChips() {
      return this.allChips.slice(this.maxVisible)
    },
    // Кандидаты за вычетом тех, кто уже в строке. Поиск сюда не входит намеренно:
    // по нему гейтится кнопка, и пустая выдача поиска прятала бы её вместе с полем ввода.
    assignableCandidates() {
      const taken = new Set([
        ...this.approvers.map(a => a.user_id),
        ...this.readers.map(r => r.user_id)
      ])
      return this.candidateUsers.filter(u => !taken.has(u.userId))
    },
    availableCandidates() {
      const q = this.search.trim().toLowerCase()
      if (!q) return this.assignableCandidates
      return this.assignableCandidates
        .filter(u => u.name.toLowerCase().includes(q) || (u.position || '').toLowerCase().includes(q))
    },
    canAddRecipients() {
      return this.assignableCandidates.length > 0
    }
  },
  watch: {
    // Читателей можно удалять прямо из окна «Ещё N»: когда список опустел, обёртка
    // с ref размонтируется, и закрыть окно по клику вне уже некому - гасим сами.
    'overflowChips.length'(count) {
      if (!count && this.showOverflow) {
        this.showOverflow = false
        this.syncPopover(false)
      }
    },
    // Последнего кандидата забрали в читатели - кнопка пропадает вместе с открытым
    // окном, и закрыть его по клику вне уже некому (ref размонтирован).
    canAddRecipients(can) {
      if (!can && this.showAdd) {
        this.showAdd = false
        this.syncPopover(false)
      }
    },
    // Переход между мобильной и десктопной раскладкой с открытым окном оставил бы
    // либо мёртвые координаты, либо лишние слушатели.
    isNarrow() {
      if (this.showAdd) this.syncPopover(true, this.$refs.addRef)
      else if (this.showOverflow) this.syncPopover(true, this.$refs.overflowRef)
      else this.syncPopover(false)
    }
  },
  mounted() {
    this.fetchCandidates()
    document.addEventListener('mousedown', this.handleOutside)
    this.initNarrowWatcher()
  },
  beforeUnmount() {
    if (this.hintTimer) clearTimeout(this.hintTimer)
    document.removeEventListener('mousedown', this.handleOutside)
    this.stopReposition()
    if (this.narrowMql) {
      if (this.narrowMql.removeEventListener) this.narrowMql.removeEventListener('change', this.onNarrowChange)
      else if (this.narrowMql.removeListener) this.narrowMql.removeListener(this.onNarrowChange)
    }
  },
  methods: {
    /**
     * Кандидаты в получатели: коллеги по организации и компании плюс руководители.
     * Кого пускать - решает бэк (#1921), тем же предикатом он потом проверяет readers
     * при подаче; клиентского фильтра по типу здесь нет намеренно, иначе форма
     * предлагала бы людей, которых подача молча выбросит.
     *
     * silent403: отказ гасит кнопку «+ получатель», и тост об этом лишний - выбор
     * получателей не действие пользователя, а фоновая загрузка при открытии формы.
     */
    async fetchCandidates() {
      try {
        const response = await apiRequest('/users/recipient-candidates', { silent403: true })
        if (!response.ok) return
        const users = await response.json()
        this.candidateUsers = (users || []).map(u => ({
          userId: u.id,
          name: this.displayName(u),
          username: u.username,
          position: u.position || '',
          pdHidden: !!u.pd_hidden
        }))
      } catch (error) {
        console.error('Ошибка загрузки кандидатов в получатели:', error)
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
      this.syncPopover(false)
    },

    removeReader(userId) {
      this.$emit('update:readers', this.readers.filter(r => r.user_id !== userId))
    },

    toggleAdd() {
      this.showAdd = !this.showAdd
      if (this.showAdd) this.showOverflow = false
      this.syncPopover(this.showAdd, this.$refs.addRef)
    },

    toggleOverflow() {
      this.showOverflow = !this.showOverflow
      if (this.showOverflow) this.showAdd = false
      this.syncPopover(this.showOverflow, this.$refs.overflowRef)
    },

    /**
     * Имя в чипе сокращено до «Фамилия И.О.», полное живёт в подсказке на hover -
     * на телефоне его не увидеть. По тапу показываем и гасим через пару секунд.
     */
    revealName(chip) {
      if (!this.isNarrow) return
      if (this.hintTimer) clearTimeout(this.hintTimer)
      if (this.hintedChip === chip.key) {
        this.hintedChip = null
        return
      }
      this.hintedChip = chip.key
      this.hintTimer = setTimeout(() => { this.hintedChip = null }, 2500)
    },

    initNarrowWatcher() {
      if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return
      this.narrowMql = window.matchMedia('(max-width: 768px)')
      this.isNarrow = this.narrowMql.matches
      this.onNarrowChange = (e) => { this.isNarrow = e.matches }
      if (this.narrowMql.addEventListener) this.narrowMql.addEventListener('change', this.onNarrowChange)
      else if (this.narrowMql.addListener) this.narrowMql.addListener(this.onNarrowChange)
    },

    syncPopover(open, anchor) {
      if (!open || !this.isNarrow) {
        this.popoverStyle = null
        this.stopReposition()
        return
      }
      this.popoverAnchor = anchor || null
      this.reposition()
      window.addEventListener('scroll', this.reposition, true)
      window.addEventListener('resize', this.reposition)
    },

    stopReposition() {
      window.removeEventListener('scroll', this.reposition, true)
      window.removeEventListener('resize', this.reposition)
      this.popoverAnchor = null
    },

    reposition() {
      this.popoverStyle = this.buildPopoverStyle(this.popoverAnchor)
    },

    /**
     * Список открывался вниз-вправо от кнопки и на телефоне уезжал за правый край.
     * Считаем координаты сами и держим окно в пределах экрана; на десктопе оставляем
     * штатное absolute-позиционирование. rect в device-px под корневым масштабом,
     * innerWidth/innerHeight - нет, поэтому к layout-px приводятся обе величины.
     */
    buildPopoverStyle(anchor) {
      if (!this.isNarrow || !anchor || typeof window === 'undefined') return null
      const zoom = getViewportZoom() || 1
      const rect = anchor.getBoundingClientRect()
      const viewportWidth = window.innerWidth / zoom
      const viewportHeight = window.innerHeight / zoom
      const margin = 12
      const gap = 8
      const width = Math.min(280, viewportWidth - margin * 2)
      const left = Math.min(Math.max(margin, rect.left / zoom), viewportWidth - margin - width)
      const spaceBelow = viewportHeight - rect.bottom / zoom - gap
      const spaceAbove = rect.top / zoom - gap
      const flipUp = spaceBelow < 200 && spaceAbove > spaceBelow
      const maxHeight = Math.max(160, Math.min(320, (flipUp ? spaceAbove : spaceBelow) - margin))
      return {
        position: 'fixed',
        left: `${Math.round(left)}px`,
        width: `${Math.round(width)}px`,
        maxHeight: `${Math.round(maxHeight)}px`,
        overflowY: 'auto',
        // top: 'auto' в ветке вверх обязателен - иначе базовое top: calc(100% + 8px)
        // остаётся в силе и спорит с bottom.
        ...(flipUp
          ? { bottom: `${Math.round(viewportHeight - rect.top / zoom + gap)}px`, top: 'auto' }
          : { top: `${Math.round(rect.bottom / zoom + gap)}px`, bottom: 'auto' })
      }
    },

    // Клик вне конкретного дропдауна закрывает именно его - в т.ч. клик по свободному
    // месту при открытом "+ получатель".
    handleOutside(event) {
      const addEl = this.$refs.addRef
      if (this.showAdd && addEl && !addEl.contains(event.target)) {
        this.showAdd = false
        this.syncPopover(false)
      }
      const overflowEl = this.$refs.overflowRef
      if (this.showOverflow && overflowEl && !overflowEl.contains(event.target)) {
        this.showOverflow = false
        this.syncPopover(false)
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
  color: var(--accent-text);
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
  border: 1px solid var(--border);
  background: var(--accent-tint);
  font-size: 0.78rem;
  color: var(--accent-text);
  white-space: nowrap;
}

/* Согласующих выделяем синим border (без текста роли). */
.recipient-chip.is-approver {
  border-color: var(--accent);
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
  background: var(--hint-bg);
  color: var(--hint-text);
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
  box-shadow: 0 2px 8px var(--shadow-drop);
}

.recipient-chip[data-hint]::before {
  content: '';
  position: absolute;
  top: calc(100% + 1px);
  left: 50%;
  transform: translateX(-50%);
  border: 5px solid transparent;
  border-bottom-color: var(--hint-bg);
  z-index: 61;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.15s;
}

.recipient-chip[data-hint].is-hinted::after,
.recipient-chip[data-hint].is-hinted::before,
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
  color: var(--text-muted);
  font-size: 0.95rem;
  line-height: 1;
  cursor: pointer;
  transition: all 0.15s ease;
}

.recipient-chip__remove:hover {
  background: color-mix(in srgb, var(--danger) 12%, var(--surface));
  color: var(--danger-text);
}

.recipients-empty {
  font-size: 0.75rem;
  color: var(--text-muted);
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
  border: 1px dashed color-mix(in srgb, var(--accent) 25%, var(--surface));
  background: var(--surface);
  font-size: 0.76rem;
  font-weight: 500;
  color: var(--accent-text);
  cursor: pointer;
  transition: all 0.15s ease;
}

.recipients-extra__btn:hover,
.recipients-add__btn:hover {
  border-color: var(--accent);
  background: color-mix(in srgb, var(--accent) 8%, var(--surface));
}

/* Векторная стрелка: png-иконка на телефоне мылилась. currentColor - в цвет текста. */
.recipients-chevron {
  width: 10px;
  height: 10px;
  flex-shrink: 0;
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
  background: var(--surface);
  border: 1px solid var(--border);
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
  border: 1px solid var(--border);
  border-radius: var(--radius-md, 15px);
  font-size: 0.78rem;
  margin-bottom: 8px;
  outline: none;
}

.recipients-search:focus {
  border-color: var(--accent);
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
  background: var(--accent-tint);
}

.recipients-add-item__name {
  font-size: 0.78rem;
  color: var(--accent-text);
  font-weight: 500;
}

.recipients-add-item__pos {
  font-size: 0.65rem;
  color: var(--text-muted);
}

/* Подпись «почему вместо ФИО логин» - своим классом: должность рядом с ней остаётся
   должностью, и стиль одной не тянет за собой другую. */
.recipients-add-item__masked {
  font-size: 0.65rem;
  color: var(--text-muted);
  font-style: italic;
}

.recipients-add-empty {
  padding: 10px 8px;
  text-align: center;
  color: var(--text-muted);
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
@media (max-width: 768px) {
  /* Строка должна оставаться одной строкой: получатель, «Ещё N», плюс. */
  .recipients-row {
    flex-wrap: nowrap;
    gap: 8px;
    /* Без ограничения строка растёт по содержимому и распирает страницу. */
    width: 100%;
    max-width: 100%;
    min-width: 0;
  }

  .recipients-chips {
    flex: 1 1 auto;
    flex-wrap: nowrap;
    min-width: 0;
  }

  /* Чип отдаёт ширину первым: на узком экране имя ужимается многоточием,
     а «Ещё N» и плюс остаются целыми. */
  .recipient-chip {
    flex: 0 1 auto;
    min-width: 72px;
    padding: 6px 10px;
    overflow: hidden;
  }

  .recipient-chip__name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* Крестик был 16px - в него не попасть пальцем; растим зону клика,
     не раздувая сам чип. */
  .recipient-chip__remove {
    position: relative;
    width: 20px;
    height: 20px;
    flex-shrink: 0;
  }

  .recipient-chip__remove::before {
    content: '';
    position: absolute;
    inset: -8px;
  }

  /* Одна ширина независимо от числа: «Ещё 2» и «Ещё 12» не двигают соседей. */
  .recipients-extra__btn {
    min-height: 32px;
    width: 84px;
    justify-content: center;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .recipients-add__btn {
    width: 36px;
    height: 36px;
    padding: 0;
    justify-content: center;
    font-size: 1.1rem;
    flex-shrink: 0;
  }

  /* Ширину и координаты задаёт JS (buildPopoverStyle) - здесь только страховка
     от старой фиксированной ширины 240px. */
  .recipients-popover--add,
  .recipients-popover--overflow {
    width: auto;
    min-width: 0;
    max-width: calc(100vw - 24px);
  }

  .recipients-add-item {
    min-height: 40px;
  }
}
</style>
