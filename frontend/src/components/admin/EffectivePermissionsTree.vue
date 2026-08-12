<template>
  <div class="ep-tree">
    <section
      v-for="section in sections"
      :key="section.category"
      class="ep-section"
    >
      <div class="ep-section__title">
        {{ section.category }}
      </div>

      <template
        v-for="entry in section.entries"
        :key="entry.id"
      >
        <div
          v-if="entry.type === 'table'"
          class="ep-group"
          :data-table="entry.slug"
        >
          <div class="ep-group__head">
            <button
              type="button"
              class="ep-group__toggle"
              :aria-expanded="isOpen(entry) ? 'true' : 'false'"
              :aria-controls="entry.domId"
              :aria-label="`${entry.name}: выдано ${entry.granted} из ${entry.total}`"
              @click="toggleGroup(entry.id)"
            >
              <svg
                class="ep-group__chevron"
                :class="{ 'ep-group__chevron--open': isOpen(entry) }"
                width="10"
                height="10"
                viewBox="0 0 10 10"
                fill="none"
                aria-hidden="true"
              >
                <path
                  d="M3 1.5L6.5 5L3 8.5"
                  stroke="currentColor"
                  stroke-width="1.6"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              <span class="ep-group__name">{{ entry.name }}</span>
              <span class="ep-group__count">{{ entry.granted }} из {{ entry.total }}</span>
            </button>
            <button
              v-if="entry.togglable"
              type="button"
              class="ep-group__all"
              @click="toggleAll(entry)"
            >
              {{ entry.allOn ? 'Снять все' : 'Выбрать все' }}
            </button>
          </div>

          <div
            class="ep-group__body"
            :class="{ 'ep-group__body--open': isOpen(entry) }"
          >
            <div
              :id="entry.domId"
              class="ep-group__inner"
              :inert="isOpen(entry) ? null : true"
            >
              <EffectivePermissionRow
                v-for="verb in entry.nodes"
                :key="verb.node.key"
                :node="verb.node"
                :state="stateOf(verb.node.key)"
                :label="verb.label"
                modifier="verb"
                @toggle="onToggle"
              />
            </div>
          </div>
        </div>

        <template v-else>
          <EffectivePermissionRow
            :node="entry.node"
            :state="stateOf(entry.node.key)"
            @toggle="onToggle"
          />
          <EffectivePermissionRow
            v-for="child in entry.node.children || []"
            :key="child.key"
            :node="child"
            :state="stateOf(child.key)"
            modifier="child"
            @toggle="onToggle"
          />
        </template>
      </template>
    </section>

    <p
      v-if="sections.length === 0"
      class="ep-empty"
    >
      Нет доступных прав
    </p>
  </div>
</template>

<script>
import EffectivePermissionRow from './EffectivePermissionRow.vue';
import { parseTableKey, parseTablePermission } from '@/utils/permissionCatalog';

/**
 * Презентационный правый столбец модалки прав: дерево эффективных прав из каталога
 * с бейджем источника (роль/группа/лично/админ) и тумблером на каждое право.
 * Вся бизнес-логика (режим/наследование/override) считается в UserAccessModal и
 * приходит готовой в stateByKey -- здесь только отрисовка.
 *
 * Права системных таблиц (table.<slug>.<verb>) собираются во второй уровень: на
 * таблицу их приходит десяток, и плоским списком один пост занимал 20 строк
 * прокрутки (#1880). Статические категории раскрыты всегда, свёрнуты только
 * таблицы; expandAll раскрывает всё принудительно -- потребители включают его на
 * время поиска, чтобы найденное право было видно и кликабельно сразу.
 */
