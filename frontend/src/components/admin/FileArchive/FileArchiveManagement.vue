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
          @refresh="onRefresh"
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
        <!-- v-show, не v-if/else-if: переключение вкладок не должно сбрасывать
             выбранный период и оценку выгрузки - секция остаётся смонтированной. -->
        <section
          v-show="activeTab === 'overview'"
          class="file-archive__panel file-archive__panel--overview"
        >
          <ArchiveStatusPanel ref="overviewRef" />
          <hr class="file-archive__divider">
          <ArchiveDownloadPanel />
          <hr class="file-archive__divider">
          <ArchiveBackfillPanel />
          <hr class="file-archive__divider">
          <ArchiveSettingsView
            v-if="settings"
            :settings="settings"
          />
        </section>
        <section
          v-show="activeTab === 'errors'"
          class="file-archive__panel"
        >
          <ArchiveFailuresList ref="failuresRef" />
        </section>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import RefreshButton from '@/components/RefreshButton.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import ArchiveStatusPanel from './ArchiveStatusPanel.vue';
import ArchiveSettingsView from './ArchiveSettingsView.vue';
import ArchiveDownloadPanel from './ArchiveDownloadPanel.vue';
import ArchiveBackfillPanel from './ArchiveBackfillPanel.vue';
import ArchiveFailuresList from './ArchiveFailuresList.vue';
import { getArchiveSettings } from '@/api/fileArchive';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Каркас раздела «Файловый архив» (#1615). Две вкладки: «Обзор» - состояние места,
 * разбивка по периодам, выгрузка за период, донаполнение и справка о действующих
 * настройках; «Лента» - записи реестра: что записано, что ждёт очереди, что не вышло.
 *
 * Вкладки правки настроек здесь нет: раскладку каталогов и пороги задаёт команда
 * server archive на сервере. Текущие настройки всё равно загружаются - по ним
 * бейдж в шапке показывает, идёт выгрузка или выключена.
 */
const TABS = [
  { key: 'overview', label: 'Обзор' },
  { key: 'errors', label: 'Лента' },
];

const activeTab = ref('overview');
const loading = ref(false);
const settings = ref(null);
const loadError = ref('');
const overviewRef = ref(null);
const failuresRef = ref(null);

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

// Шапка обновляет рубильник всегда (бейдж статуса виден на любой вкладке) и
// дополнительно данные активной вкладки - «Настройки» перечитывает свои данные
// сама при монтировании (не держит смонтированное состояние вне своей вкладки).
function onRefresh() {
  loadSettings();
  if (activeTab.value === 'overview') overviewRef.value?.refresh();
  if (activeTab.value === 'errors') failuresRef.value?.refresh();
}

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

.file-archive__panel--overview {
  display: flex;
  flex-direction: column;
  gap: 26px;
}

.file-archive__divider {
  border: none;
  border-top: 1px solid var(--border);
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
    padding: 0 var(--gutter);
  }

  .file-archive__tabs {
    padding: 0 var(--gutter);
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .file-archive__tabs::-webkit-scrollbar {
    display: none;
  }

  /* min-height - тач-таргет 44px: одного padding 12px хватало только на 42. */
  .file-archive__tab {
    flex-shrink: 0;
    white-space: nowrap;
    min-height: 44px;
    padding: 12px 14px;
    font-size: 14px;
  }

  .file-archive__tab--active::after {
    bottom: 0;
  }

  .file-archive__panel {
    padding: 14px var(--gutter) 24px;
  }
}
</style>
