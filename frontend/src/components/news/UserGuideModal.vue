<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="guide-overlay"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        @click.self="close"
      >
        <div class="guide">
          <header class="guide__header">
            <div>
              <h2
                :id="titleId"
                class="guide__title"
              >
                {{ title }}
              </h2>
              <p class="guide__sub">
                Инструкции по работе с системой «Бюро пропусков»
              </p>
            </div>
            <button
              type="button"
              class="guide__close"
              aria-label="Закрыть"
              @click="close"
            >
              ×
            </button>
          </header>

          <div
            v-if="loading"
            class="guide__state"
          >
            <LoaderSpinner label="Загрузка руководства…" />
          </div>

          <div
            v-else-if="!sections.length"
            class="guide__state"
          >
            Нет доступных разделов руководства
          </div>

          <template v-else>
            <div
              class="guide__roles"
              role="tablist"
              aria-label="Разделы руководства"
            >
              <button
                v-for="section in sections"
                :key="section.role"
                type="button"
                role="tab"
                class="role-pill"
                :class="{ 'role-pill--active': section.role === activeRole }"
                :aria-selected="section.role === activeRole"
                @click="activeRole = section.role"
              >
                <span class="role-pill__dot" />
                {{ roleLabel(section.role) }}
              </button>
            </div>

            <div
              v-if="currentSection"
              class="guide__body"
            >
              <div
                v-if="currentSection.file"
                class="file-card"
              >
                <span class="file-card__icon">
                  <FileTypeIcon
                    :ext="currentSection.file.ext || 'pdf'"
                    :size="40"
                  />
                </span>
                <div class="file-card__main">
                  <div class="file-card__name">
                    {{ currentSection.file.name }}
                  </div>
                  <div class="file-card__meta">
                    {{ fileTypeLabel(currentSection.file) }}
                    <span class="sep">·</span>
                    {{ formatSize(currentSection.file.size) }}
                    <span class="sep">·</span>
                    обновлено {{ formatDate(currentSection.file.updated_at) }}
                  </div>
                </div>
                <button
                  type="button"
                  class="file-card__dl"
                  :disabled="downloadingRole === currentSection.role"
                  @click="download(currentSection)"
                >
                  <svg
                    width="14"
                    height="14"
                    viewBox="0 0 16 16"
                    fill="none"
                  >
                    <path
                      d="M8 1v9m0 0L4.5 6.5M8 10l3.5-3.5M2 13h12"
                      stroke="currentColor"
                      stroke-width="1.7"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    />
                  </svg>
                  {{ downloadingRole === currentSection.role ? 'Скачивание…' : 'Скачать' }}
                </button>
              </div>
              <div
                v-else
                class="file-card file-card--empty"
              >
                <span class="file-card__icon">
                  <FileTypeIcon
                    ext="pdf"
                    :size="40"
                  />
                </span>
                <div class="file-card__main">
                  <div class="file-card__name">
                    Файл руководства ещё не загружен
                  </div>
                  <div class="file-card__meta">
                    Скоро здесь появится PDF для скачивания
                  </div>
                </div>
              </div>

              <div class="descr">
                <p class="descr__label">
                  Что внутри
                </p>
                <p
                  v-if="currentSection.lead"
                  class="descr__lead"
                >
                  {{ currentSection.lead }}
                </p>
                <ul
                  v-if="currentSection.items && currentSection.items.length"
                  class="descr__list"
                >
                  <li
                    v-for="(item, i) in currentSection.items"
                    :key="i"
                  >
                    {{ item }}
                  </li>
                </ul>
              </div>
            </div>
          </template>

          <footer class="guide__footer">
            <button
              type="button"
              class="guide__done"
              @click="close"
            >
              Готово
            </button>
          </footer>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import FileTypeIcon from '@/components/ui/FileTypeIcon.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import { downloadGuideFile } from '@/api/guide';
import { useDeletionsStore } from '@/stores/deletions';

let uid = 0;

const ROLE_LABELS = {
  user: 'Пользователь',
  guard: 'Охранник',
  admin: 'Администратор',
};

export default {
  name: 'UserGuideModal',
  components: { FileTypeIcon, LoaderSpinner },
  props: {
    show: { type: Boolean, default: false },
    title: { type: String, default: 'Руководство' },
    /**
     * Разделы руководства из GET /guide/sections (уже отфильтрованы по правам).
     * Каждый: { role, title, lead, items[], file: null | { name, ext, mime_type, size, updated_at, download_url } }.
     */
    sections: { type: Array, default: () => [] },
    loading: { type: Boolean, default: false },
  },
  emits: ['close'],
  data() {
    uid++;
    return {
      activeRole: this.sections[0]?.role || '',
      downloadingRole: null,
      titleId: `guide-modal-title-${uid}`,
    };
  },
  computed: {
    currentSection() {
      return this.sections.find(s => s.role === this.activeRole) || this.sections[0] || null;
    },
  },
  watch: {
    // Модалка всегда смонтирована (для leave-анимации): блокировку скролла и
    // сброс активной вкладки вешаем на открытие.
    show(visible) {
      document.body.style.overflow = visible ? 'hidden' : '';
      if (visible) this.activeRole = this.sections[0]?.role || '';
    },
    // Разделы приходят асинхронно (загрузка по открытию) — подхватываем первую вкладку.
    sections(list) {
      if (!list.some(s => s.role === this.activeRole)) {
        this.activeRole = list[0]?.role || '';
      }
    },
  },
  mounted() {
    document.addEventListener('keydown', this.onKey);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKey);
    document.body.style.overflow = '';
  },
  methods: {
    roleLabel(role) {
      return ROLE_LABELS[role] || role;
    },
    fileTypeLabel(file) {
      const ext = (file.ext || '').replace(/^\./, '').toUpperCase();
      return ext || 'PDF';
    },
    formatSize(bytes) {
      if (!bytes) return '';
      const mb = bytes / (1024 * 1024);
      if (mb >= 1) return `${mb.toFixed(1).replace('.', ',')} МБ`;
      const kb = Math.max(1, Math.round(bytes / 1024));
      return `${kb} КБ`;
    },
    formatDate(iso) {
      if (!iso) return '';
      const d = new Date(iso);
      if (Number.isNaN(d.getTime())) return '';
      return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
    },
    async download(section) {
      if (!section.file || this.downloadingRole === section.role) return;
      this.downloadingRole = section.role;
      try {
        await downloadGuideFile(section.file.download_url, section.file.name);
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Ошибка скачивания: ', bold: e?.message || 'сбой', type: 'error' });
      } finally {
        this.downloadingRole = null;
      }
    },
    onKey(e) {
      if (this.show && e.key === 'Escape') this.close();
    },
    close() {
      this.$emit('close');
    },
  },
};
</script>

