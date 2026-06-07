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
        @count="vehicleCount = $event"
      />
      <PersonBlacklistTab
        v-show="activeTab === 'persons'"
        @count="personCount = $event"
      />
    </div>
  </section>
</template>

<script>
import FilterTabs from '@/components/ui/FilterTabs.vue';
import VehicleBlacklistTab from '@/components/admin/blacklist/VehicleBlacklistTab.vue';
import PersonBlacklistTab from '@/components/admin/blacklist/PersonBlacklistTab.vue';

/**
 * Страница "Чёрный список" (#443): две вкладки (Машины / Люди) со счётчиками
 * активных записей. Обе вкладки смонтированы (v-show), чтобы счётчики
 * подтягивались сразу. Создание/история/архив-действия - в следующих срезах.
 */
export default {
  name: 'BlacklistView',
  components: { FilterTabs, VehicleBlacklistTab, PersonBlacklistTab },
  data() {
    return {
      activeTab: 'vehicles',
      vehicleCount: 0,
      personCount: 0,
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
