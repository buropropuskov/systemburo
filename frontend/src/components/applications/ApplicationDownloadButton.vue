<script>
import AppIcon from '@/components/icons/AppIcon.vue';

/** Both application lists open the same modal state; no request or permission change. */
export function openApplicationDownload(state, application) {
  state.downloadAppId = application.id;
  state.downloadAppInfo = application;
  state.showDownloadModal = true;
}

export default { components: { AppIcon } };
</script>

<template>
  <button
    class="download-btn"
    title="Скачать"
  >
    <AppIcon
      name="download"
      class="application-download-icon"
    />
    <span class="application-download-label">Скачать</span>
  </button>
</template>
