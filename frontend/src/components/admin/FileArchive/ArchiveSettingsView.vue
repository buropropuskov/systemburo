<template>
  <section class="asv">
    <header class="asv__head">
      <h3 class="asv__title">
        Как настроено
      </h3>
      <StatusBadge :status="settings.enabled ? 'Активен' : 'Неактивен'" />
    </header>

    <dl class="asv__list">
      <div
        v-for="row in rows"
        :key="row.label"
        class="asv__row"
      >
        <dt class="asv__label">
          {{ row.label }}
        </dt>
        <dd class="asv__value">
          {{ row.value }}
        </dd>
      </div>
    </dl>

    <p class="asv__hint">
      Раскладка каталогов и пороги задаются на сервере командой
      <code>server archive</code>: это настройка хранения, её делает тот, кто
      разворачивает систему. Здесь показано действующее значение.
    </p>
  </section>
</template>

<script setup>
/**
 * Действующие настройки файлового архива, только для чтения (#1615).
 *
 * Форма правки отсюда убрана намеренно: сменённый шаблон каталогов переносит дерево
 * заявок целиком, то есть по последствиям близок к смене самого корня архива, а
 * корень и так живёт в переменной окружения. Держать такое за веб-сессией
 * администратора бюро значит отдавать управление хранилищем персональных данных
 * тому, чья работа - пропуска. Показ остаётся: дежурному нужно видеть, куда пишутся
 * файлы, не заходя на сервер.
 */
import { computed } from 'vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import { formatBytes } from '@/utils/download';

const props = defineProps({
  settings: { type: Object, required: true },
});

const rows = computed(() => {
  const s = props.settings || {};
  return [
    { label: 'Шаблон каталогов', value: s.dir_template || '-' },
    { label: 'Шаблон имени файла', value: s.file_template || '-' },
    {
      label: 'Предельный объём архива',
      value: s.quota_bytes > 0 ? formatBytes(s.quota_bytes) : 'без ограничения',
    },
    { label: 'Остаётся свободным не меньше', value: formatBytes(s.min_free_bytes || 0) },
    { label: 'Порог предупреждения', value: `${s.warn_percent ?? 0} %` },
    { label: 'Окно ночной сверки', value: `${s.recheck_days ?? 0} дн.` },
    { label: 'Заморозка через', value: `${s.freeze_after_days ?? 0} дн. после окончания заявки` },
    { label: 'Потолок одной выгрузки', value: formatBytes(s.zip_max_bytes || 0) },
  ];
});
</script>

<style scoped>
.asv {
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 16px;
  background: var(--surface);
}

.asv__head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.asv__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
}

.asv__list {
  margin: 0;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 8px 24px;
}

.asv__row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 6px 0;
  border-bottom: 1px solid var(--border);
}

.asv__label {
  color: var(--text-muted);
  font-size: 13px;
}

.asv__value {
  margin: 0;
  color: var(--text);
  font-size: 13px;
  text-align: right;
  word-break: break-word;
}

.asv__hint {
  margin: 12px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}

code {
  font-family: inherit;
  background: var(--surface-2);
  border-radius: 6px;
  padding: 1px 6px;
}
</style>
