<template>
  <section class="notif-settings">
    <header class="notif-settings__header">
      <h1 class="notif-settings__title">
        Настройка уведомлений
      </h1>
      <button
        class="lk-button lk-button--primary"
        data-testid="notif-settings-save"
        :disabled="!isDirty || saving"
        @click="save"
      >
        {{ saving ? 'Сохранение...' : 'Сохранить' }}
      </button>
    </header>

    <WebPushSettings />

    <p
      v-if="loading"
      class="notif-settings__state"
    >
      Загрузка настроек...
    </p>

    <div
      v-else-if="error"
      class="notif-settings__state notif-settings__state--error"
    >
      <p>{{ error }}</p>
      <button
        class="lk-button lk-button--ghost"
        @click="load"
      >
        Повторить
      </button>
    </div>

    <p
      v-else-if="groups.length === 0"
      class="notif-settings__state"
    >
      Типов уведомлений пока нет.
    </p>

    <div
      v-else
      class="notif-settings__categories"
      data-testid="ob-notif-categories"
    >
      <section
        v-for="(group, index) in groups"
        :key="group.category"
        class="lk-card notif-category"
        :data-testid="index === 0 ? 'ob-notif-category' : null"
      >
        <header class="notif-category__header">
          <h2 class="notif-category__title">
            {{ group.label }}
          </h2>
          <ToggleSwitch
            :model-value="categoryEnabled(group)"
            :disabled="categoryMandatory(group)"
            :data-testid="`category-toggle-${group.category}`"
            @update:model-value="setCategory(group, $event)"
          />
        </header>
        <p
          v-if="categoryMandatory(group)"
          class="notif-category__hint"
        >
          Эти уведомления нельзя отключить - о них вы узнаете в любом случае.
        </p>

        <ul class="notif-category__list">
          <li
            v-for="item in group.items"
            :key="item.type_code"
            class="notif-item"
          >
            <div class="notif-item__text">
              <p class="notif-item__label">
                {{ item.label }}
              </p>
              <p class="notif-item__description">
                {{ item.description }}
              </p>
              <p
                v-if="item.mandatory"
                class="notif-item__hint"
              >
                Нельзя отключить
              </p>
            </div>
            <ToggleSwitch
              class="notif-item__toggle"
              :model-value="item.enabled"
              :disabled="item.mandatory"
              :data-testid="`item-toggle-${item.type_code}`"
              @update:model-value="setItem(item, $event)"
            />
          </li>
        </ul>
      </section>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue';
import WebPushSettings from '@/components/NotificationSettings/WebPushSettings.vue';
import { getNotificationPreferences, updateNotificationPreferences } from '@/api/notificationPreferences';
import { useDeletionsStore } from '@/stores/deletions';

// Русские названия категорий - каталог (#1748) отдаёт только код, подписи для
// экрана целиком на фронте.
const CATEGORY_LABELS = {
  application: 'Заявки',
  security: 'Учётная запись и безопасность',
  passage: 'Проходы и сроки',
  content: 'Новости и обращения',
  system: 'Система',
};
const CATEGORY_ORDER = ['application', 'security', 'passage', 'content', 'system'];

const items = ref([]);
// Снимок серверного состояния на момент загрузки/последнего успешного
// сохранения - по нему считаем dirty-набор и диф для PUT.
const original = ref(new Map());
const loading = ref(true);
const saving = ref(false);
const error = ref(null);

// Бэкенд отдаёт каталог секциями: [{category, items: [...]}]. Экран группирует
// сам (порядок категорий свой, подписи русские), поэтому секции разворачиваются
// в плоский список. Плоский ответ тоже принимается - на случай, если форма
// когда-нибудь упростится, экран от этого не сломается.
function flattenPreferences(payload) {
  if (!Array.isArray(payload)) return [];
  return payload.flatMap((entry) => {
    if (entry && Array.isArray(entry.items)) {
      return entry.items.map((item) => ({ category: entry.category, ...item }));
    }
    return entry ? [entry] : [];
  });
}

