<template>
  <div class="permission-tree">
    <div
      v-for="group in groupedTree"
      :key="group.category"
      class="permission-group"
    >
      <div
        class="group-header"
        @click="toggleGroup(group.category)"
      >
        <span class="group-toggle">{{ expandedGroups[group.category] ? '▼' : '▶' }}</span>
        <span class="group-name">{{ group.category }}</span>
      </div>

      <div
        v-if="expandedGroups[group.category]"
        class="group-items"
      >
        <div
          v-for="node in group.items"
          :key="node.key"
          class="permission-item"
        >
          <label class="permission-label">
            <input
              type="checkbox"
              :checked="selected[node.key] === 'allow'"
              :disabled="readonly"
              @change="onToggle(node)"
            >
            <span class="permission-name">{{ node.display_name }}</span>
            <span
              v-if="node.granted_by_name"
              class="granted-by"
            >
              ({{ node.granted_by_name }})
            </span>
          </label>

          <div
            v-if="node.children && node.children.length"
            class="permission-children"
          >
            <div
              v-for="child in node.children"
              :key="child.key"
              class="permission-item child"
            >
              <label class="permission-label">
                <input
                  type="checkbox"
                  :checked="selected[child.key] === 'allow'"
                  :disabled="readonly"
                  @change="onToggle(child)"
                >
                <span class="permission-name">{{ child.display_name }}</span>
                <span
                  v-if="child.granted_by_name"
                  class="granted-by"
                >
                  ({{ child.granted_by_name }})
                </span>
              </label>
            </div>
          </div>
        </div>
      </div>
    </div>

    <p
      v-if="!tree || tree.length === 0"
      class="empty-message"
    >
      Нет доступных разрешений
    </p>
  </div>
</template>

<script>
export default {
  name: 'PermissionTree',

  props: {
    tree: { type: Array, default: () => [] },
    selected: { type: Object, default: () => ({}) },
    readonly: { type: Boolean, default: false },
  },

  emits: ['change'],

  data() {
    return {
      expandedGroups: {},
    }
  },

  computed: {
    groupedTree() {
      const groups = {}
      for (const node of this.tree) {
        const cat = node.category || 'Прочее'
        if (!groups[cat]) groups[cat] = { category: cat, items: [] }
        groups[cat].items.push(node)
      }
      return Object.values(groups)
    },
  },

  watch: {
    tree: {
      immediate: true,
      handler() {
        for (const group of this.groupedTree) {
          if (!(group.category in this.expandedGroups)) {
            this.expandedGroups[group.category] = true
          }
        }
      },
    },
  },

  methods: {
    toggleGroup(category) {
      this.expandedGroups[category] = !this.expandedGroups[category]
    },

    onToggle(node) {
      const currentValue = this.selected[node.key]
      const newValue = currentValue === 'allow' ? 'deny' : 'allow'
      this.$emit('change', node.key, newValue)
    },
  },
}
</script>

<style scoped>
.permission-tree {
  font-size: 14px;
}

.permission-group {
  margin-bottom: 12px;
}

.group-header {
  cursor: pointer;
  font-weight: 600;
  padding: 6px 0;
  user-select: none;
}

.group-toggle {
  display: inline-block;
  width: 16px;
  font-size: 10px;
}

.group-items {
  padding-left: 20px;
}

.permission-item {
  padding: 4px 0;
}

.permission-children {
  display: block;
}

.permission-item.child {
  padding-left: 20px;
}

.permission-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.permission-label input[type="checkbox"] {
  cursor: pointer;
}

.granted-by {
  color: var(--text-muted);
  font-size: 12px;
  margin-left: 4px;
}

.empty-message {
  color: var(--text-muted);
  font-style: italic;
}
</style>
