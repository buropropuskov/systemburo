<template>
  <section class="blacklist-page">
    <div
      class="blacklist-card"
      data-testid="ob-admin-blacklist"
    >
      <VehicleBlacklistTab
        v-show="activeTab === 'vehicles'"
        :current-user-name="currentUserName"
        @count="vehicleCount = $event"
      >
        <template #header-left>
          <div class="bl-head-left">
            <h2 class="bl-page-title">
              Чёрный список
            </h2>
            <FilterTabs
              v-model="activeTab"
              :tabs="tabs"
            />
          </div>
        </template>
      </VehicleBlacklistTab>
      <PersonBlacklistTab
        v-show="activeTab === 'persons'"
        :current-user-name="currentUserName"
        @count="personCount = $event"
      >
        <template #header-left>
          <div class="bl-head-left">
            <h2 class="bl-page-title">
              Чёрный список
            </h2>
            <FilterTabs
              v-model="activeTab"
              :tabs="tabs"
            />
          </div>
        </template>
      </PersonBlacklistTab>
    </div>
  </section>
</template>

<script>
import FilterTabs from '@/components/ui/FilterTabs.vue';
import VehicleBlacklistTab from '@/components/admin/blacklist/VehicleBlacklistTab.vue';
import PersonBlacklistTab from '@/components/admin/blacklist/PersonBlacklistTab.vue';
import { apiRequest } from '@/api/client';

/**
 * Страница "Чёрный список" (#443): две вкладки (Машины / Люди) со счётчиками
 * активных записей. Обе вкладки смонтированы (v-show), чтобы счётчики
 * подтягивались сразу. Имя текущего пользователя грузим один раз тут и
 * прокидываем в обе вкладки - для подписи в Excel-экспорте истории.
 */
export default {
  name: 'BlacklistView',
  components: { FilterTabs, VehicleBlacklistTab, PersonBlacklistTab },
  data() {
    return {
      // Из адреса: переход из сквозного поиска знает, в какой вкладке лежит найденное.
      activeTab: this.$route?.query?.tab === 'persons' ? 'persons' : 'vehicles',
      vehicleCount: 0,
      personCount: 0,
      currentUserName: '',
    };
  },
  computed: {
    tabs() {
      return [
        { key: 'vehicles', label: 'Машины', count: this.vehicleCount },
        { key: 'persons', label: 'Люди', count: this.personCount },
      ];
    },
  },
  mounted() {
    this.fetchCurrentUser();
  },
  methods: {
    async fetchCurrentUser() {
      try {
        const res = await apiRequest('/users/me');
        const data = await res.json();
        const parts = [data.last_name, data.first_name, data.middle_name].filter(Boolean);
        this.currentUserName = parts.join(' ') || data.username || '';
      } catch {
        this.currentUserName = '';
      }
    },
  },
};
</script>

<style scoped>
.blacklist-page {
  padding: 16px;
}

.bl-head-left {
  display: flex;
  align-items: center;
  gap: 16px;
  min-width: 0;
}

/* Табы "Машины"/"Люди" держим в одну строку (таргетно, без правки общего FilterTabs). */
.bl-head-left :deep(.filter-tabs) {
  flex-wrap: nowrap;
}

.bl-page-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  white-space: nowrap;
}

.blacklist-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg, 20px);
  overflow: hidden;
}

@media (max-width: 767.98px) {
  /* Заголовок + табы в один флекс-ряд без wrap (nowrap выше) не помещаются на узких
     экранах (iPhone SE ~320px) - позволяем блоку в целом переноситься на вторую строку,
     сами табы "Машины"/"Люди" остаются рядом друг с другом. */
  .bl-head-left {
    flex-wrap: wrap;
    row-gap: 6px;
  }

  /* Пилюли FilterTabs высотой 30px ниже тач-таргета 44px (WCAG) - точечно поднимаем
     зону нажатия только тут, не трогая общий FilterTabs.vue (другие потребители не
     должны получить эту правку). */
  .bl-head-left :deep(.filter-tab) {
    min-height: 44px;
  }
}
</style>
