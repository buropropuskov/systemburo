<template>
  <BaseModal :show="show" title="Объявление" @close="$emit('close')">
    <template v-if="announcement">
      <span v-if="announcement.is_important" class="announcement-badge">Важно</span>

      <h4 class="announcement-title">{{ announcement.title }}</h4>

      <p class="announcement-description">{{ announcement.description }}</p>

      <p
        v-if="announcement.full_text && announcement.full_text !== announcement.description"
        class="announcement-full-text"
      >
        {{ announcement.full_text }}
      </p>

      <time class="announcement-date">
        {{ formatDate(announcement.created_at) }}
      </time>
    </template>

    <template #actions>
      <button class="btn btn--primary" @click="$emit('close')">Закрыть</button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'

export default {
  name: 'AnnouncementModal',
  components: { BaseModal },

  props: {
    show: {
      type: Boolean,
      default: false,
    },
    announcement: {
      type: Object,
      default: null,
    },
  },

  emits: ['close'],

  methods: {
    formatDate(dateStr) {
      if (!dateStr) return ''
      return new Date(dateStr).toLocaleDateString('ru-RU')
    },
  },
}
</script>

<style scoped>
.announcement-badge {
  display: inline-block;
  padding: 4px 12px;
  background-color: var(--color-danger);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  border-radius: var(--radius-sm);
  margin-bottom: 12px;
}

.announcement-title {
  margin: 0 0 12px;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text);
}

.announcement-description {
  margin: 0 0 12px;
  font-size: 14px;
  line-height: 1.6;
  color: var(--color-text);
}

.announcement-full-text {
  margin: 0 0 16px;
  font-size: 14px;
  line-height: 1.6;
  color: var(--color-text);
  padding-top: 12px;
  border-top: 1px solid var(--color-border);
}

.announcement-date {
  display: block;
  font-size: 13px;
  color: var(--color-text-muted);
}

.btn {
  padding: 10px 24px;
  border: none;
  border-radius: var(--radius-sm);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.btn--primary {
  background-color: var(--color-primary);
  color: #fff;
}

.btn--primary:hover {
  background-color: var(--color-primary-hover);
}
</style>