const groups = computed(() => {
  const byCategory = new Map();
  for (const item of items.value) {
    if (!byCategory.has(item.category)) byCategory.set(item.category, []);
    byCategory.get(item.category).push(item);
  }
  return CATEGORY_ORDER
    .filter((cat) => byCategory.has(cat))
    .map((cat) => ({ category: cat, label: CATEGORY_LABELS[cat] || cat, items: byCategory.get(cat) }));
});

// Обязательные типы исключены из dirty-проверки и из диффа сохранения
// намеренно дважды (здесь и в save()) - тумблер и так задизейблен, но диф
// не должен зависеть от того, что нарисовано на экране.
const isDirty = computed(() => items.value.some(
  (item) => !item.mandatory && item.enabled !== original.value.get(item.type_code),
));

function categoryMandatory(group) {
  return group.items.every((item) => item.mandatory);
}

function categoryEnabled(group) {
  return group.items.every((item) => item.enabled);
}

function setCategory(group, value) {
  for (const item of group.items) {
    if (item.mandatory) continue;
    item.enabled = value;
  }
}

function setItem(item, value) {
  if (item.mandatory) return;
  item.enabled = value;
}

async function load() {
  loading.value = true;
  error.value = null;
  try {
    const data = await getNotificationPreferences();
    items.value = flattenPreferences(data).map((item) => ({
      ...item,
      enabled: item.enabled ?? item.default_enabled,
    }));
    original.value = new Map(items.value.map((item) => [item.type_code, item.enabled]));
  } catch (e) {
    error.value = e?.message || 'Не удалось загрузить настройки уведомлений';
  } finally {
    loading.value = false;
  }
}

async function save() {
  const changed = items.value.filter(
    (item) => !item.mandatory && item.enabled !== original.value.get(item.type_code),
  );
  if (changed.length === 0) return;

  saving.value = true;
  try {
    await updateNotificationPreferences(
      changed.map((item) => ({ type_code: item.type_code, enabled: item.enabled })),
    );
    for (const item of changed) {
      original.value.set(item.type_code, item.enabled);
    }
    useDeletionsStore().notify({ bold: 'Настройки уведомлений', suffix: ' сохранены' });
  } catch (e) {
    useDeletionsStore().notify({
      prefix: 'Не удалось сохранить настройки: ',
      bold: e?.message || 'ошибка сети',
      type: 'error',
    });
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<style scoped>
.notif-settings {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 24px;
  font-family: 'Montserrat', sans-serif;
}

.notif-settings__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.notif-settings__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: var(--color-text);
}

.notif-settings__state {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
  padding: 24px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface);
  color: var(--text-muted);
  font-size: 14px;
}

.notif-settings__state--error {
  color: var(--danger-text);
}

.notif-settings__categories {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.notif-category {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.notif-category__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.notif-category__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text);
}

.notif-category__hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

.notif-category__list {
  list-style: none;
  margin: 4px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
}

.notif-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
  border-top: 1px solid var(--surface-2);
}

.notif-category__list li.notif-item:first-child {
  border-top: none;
}

.notif-item__text {
  flex: 1;
  min-width: 0;
}

.notif-item__label {
  margin: 0;
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text);
}

.notif-item__description {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
}

.notif-item__hint {
  margin: 4px 0 0;
  font-size: 11px;
  color: var(--text-muted);
  font-style: italic;
}

.notif-item__toggle {
  flex-shrink: 0;
  margin-top: 2px;
}

/* Мобилка: тумблер не наезжает на текст описания - текст уже flex:1/min-width:0,
   здесь только уменьшаем внешние отступы карточек под узкий экран. */
@media (max-width: 480px) {
  .notif-settings {
    padding: 15px;
  }

  .notif-item {
    gap: 10px;
  }
}
</style>
