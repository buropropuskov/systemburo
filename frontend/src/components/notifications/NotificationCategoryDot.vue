<template>
  <span
    class="notif-category-dot"
    :class="'notif-category-dot--' + category"
    :title="label"
    :aria-label="label"
    role="img"
  />
</template>

<script setup>
// Цветовая метка категории уведомления в списке (#1748 S7) - общая для колокольчика
// и блока личного кабинета, чтобы не заводить второй способ показать категорию.
import { computed } from 'vue';
import { notificationCategory, notificationCategoryLabel } from '@/utils/notificationDetails';

const props = defineProps({
  type: { type: String, default: null },
});

const category = computed(() => notificationCategory(props.type));
const label = computed(() => notificationCategoryLabel(category.value));
</script>

<style scoped>
.notif-category-dot {
  display: inline-block;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.notif-category-dot--application { background: var(--accent); }
.notif-category-dot--security { background: var(--danger); }
.notif-category-dot--passage { background: var(--warning); }
.notif-category-dot--content { background: var(--success); }
.notif-category-dot--system { background: var(--text-muted); }
</style>
