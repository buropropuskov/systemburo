<template>
  <section class="blacklist-page">
    <header class="page-header">
      <h2 class="page-title">
        Чёрный список
      </h2>
    </header>

    <FilterTabs
      v-model="activeTab"
      :tabs="tabs"
    />

    <div class="blacklist-card">
      <VehicleBlacklistTab
        v-show="activeTab === 'vehicles'"
        :current-user-name="currentUserName"
        @count="vehicleCount = $event"
      />
      <PersonBlacklistTab
        v-show="activeTab === 'persons'"
        :current-user-name="currentUserName"
        @count="personCount = $event"
      />
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
      activeTab: 'vehicles',
      vehicleCount: 0,
      personCount: 0,
      currentUserName: '',
    };
  },
  computed: {
    tabs() {
      return [
        { key: 'vehicles', label: `Машины (${this.vehicleCount})` },
        { key: 'persons', label: `Люди (${this.personCount})` },
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
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.page-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.blacklist-card {
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: var(--radius-lg, 20px);
  overflow: hidden;
}
</style>
