<template>
  <div
    class="docs-block"
    data-testid="ob-documents"
  >
    <div class="docs-block__head">
      <span class="docs-block__title">Документы</span>
    </div>

    <!-- Строка групп-пилюль + «Ещё» -->
    <div
      v-if="groups.length"
      class="docs-block__nav"
    >
      <div class="grp-bar">
        <button
          class="grp-pill"
          :class="{ 'grp-pill--active': activeGroupId === null }"
          @click="setGroup(null)"
        >
          Все
        </button>
        <button
          v-for="g in visibleGroups"
          :key="g.id"
          class="grp-pill"
          :class="{ 'grp-pill--active': activeGroupId === g.id }"
          @click="setGroup(g.id)"
        >
          {{ g.name }}
        </button>
        <div
          v-if="hiddenGroups.length"
          class="grp-more"
          :class="{ 'grp-more--open': moreOpen }"
        >
          <button
            class="grp-pill"
            @click="moreOpen = !moreOpen"
          >
            Ещё {{ hiddenGroups.length }} ▾
          </button>
          <div class="grp-more__menu">
            <button
              v-for="g in hiddenGroups"
              :key="g.id"
              class="grp-more__item"
              @click="setGroup(g.id); moreOpen = false"
            >
              {{ g.name }}
              <span class="grp-more__cnt">{{ g.doc_count || 0 }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Список документов -->
    <div
      v-if="loading"
      class="docs-block__empty"
    >
      Загрузка...
    </div>
    <div
      v-else-if="!filteredDocs.length"
      class="docs-block__empty"
    >
      Пока нет опубликованных документов
    </div>
    <div
      v-else
      class="docs-block__list"
    >
      <div
        v-for="doc in filteredDocs"
        :key="doc.id"
        class="doc-row"
      >
        <span
          class="doc-row__icon"
          title="Скачать"
          @click="download(doc)"
        >
          <FileTypeIcon
            :ext="doc.file_ext || 'file'"
            :size="30"
          />
        </span>
        <div
          class="doc-row__main"
          @click="download(doc)"
        >
          <div class="doc-row__name">{{ doc.title }}</div>
          <div class="doc-row__desc">
            <template v-if="doc.description">{{ doc.description }} &middot; </template>
            <span class="doc-row__date">{{ formatDate(doc.published_at || doc.created_at) }}</span>
          </div>
        </div>
        <button
          class="doc-row__dl"
          @click="download(doc)"
        >
          <svg
            width="12"
            height="12"
            viewBox="0 0 16 16"
            fill="none"
            style="margin-right: 4px"
          >
            <path
              d="M8 1v9m0 0L4.5 6.5M8 10l3.5-3.5M2 13h12"
              stroke="currentColor"
              stroke-width="1.6"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
          Скачать
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import FileTypeIcon from '@/components/ui/FileTypeIcon.vue';
import { listPublicDocuments, downloadDocument } from '@/api/documents';
import { useDeletionsStore } from '@/stores/deletions';

// Фиксированный порядок групп из backend (sort_order), хвост — в «Ещё».
// Сколько влезает в строку: 2 группы + «Все» фиксировано, остальное в дропдаун.
const VISIBLE_GROUPS_COUNT = 2;

export default {
  name: 'DocumentsBlock',
  components: { FileTypeIcon },
  data() {
    return {
      loading: false,
      documents: [],
      groups: [],
      shuffledGroups: [],
      activeGroupId: null,
      moreOpen: false,
    };
  },
  computed: {
    visibleGroups() {
      return this.shuffledGroups.slice(0, VISIBLE_GROUPS_COUNT);
    },
    hiddenGroups() {
      return this.shuffledGroups.slice(VISIBLE_GROUPS_COUNT);
    },
    filteredDocs() {
      if (this.activeGroupId === null) return this.documents;
      return this.documents.filter((d) => d.group_id === this.activeGroupId);
    },
  },
  mounted() {
    this.loadDocuments();
    document.addEventListener('click', this.onDocClick);
  },
  beforeUnmount() {
    document.removeEventListener('click', this.onDocClick);
  },
  methods: {
    async loadDocuments() {
      this.loading = true;
      try {
        const data = await listPublicDocuments();
        // Бэкенд отдаёт плоский массив групп: [{ id, name, sort_order, documents: [] }].
        // Виртуальная группа «Прочее» приходит с id=0. Нормализуем group_id документа под id
        // его группы, чтобы фильтрация по пилюле работала единообразно (в т.ч. для «Прочее»,
        // где документы приходят с group_id=null).
        const groups = Array.isArray(data) ? data : (data.groups || []);
        this.groups = groups.map((g) => ({ id: g.id, name: g.name, doc_count: (g.documents || []).length }));
        this.documents = groups.flatMap((g) =>
          (g.documents || []).map((d) => ({ ...d, group_id: g.id }))
        );
        // Случайный набор видимых разделов на каждую загрузку, остальное в «Ещё» (по ТЗ п.39).
        this.shuffledGroups = this.shuffle([...this.groups]);
      } catch {
        useDeletionsStore().notify({ prefix: 'Ошибка загрузки документов', bold: '', type: 'error' });
      } finally {
        this.loading = false;
      }
    },
    shuffle(arr) {
      for (let i = arr.length - 1; i > 0; i -= 1) {
        const j = Math.floor(Math.random() * (i + 1));
        [arr[i], arr[j]] = [arr[j], arr[i]];
      }
      return arr;
    },
    setGroup(id) {
      this.activeGroupId = id;
    },
    async download(doc) {
      try {
        await downloadDocument(doc.id, doc.file_name);
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Ошибка скачивания: ', bold: e?.message || 'сбой', type: 'error' });
      }
    },
    formatDate(dt) {
      if (!dt) return '';
      return new Date(dt).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
    },
    onDocClick(e) {
      // Закрыть дропдаун «Ещё» при клике вне него
      if (this.moreOpen && !this.$el.querySelector('.grp-more')?.contains(e.target)) {
        this.moreOpen = false;
      }
    },
  },
};
</script>

<style scoped>
.docs-block {
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: 22px;
  overflow: hidden;
}

.docs-block__head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 13px 16px 11px;
  gap: 10px;
}

