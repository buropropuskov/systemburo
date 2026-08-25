<template>
  <div
    class="ep-row"
    :class="[modifierClass, { 'ep-row--locked': state.locked }]"
    :data-key="node.key"
  >
    <span class="ep-row__label" :title="node.description || undefined">
      {{ label || node.display_name }}
      <small v-if="node.super_only">только Системный администратор</small>
    </span>
    <span
      v-if="badge"
      class="src"
      :class="`src--${badge}`"
    >
      {{ badgeLabel }}
    </span>
    <button
      type="button"
      class="tgl"
      :class="{ on: state.on, locked: state.locked }"
      :disabled="state.locked"
      :aria-pressed="state.on"
      :aria-label="node.display_name"
      @click="onClick"
    />
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
 * Одна строка каталога прав: подпись, бейдж источника и тумблер.
 * Вынесена из EffectivePermissionsTree, потому что теперь рисуется в трёх
 * местах -- верхний уровень секции, дочерний узел каталога и глагол внутри
 * свёрнутой таблицы (#1880). Контракт для e2e и спек: data-key на корне,
 * кнопка с классом .tgl и aria-pressed.
 */
export default {
  name: 'EffectivePermissionRow',
  props: {
    node: { type: Object, required: true },
    // { on: boolean, source: 'role'|'group'|'override'|'admin'|null, locked: boolean }
    state: { type: Object, required: true },
    // '' -- обычная строка, 'child' -- дочерний узел каталога, 'verb' -- право таблицы.
    modifier: { type: String, default: '' },
    // Короткая подпись вместо display_name: внутри группы таблицы её имя уже
    // стоит на свёрнутой строке, и повторять его в каждом глаголе незачем.
    // aria-label и подсказки читалок при этом остаются полными.
    label: { type: String, default: '' },
  },
  emits: ['toggle'],
  computed: {
    modifierClass() {
      return this.modifier ? `ep-row--${this.modifier}` : null;
    },
    badge() {
      return this.state.on ? this.state.source : null;
    },
    badgeLabel() {
      return SRC_LABELS[this.badge] || this.badge;
    },
  },
  methods: {
    onClick() {
      if (this.state.locked) return;
      this.$emit('toggle', this.node.key);
    },
  },
};
</script>

<style scoped>
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

/* Глагол внутри свёрнутой таблицы. Строка лежит на подложке группы
   (--surface-2), поэтому обычная подсветка наведением совпала бы с фоном. */
.ep-row--verb {
  margin: 0 10px;
  padding: 7px 8px 7px 24px;
}

.ep-row--verb:last-child {
  margin-bottom: 6px;
}

.ep-row--verb .ep-row__label {
  font-size: 13px;
  font-weight: 500;
}

.ep-row--verb:hover {
  background: var(--accent-tint);
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
.src--admin { background: #e9f9ef; color: var(--success-text); }

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
</style>
