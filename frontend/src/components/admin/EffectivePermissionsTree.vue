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
        v-for="node in section.nodes"
        :key="node.key"
      >
        <div
          class="ep-row"
          :class="{ 'ep-row--locked': stateOf(node.key).locked }"
          :data-key="node.key"
        >
          <span class="ep-row__label">
            {{ node.display_name }}
            <small v-if="node.super_only">только Системный администратор</small>
          </span>
          <span
            v-if="badgeFor(node.key)"
            class="src"
            :class="`src--${badgeFor(node.key)}`"
          >
            {{ srcLabel(badgeFor(node.key)) }}
          </span>
          <button
            type="button"
            class="tgl"
            :class="{ on: stateOf(node.key).on, locked: stateOf(node.key).locked }"
            :disabled="stateOf(node.key).locked"
            :aria-pressed="stateOf(node.key).on"
            :aria-label="node.display_name"
            @click="onToggle(node.key)"
          />
        </div>

        <div
          v-for="child in node.children || []"
          :key="child.key"
          class="ep-row ep-row--child"
          :class="{ 'ep-row--locked': stateOf(child.key).locked }"
          :data-key="child.key"
        >
          <span class="ep-row__label">{{ child.display_name }}</span>
          <span
            v-if="badgeFor(child.key)"
            class="src"
            :class="`src--${badgeFor(child.key)}`"
          >
            {{ srcLabel(badgeFor(child.key)) }}
          </span>
          <button
            type="button"
            class="tgl"
            :class="{ on: stateOf(child.key).on, locked: stateOf(child.key).locked }"
            :disabled="stateOf(child.key).locked"
            :aria-pressed="stateOf(child.key).on"
            :aria-label="child.display_name"
            @click="onToggle(child.key)"
          />
        </div>
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
const SRC_LABELS = {
  role: 'роль',
  group: 'группа',
  override: 'лично',
  admin: 'админ',
};

/**
 * Презентационный правый столбец модалки прав: дерево эффективных прав из каталога
 * с бейджем источника (роль/группа/лично/админ) и тумблером на каждое право.
 * Вся бизнес-логика (режим/наследование/override) считается в UserAccessModal и
 * приходит готовой в stateByKey -- здесь только отрисовка. Изолирован от
 * PermissionTree.vue (тот шарится с AdminPermissionGroups в checkbox-виде).
 */
export default {
  name: 'EffectivePermissionsTree',
  props: {
    catalog: { type: Array, default: () => [] },
    // key -> { on: boolean, source: 'role'|'group'|'override'|'admin'|null, locked: boolean }
    stateByKey: { type: Object, default: () => ({}) },
  },
  emits: ['toggle'],
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
      return order.map((category) => ({ category, nodes: byCat.get(category) }));
    },
  },
  methods: {
    stateOf(key) {
      return this.stateByKey[key] || { on: false, source: null, locked: false };
    },
    badgeFor(key) {
      const st = this.stateOf(key);
      return st.on ? st.source : null;
    },
    srcLabel(source) {
      return SRC_LABELS[source] || source;
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

.ep-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 9px 6px;
  border-radius: 12px;
  transition: background 0.12s ease;
}

.ep-row:hover {
  background: var(--color-bg);
}

.ep-row--child {
  padding-left: 26px;
}

.ep-row--child .ep-row__label {
  font-size: 13px;
  color: var(--accent-text);
}

.ep-row--locked {
  opacity: 0.65;
}

.ep-row__label {
  flex: 1;
  font-size: 14px;
  font-weight: 500;
}

.ep-row__label small {
  display: block;
  font-size: 11.5px;
  color: var(--color-text-muted);
  font-weight: 500;
  margin-top: 1px;
}

.src {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.02em;
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  text-transform: lowercase;
  white-space: nowrap;
}

.src--role { background: #eef0f6; color: #6b7280; }
.src--group { background: var(--color-primary-tint); color: var(--accent-text); }
.src--override { background: #fff4e3; color: #e8870c; }
.src--admin { background: #e9f9ef; color: var(--color-success); }

.tgl {
  --w: 40px;
  --h: 23px;
  --d: 17px;
  width: var(--w);
  height: var(--h);
  flex: none;
  border-radius: var(--radius-pill);
  background: var(--border);
  position: relative;
  cursor: pointer;
  border: none;
  padding: 0;
  transition: background 0.2s ease;
}

.tgl::after {
  content: '';
  position: absolute;
  top: 3px;
  left: 3px;
  width: var(--d);
  height: var(--d);
  border-radius: 50%;
  background: var(--surface);
  box-shadow: 0 1px 3px var(--shadow-drop);
  transition: left 0.2s ease;
}

.tgl.on { background: var(--color-primary); }
.tgl.on::after { left: calc(var(--w) - var(--d) - 3px); }

.tgl.locked {
  background: var(--accent-tint);
  cursor: not-allowed;
  opacity: 0.7;
}

.tgl.locked.on { background: #b9bedd; }

.ep-empty {
  color: var(--color-text-muted);
  font-style: italic;
  font-size: 13px;
}
</style>
