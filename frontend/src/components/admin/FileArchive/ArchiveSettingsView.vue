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
        <!-- Дерево вместо перечисления уровней: раскладка папок объясняется
             самой раскладкой, а не описанием раскладки словами. -->
        <div class="asv__tree">
          <div
            v-for="(node, index) in treeNodes"
            :key="index"
            class="asv__tree-node"
            :style="{ paddingLeft: index * 14 + 'px' }"
          >
            <span
              v-if="index"
              class="asv__tree-branch"
              aria-hidden="true"
            >└</span>
            <span :class="node.file ? 'asv__tree-file' : 'asv__tree-dir'">{{ node.text }}</span>
          </div>
        </div>
        <p class="asv__example">
          Пример собран на образцах значений: у настоящей заявки здесь её
          собственные дата, номер и организация.
        </p>
      </div>

      <div class="asv__block">
        <h4 class="asv__block-title">
          Что и когда делает система
        </h4>
        <dl class="asv__list">
          <div
            v-for="row in limitRows"
            :key="row.when"
            class="asv__row"
          >
            <dt class="asv__when">
              {{ row.when }}
            </dt>
            <dd class="asv__then">
              {{ row.then }}
            </dd>
          </div>
        </dl>
        <p
          v-if="!settings.quota_bytes"
          class="asv__note"
        >
          Предельный объём архива не задан: запись остановит только нехватка места.
        </p>
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
import { renderTemplateExample } from '@/utils/archiveTemplate';

const props = defineProps({
  settings: { type: Object, required: true },
});

// Уровни дерева: каталоги из шаблона плюс имя файла последней строкой. Показываем
// готовые имена, а не описания вроде «месяц двумя цифрами» - по образцу видно и
// порядок вложенности, и вид каждого имени сразу.
const treeNodes = computed(() => {
  const dir = renderTemplateExample(props.settings?.dir_template);
  const levels = String(dir || '').split('/').map((p) => p.trim()).filter(Boolean);
  // Концевые точки и пробелы срезает сервер (Windows отбрасывает точку в конце
  // имени), поэтому «Иванов И.И.» + «.xlsx» на диске даёт одну точку, не две.
  const file = renderTemplateExample(props.settings?.file_template).replace(/[.\s]+$/, '');
  const nodes = levels.map((text) => ({ text, file: false }));
  if (file) nodes.push({ text: `${file}.xlsx`, file: true });
  return nodes;
});

// Каждая строка - пара «когда» и «что тогда»: администратор читает этот блок,
// когда что-то встало, и ему нужно сопоставить наблюдаемое с настройкой, а не
// прочесть абзац про каждую. Отсюда телеграфный слог и никаких предложений.
const limitRows = computed(() => {
  const s = props.settings || {};
  const minFree = formatBytes(s.min_free_bytes || 0);
  const rows = [
    {
      when: `Свободно меньше ${minFree}`,
      then: 'запись встаёт, пойдёт сама, когда место освободится',
    },
    {
      when: `Раздел занят на ${s.warn_percent ?? 0} %`,
      then: 'уведомление ответственным, запись продолжается',
    },
    {
      when: 'Каждую ночь',
      then: `сверка с диском за последние ${s.recheck_days ?? 0} дн., пропавшее дописывается`,
    },
    {
      when: `Через ${s.freeze_after_days ?? 0} дн. после окончания заявки`,
      then: 'файлы замораживаются и больше не переписываются',
    },
    {
      when: 'Одна выгрузка',
      then: `не больше ${formatBytes(s.zip_max_bytes || 0)}, дальше период сужают`,
    },
  ];
  // Строку про объём показываем, только когда предел задан: «не ограничен» -
  // это отсутствие правила, и в перечне правил оно лишнее.
  if (s.quota_bytes > 0) {
    rows.unshift({
      when: `Архив дорос до ${formatBytes(s.quota_bytes)}`,
      then: 'запись новых бланков останавливается',
    });
  }
  return rows;
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

/* Дерево: вложенность показывается сдвигом, как в проводнике. */
.asv__tree {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 13px;
}

.asv__tree-node {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

/* Имена папок короткие и переносить их незачем, а вот имя файла из четырёх
   склеенных значений длиннее колонки: пусть переносится, чем уезжает в скролл
   или прячется под многоточием - в него и надо всмотреться. */
.asv__tree-node > span:last-child {
  overflow-wrap: anywhere;
}

.asv__tree-branch {
  color: var(--text-muted);
}

.asv__tree-dir {
  color: var(--text);
}

/* Имя файла отличается от папок цветом: это конец пути, дальше вложенности нет. */
.asv__tree-file {
  color: var(--accent);
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
  gap: 10px;
}

.asv__row {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.asv__when {
  color: var(--text);
  font-size: 13px;
  font-weight: 600;
}

.asv__then {
  margin: 0;
  color: var(--text-muted);
  font-size: 13px;
}

.asv__note {
  margin: 12px 0 0;
  color: var(--text-muted);
  font-size: 12px;
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
