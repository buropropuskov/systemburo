<template>
  <section class="asv">
    <header class="asv__head">
      <h3 class="asv__title">
        Как настроено
      </h3>
      <StatusBadge :status="settings.enabled ? 'Активен' : 'Неактивен'" />
    </header>

    <div class="asv__layout">
      <div class="asv__block">
        <h4 class="asv__block-title">
          Куда лягут файлы
        </h4>
        <ol class="asv__levels">
          <li
            v-for="(level, index) in dirLevels"
            :key="index"
            class="asv__level"
          >
            {{ level }}
          </li>
        </ol>
        <p class="asv__file-rule">
          Имя файла: {{ fileRule }}
        </p>
        <p class="asv__example">
          Например: <code>{{ pathExample }}</code>
        </p>
      </div>

      <div class="asv__block">
        <h4 class="asv__block-title">
          Пороги и сроки
        </h4>
        <dl class="asv__list">
          <div
            v-for="row in limitRows"
            :key="row.label"
            class="asv__row"
          >
            <dt class="asv__label">
              {{ row.label }}
              <span class="asv__value">{{ row.value }}</span>
            </dt>
            <dd class="asv__meaning">
              {{ row.meaning }}
            </dd>
          </div>
        </dl>
      </div>
    </div>

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
 *
 * Значения показываются словами, а не в том виде, в каком их задают в консоли
 * (followup, срез S6): шаблон с фигурными скобками и голое число порога отвечают
 * на вопрос «что записано в настройке», а читающему нужен ответ на вопрос «что
 * система сделает».
 */
import { computed } from 'vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import { formatBytes } from '@/utils/download';
import { describeDirTemplate, describeTemplatePart, renderTemplateExample } from '@/utils/archiveTemplate';

const props = defineProps({
  settings: { type: Object, required: true },
});

const dirLevels = computed(() => describeDirTemplate(props.settings?.dir_template));

const pathExample = computed(() => {
  const dir = renderTemplateExample(props.settings?.dir_template);
  // Концевые точки и пробелы срезает сервер (Windows отбрасывает точку в конце
  // имени), поэтому «Иванов И.И.» + «.xlsx» на диске даёт одну точку, не две.
  // Пример обязан совпадать с тем, что реально ляжет в каталог.
  const file = renderTemplateExample(props.settings?.file_template).replace(/[.\s]+$/, '');
  return `${dir}/${file}.xlsx`;
});

const fileRule = computed(() => describeTemplatePart(props.settings?.file_template) || 'не задано');

const limitRows = computed(() => {
  const s = props.settings || {};
  const quota = s.quota_bytes > 0 ? formatBytes(s.quota_bytes) : 'без ограничения';
  const minFree = formatBytes(s.min_free_bytes || 0);
  const zipMax = formatBytes(s.zip_max_bytes || 0);
  return [
    {
      label: 'Предельный объём архива',
      value: quota,
      meaning: s.quota_bytes > 0
        ? `Когда архив дорастёт до ${quota}, запись новых бланков остановится.`
        : 'Объём не ограничен - запись остановит только нехватка места на диске.',
    },
    {
      label: 'Наименьший остаток на диске',
      value: minFree,
      meaning: `Свободного места стало меньше ${minFree} - очередь встаёт. Освободите место, дальше она пойдёт сама.`,
    },
    {
      label: 'Порог предупреждения',
      value: `${s.warn_percent ?? 0} %`,
      meaning: `При заполнении раздела на ${s.warn_percent ?? 0} % ответственным придёт уведомление, запись продолжится.`,
    },
    {
      label: 'Ночная сверка',
      value: `${s.recheck_days ?? 0} дн.`,
      meaning: `Каждую ночь система сверяет с диском заявки за последние ${s.recheck_days ?? 0} дн. и дописывает пропавшие файлы.`,
    },
    {
      label: 'Заморозка файлов',
      value: `${s.freeze_after_days ?? 0} дн.`,
      meaning: `Через ${s.freeze_after_days ?? 0} дн. после окончания заявки файлы считаются окончательными и больше не переписываются - в том числе кнопкой «Пересобрать».`,
    },
    {
      label: 'Потолок одной выгрузки',
      value: zipMax,
      meaning: `За один раз скачивается не больше ${zipMax}; за больший период архив забирается частями.`,
    },
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

.asv__layout {
  display: grid;
  grid-template-columns: minmax(240px, 1fr) minmax(320px, 1.5fr);
  gap: 24px;
}

.asv__block-title {
  margin: 0 0 10px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted);
}

/* Уровни каталогов пронумерованы: порядок здесь и есть содержание - он повторяет
   вложенность папок на диске сверху вниз. */
.asv__levels {
  margin: 0;
  padding-left: 20px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 14px;
  color: var(--text);
}

.asv__file-rule {
  margin: 12px 0 0;
  font-size: 14px;
  color: var(--text);
}

.asv__example {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--text-muted);
  overflow-wrap: anywhere;
}

.asv__list {
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.asv__row {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.asv__label {
  display: flex;
  align-items: baseline;
  gap: 8px;
  flex-wrap: wrap;
  color: var(--text-muted);
  font-size: 13px;
}

.asv__value {
  color: var(--text);
  font-size: 14px;
  font-weight: 600;
}

.asv__meaning {
  margin: 0;
  color: var(--text-muted);
  font-size: 13px;
}

.asv__hint {
  margin: 16px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}

code {
  font-family: inherit;
  padding: 1px 6px;
  border-radius: 6px;
  background: var(--surface-2);
}

@media (max-width: 767.98px) {
  .asv__layout {
    grid-template-columns: 1fr;
    gap: 20px;
  }
}
</style>