export default {
  name: 'EffectivePermissionsTree',
  components: { EffectivePermissionRow },
  props: {
    catalog: { type: Array, default: () => [] },
    // key -> { on: boolean, source: 'role'|'group'|'override'|'admin'|null, locked: boolean }
    stateByKey: { type: Object, default: () => ({}) },
    expandAll: { type: Boolean, default: false },
  },
  emits: ['toggle'],
  data() {
    return {
      openGroups: new Set(),
    };
  },
  computed: {
    sections() {
      const order = [];
      const byCat = new Map();
      for (const node of this.catalog) {
        const cat = node.category || 'Прочее';
        if (!byCat.has(cat)) {
          byCat.set(cat, []);
          order.push(cat);
        }
        byCat.get(cat).push(node);
      }
      return order.map((category) => ({
        category,
        entries: this.buildEntries(byCat.get(category)),
      }));
    },
  },
  methods: {
    stateOf(key) {
      return this.stateByKey[key] || { on: false, source: null, locked: false };
    },
    /**
     * Строки секции: право таблицы уходит в группу по слагу (позицию занимает
     * первое право этой таблицы), всё остальное остаётся строкой верхнего уровня.
     *
     * Группа собирается по слагу из ключа, а не по успеху полного разбора: право
     * с глаголом вне словаря (в базе живут legacy `table.<slug>.edit`) иначе
     * висело бы отдельной строкой рядом со своей же таблицей, и администратор
     * читал бы её как поломку. Такому праву достаётся пустая подпись -- строка
     * покажет display_name от бэкенда, единственный источник смысла для
     * незнакомого глагола. Слаг из ключа не выводится -- строка остаётся наверху.
     */
    buildEntries(nodes) {
      const entries = [];
      const groups = new Map();
      for (const node of nodes) {
        const parsed = parseTableKey(node.key);
        if (!parsed) {
          entries.push({ type: 'node', id: node.key, node });
          continue;
        }
        const table = parseTablePermission(node);
        let group = groups.get(parsed.slug);
        if (!group) {
          group = {
            type: 'table',
            id: `table::${parsed.slug}`,
            domId: `ep-group-${parsed.slug.replace(/[^a-zA-Z0-9_-]/g, '-')}`,
            slug: parsed.slug,
            name: '',
            nodes: [],
          };
          groups.set(parsed.slug, group);
          entries.push(group);
        }
        if (!group.name && table) group.name = table.tableName;
        group.nodes.push({ node, label: table ? table.verbTitle : '' });
      }
      for (const group of groups.values()) {
        // Ни одно право таблицы не разобралось целиком -- живого имени взять
        // неоткуда, показываем слаг: он хотя бы совпадает с ключами внутри.
        if (!group.name) group.name = group.slug;
        this.countGroup(group);
      }
      return entries;
    },
    /**
     * Счётчик группы. Заблокированные права идут в «выдано» (они действуют, просто
     * приходят из роли или группы), но не в число переключаемых. Знаменатель --
     * число прав этой таблицы, реально пришедших в catalog, а не длина словаря
     * глаголов: новый глагол на бэкенде увеличивает его сам, а при активном поиске
     * счётчик описывает ровно видимые строки, а не всю таблицу.
     */
    countGroup(group) {
      let granted = 0;
      let togglable = 0;
      let togglableOn = 0;
      for (const { node } of group.nodes) {
        const st = this.stateOf(node.key);
        if (st.on) granted += 1;
        if (!st.locked) {
          togglable += 1;
          if (st.on) togglableOn += 1;
        }
      }
      group.granted = granted;
      group.total = group.nodes.length;
      group.togglable = togglable;
      group.allOn = togglable > 0 && togglableOn === togglable;
    },
    isOpen(group) {
      return this.expandAll || this.openGroups.has(group.id);
    },
    toggleGroup(id) {
      const next = new Set(this.openGroups);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      this.openGroups = next;
    },
    /**
     * «Выбрать все» по таблице. Заблокированные права пропускаем -- их снимают
     * только там, откуда они пришли; уже полный набор клик снимает.
     */
    toggleAll(group) {
      const turnOn = !group.allOn;
      for (const { node } of group.nodes) {
        const st = this.stateOf(node.key);
        if (st.locked || st.on === turnOn) continue;
        this.$emit('toggle', node.key);
      }
    },
    onToggle(key) {
      if (this.stateOf(key).locked) return;
      this.$emit('toggle', key);
    },
  },
};
</script>

<style scoped>
.ep-section {
  margin-top: 18px;
}

.ep-section:first-child {
  margin-top: 0;
}

.ep-section__title {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  color: var(--color-text-muted);
  padding-bottom: 8px;
  margin-bottom: 4px;
  border-bottom: 1px solid var(--color-border);
}

/* --- Свёрнутая таблица --------------------------------------------------- */
.ep-group {
  margin: 6px 0;
  border-radius: var(--radius-md);
  background: var(--surface-2);
}

.ep-group__head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 10px 4px 6px;
}

.ep-group__toggle {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 8px 4px;
  border: none;
  background: transparent;
  font-family: inherit;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
  text-align: left;
  cursor: pointer;
}

.ep-group__chevron {
  flex: none;
  color: var(--color-text-muted);
  transition: transform 0.2s ease;
}

.ep-group__chevron--open {
  transform: rotate(90deg);
}

.ep-group__name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ep-group__count {
  flex: none;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--color-text-muted);
}

.ep-group__all {
  flex: none;
  border: none;
  background: transparent;
  padding: 6px 10px;
  border-radius: var(--radius-pill);
  font-family: inherit;
  font-size: 12px;
  font-weight: 600;
  color: var(--accent-text);
  cursor: pointer;
  transition: background 0.15s ease;
}

.ep-group__all:hover {
  background: var(--accent-tint);
}

/* Раскрытие через grid-template-rows 0fr<->1fr, как выпадающие списки NavMenu:
   высота анимируется плавно и двигает то, что ниже. */
.ep-group__body {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows 0.22s ease;
}

.ep-group__body--open {
  grid-template-rows: 1fr;
}

/* Ни padding, ни margin: box-sizing здесь content-box, и они пережили бы
   схлопывание строки грида видимой полосой. Отступы несут сами строки. */
.ep-group__inner {
  overflow: hidden;
  min-height: 0;
}

.ep-empty {
  color: var(--color-text-muted);
  font-style: italic;
  font-size: 13px;
}
</style>