<style scoped>
.guide-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
  padding: 24px;
}

.guide {
  width: min(720px, 100%);
  max-height: 88vh;
  background: #fff;
  border-radius: 30px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 24px 70px rgba(20, 24, 60, 0.28);
  animation: guide-pop 0.22s ease;
}

@keyframes guide-pop {
  from { opacity: 0; transform: translateY(12px) scale(0.985); }
  to { opacity: 1; transform: none; }
}

.guide__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 22px 26px 18px;
}

.guide__title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: #1a1a1a;
}

.guide__sub {
  margin: 3px 0 0;
  font-size: 12.5px;
  color: #9a9aae;
  font-weight: 500;
}

.guide__close {
  width: 36px;
  height: 36px;
  border: none;
  background: #f3f4fa;
  border-radius: 50%;
  font-size: 22px;
  line-height: 1;
  color: #6a6a7d;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
  flex-shrink: 0;
}

.guide__close:hover {
  background: #e9eaf4;
  color: #1a1a1a;
}

.guide__state {
  padding: 40px 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #9a9aae;
  font-size: 14px;
  flex: 1 1 auto;
}

.guide__roles {
  display: flex;
  gap: 8px;
  padding: 0 26px 4px;
}

.role-pill {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 42px;
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: var(--radius-pill);
  font-family: inherit;
  font-size: 13.5px;
  font-weight: 600;
  color: #6a6a7d;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.role-pill:hover:not(.role-pill--active) {
  border-color: #cfd4ff;
  color: var(--color-primary);
}

.role-pill--active {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: #fff;
}

.role-pill__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.7;
}

.guide__body {
  padding: 18px 26px 8px;
  overflow-y: auto;
}

.file-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px 18px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: #fafbff;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.file-card:not(.file-card--empty):hover {
  border-color: #cfd4ff;
  background: #f6f7ff;
}

.file-card--empty {
  background: #f6f6f9;
}

.file-card--empty .file-card__name {
  color: #6a6a7d;
}

.file-card__icon {
  flex-shrink: 0;
  display: inline-flex;
}

.file-card--empty .file-card__icon {
  opacity: 0.55;
}

.file-card__main {
  flex: 1;
  min-width: 0;
}

.file-card__name {
  font-size: 14.5px;
  font-weight: 700;
  color: #1a1a2e;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-card__meta {
  margin-top: 4px;
  font-size: 12px;
  color: #a2a2b4;
  display: flex;
  align-items: center;
  gap: 7px;
}

.file-card__meta .sep {
  color: #d7d9e8;
}

.file-card__dl {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  height: 40px;
  padding: 0 20px;
  border-radius: var(--radius-pill);
  border: none;
  background: var(--color-primary);
  color: #fff;
  font-weight: 600;
  font-size: 13px;
  font-family: inherit;
  cursor: pointer;
  transition: background 0.15s ease;
  white-space: nowrap;
  flex-shrink: 0;
}

.file-card__dl:hover:not(:disabled) {
  background: var(--color-primary-hover);
}

.file-card__dl:disabled {
  opacity: 0.6;
  cursor: progress;
}

.descr {
  margin-top: 18px;
}

.descr__label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #b3b6cf;
  margin: 0 0 8px;
}

.descr__lead {
  margin: 0 0 11px;
  font-size: 14px;
  line-height: 1.62;
  color: #3a3a4a;
}

.descr__list {
  margin: 4px 0 12px;
  padding-left: 0;
  list-style: none;
}

.descr__list li {
  position: relative;
  padding-left: 22px;
  margin-bottom: 7px;
  font-size: 13.5px;
  line-height: 1.5;
  color: #3a3a4a;
}

.descr__list li::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 8px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-primary);
  opacity: 0.55;
}

.guide__footer {
  padding: 16px 26px 22px;
  display: flex;
  justify-content: flex-end;
}

.guide__done {
  height: 42px;
  padding: 0 26px;
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: var(--radius-pill);
  font-family: inherit;
  font-size: 13.5px;
  font-weight: 600;
  color: #6a6a7d;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.guide__done:hover {
  background: #f3f4fa;
  color: #1a1a1a;
}

@media (max-width: 560px) {
  .guide__roles {
    flex-wrap: wrap;
  }
  .role-pill {
    flex: 1 1 auto;
  }
  .file-card {
    flex-wrap: wrap;
  }
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
</style>