.docs-block__title {
  font-size: 14px;
  font-weight: 700;
  color: var(--text);
  display: flex;
  align-items: center;
  gap: 8px;
}

.docs-block__nav {
  padding: 0 12px 10px;
  border-bottom: 1px solid var(--color-border);
}

/* --- Строка групп --- */
.grp-bar {
  display: flex;
  align-items: center;
  gap: 7px;
  flex-wrap: nowrap;
  overflow: hidden;
}

.grp-pill {
  height: 26px;
  padding: 0 13px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--color-border);
  background: var(--surface);
  font-family: inherit;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--text-muted);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
  transition: border-color 0.15s ease, color 0.15s ease, background 0.15s ease;
}

.grp-pill--active {
  background: var(--accent-tint);
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
  color: var(--accent-text);
}

.grp-pill:hover:not(.grp-pill--active) {
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
  color: var(--accent-text);
}

/* --- Дропдаун «Ещё» --- */
.grp-more {
  position: relative;
  flex-shrink: 0;
}

.grp-more__menu {
  position: absolute;
  top: 32px;
  right: 0;
  z-index: 5;
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: 14px;
  box-shadow: 0 8px 24px var(--shadow-drop);
  padding: 6px;
  min-width: 170px;
  display: none;
  flex-direction: column;
  gap: 2px;
}

.grp-more--open .grp-more__menu {
  display: flex;
}

.grp-more__item {
  text-align: left;
  border: none;
  background: none;
  font-family: inherit;
  font-size: 12px;
  color: var(--text);
  padding: 8px 10px;
  border-radius: 9px;
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  gap: 10px;
  transition: background 0.12s ease, color 0.12s ease;
}

.grp-more__item:hover {
  background: var(--accent-tint);
  color: var(--accent-text);
}

.grp-more__cnt {
  color: var(--text-muted);
  font-size: 10px;
}

/* --- Список документов --- */
.docs-block__list {
  padding: 0;
  /* Фиксированная высота на 3 строки (высота строки фикс. через .doc-row min-height):
     4-я и далее - скроллом, при 1-2 документах остаётся та же высота. Высота блока не
     прыгает при смене группы. */
  height: 171px;
  overflow-y: auto;
}

.docs-block__list::-webkit-scrollbar {
  width: 5px;
}

.docs-block__list::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--accent) 22%, var(--surface));
  border-radius: 4px;
}

.docs-block__empty {
  /* Та же высота, что у списка - высота блока не меняется между группой с
     документами и пустой группой. */
  min-height: 171px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px;
  text-align: center;
  color: var(--text-muted);
  font-size: 12px;
}

/* --- Строка документа --- */
.doc-row {
  display: flex;
  align-items: center;
  gap: 13px;
  height: 57px;
  box-sizing: border-box;
  padding: 0 14px;
  border-bottom: 1px solid var(--border);
  transition: background 0.15s ease;
}

.doc-row:last-child {
  border-bottom: none;
}

.doc-row:hover {
  background: var(--accent-tint);
}

.doc-row__icon {
  flex-shrink: 0;
  cursor: pointer;
}

.doc-row__main {
  flex: 1;
  min-width: 0;
  cursor: pointer;
}

.doc-row__name {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: color 0.15s ease;
}

.doc-row__main:hover .doc-row__name {
  color: var(--accent-text);
}

.doc-row__desc {
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.doc-row__date {
  color: var(--accent-text);
}

.doc-row__dl {
  display: inline-flex;
  align-items: center;
  height: 30px;
  padding: 0 14px;
  border-radius: var(--radius-pill);
  border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  background: var(--accent-tint);
  color: var(--accent-text);
  font-weight: 600;
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
  white-space: nowrap;
  flex-shrink: 0;
}

.doc-row__dl:hover {
  background: color-mix(in srgb, var(--accent) 18%, var(--surface));
  border-color: var(--accent);
}
</style>
