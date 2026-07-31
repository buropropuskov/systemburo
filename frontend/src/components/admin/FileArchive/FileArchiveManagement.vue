<template>
  <div class="file-archive-mgmt dashboard-card">
    <div class="management-header rt-header-inline">
      <div class="file-archive__title-group">
        <h3 class="management-title">
          Файловый архив
        </h3>
        <StatusBadge
          v-if="settings"
          :status="statusLabel"
        />
      </div>
      <div class="header-controls">
        <RefreshButton
          :loading="loading"
          @refresh="loadSettings"
        />
      </div>
    </div>

    <div class="file-archive__tabs">
      <button
        v-for="tab in TABS"
        :key="tab.key"
        type="button"
        class="file-archive__tab"
        :class="{ 'file-archive__tab--active': activeTab === tab.key }"
        @click="activeTab = tab.key"
      >
        {{ tab.label }}
      </button>
    </div>

    <div class="file-archive__body">
      <p
        v-if="loadError"
        class="file-archive__error"
      >
        {{ loadError }}
      </p>
      <template v-else>
        <!-- ArchiveStatusPanel/ArchiveSizeBreakdown - срез C2 -->
        <section
          v-if="activeTab === 'overview'"
          class="file-archive__panel"
        >
          <p class="file-archive__placeholder">
            Раздел «Обзор» появится в следующем срезе.
          </p>
        </section>
        <!-- ArchiveSettingsForm/TemplatePatternField - срез C3 -->
        <section
          v-else-if="activeTab === 'settings'"
          class="file-archive__panel"
        >
          <p class="file-archive__placeholder">
            Раздел «Настройки» появится в следующем срезе.
          </p>
        </section>
        <!-- ArchiveFailuresList - срез C4 -->
        <section
          v-else-if="activeTab === 'errors'"
          class="file-archive__panel"
        >
          <p class="file-archive__placeholder">
            Раздел «Ошибки» появится в следующем срезе.
          </p>
        </section>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import RefreshButton from '@/components/RefreshButton.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import { getArchiveSettings } from '@/api/fileArchive';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Каркас раздела «Файловый архив» (#1615, срез C1). Тела вкладок приезжают
 * следующими срезами (C2 «Обзор», C3 «Настройки», C4 «Ошибки») - здесь только
 * шапка с состоянием рубильника, вкладки и загрузка текущих настроек, чтобы
 * бейдж в шапке показывал реальный статус, а не заглушку.
 */
const TABS = [
  { key: 'overview', label: 'Обзор' },
  { key: 'settings', label: 'Настройки' },
  { key: 'errors', label: 'Ошибки' },
];

const activeTab = ref('overview');
const loading = ref(false);
const settings = ref(null);
const loadError = ref('');

const statusLabel = computed(() => (settings.value?.enabled ? 'Активен' : 'Неактивен'));

async function loadSettings() {
  loading.value = true;
  loadError.value = '';
  try {
    settings.value = await getArchiveSettings();
  } catch (e) {
    loadError.value = e?.message || 'Не удалось загрузить настройки файлового архива';
    useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'настройки файлового архива', type: 'error' });
  } finally {
    loading.value = false;
  }
}

onMounted(loadSettings);

defineExpose({ loadSettings });
</script>

<style scoped>
.file-archive-mgmt {
  background: var(--surface);
  border-radius: 16px;
  border: 1px solid var(--border);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  height: 50px;
  flex-shrink: 0;
}

.management-title {
  font-size: 1.2em;
  margin: 0;
  font-weight: 600;
  color: var(--text);
}

.file-archive__title-group {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* ===== ВКЛАДКИ (образец - StatisticsView.vue) ===== */
.file-archive__tabs {
  display: flex;
  gap: 0;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.file-archive__tab {
  border: none;
  background: transparent;
  font-family: inherit;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-muted);
  padding: 14px 18px;
  cursor: pointer;
  position: relative;
  transition: color 0.18s ease;
}

.file-archive__tab:hover {
  color: var(--text);
}

.file-archive__tab--active {
  color: var(--accent-text);
}

.file-archive__tab--active::after {
  content: '';
  position: absolute;
  left: 14px;
  right: 14px;
  bottom: -1px;
  height: 3px;
  border-radius: 3px 3px 0 0;
  background: var(--accent);
}

/* ===== ТЕЛО ===== */
.file-archive__body {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.file-archive__panel {
  padding: 26px 28px 40px;
}

.file-archive__placeholder {
  color: var(--text-muted);
  font-size: 14px;
  margin: 0;
}

.file-archive__error {
  padding: 26px 28px 0;
  color: var(--danger-text);
  font-size: 14px;
}

/* ===== МОБИЛКА (<=768): каркас без переполнения (образец - StatisticsView.vue) ===== */
@media (max-width: 768px) {
  .management-header {
    padding: 0 12px;
  }

  .file-archive__tabs {
    padding: 0 12px;
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .file-archive__tabs::-webkit-scrollbar {
    display: none;
  }

  .file-archive__tab {
    flex-shrink: 0;
    white-space: nowrap;
    padding: 12px 14px;
    font-size: 14px;
  }

  .file-archive__tab--active::after {
    bottom: 0;
  }

  .file-archive__panel {
    padding: 14px 12px 24px;
  }
}
</style>
