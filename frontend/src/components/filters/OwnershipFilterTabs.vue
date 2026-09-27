<template>
  <FilterTabs
    :tabs="tabs"
    :testid-prefix="testidPrefix"
    :model-value="modelValue"
    @update:model-value="$emit('update:modelValue', $event)"
  />
</template>

<script>
import FilterTabs from '@/components/ui/FilterTabs.vue';
import { usePermissionsStore } from '@/stores/permissions';

/**
 * Область реестра: чьи записи показывать - организации, компании, свои или все.
 *
 * Ряд одинаков у «Моих сотрудников» и «Моих автомобилей» и до этого лежал в каждой
 * вью дважды: инлайном в шапке и второй копией внутри мобильного листа фильтров.
 * Четыре кнопки на два экрана в двух копиях - это сотня строк разметки, которую
 * приходилось править синхронно; заодно оба файла давно сверх порога гейта размера.
 *
 * Права на вкладки компонент считает сам: они зависят только от разделов реестра и
 * одинаковы для обеих вью, держать их вычислимыми свойствами в каждой не за чем.
 *
 * Сворачивание ряда в выпадающий список, когда он не помещается, приезжает из
 * `FilterTabs` - правило владельца, тег #55678.
 */
const LABELS = {
  employees: {
    organization: 'Сотрудники организации',
    company: 'Сотрудники компании',
    user: 'Мои сотрудники',
    all_system: 'Все сотрудники системы',
  },
  cars: {
    organization: 'Машины организации',
    company: 'Машины компании',
    user: 'Мои машины',
    all_system: 'Все машины системы',
  },
};

const TITLES = {
  employees: {
    organization: 'Сотрудники, которых привязывали пользователи вашей организации',
    company: 'Сотрудники, которых привязывали пользователи вашей компании',
    user: 'Только те сотрудники, которых привязывали лично вы',
    all_system: 'Все сотрудники, когда-либо зарегистрированные в системе',
  },
  cars: {
    organization: 'Автомобили, которых привязывали пользователи вашей организации',
    company: 'Автомобили, которых привязывали пользователи вашей компании',
    user: 'Только те автомобили, которых привязывали лично вы',
    all_system: 'Все автомобили, когда-либо зарегистрированные в системе',
  },
};

export default {
  name: 'OwnershipFilterTabs',
  components: { FilterTabs },
  props: {
    /** Реестр: `employees` или `cars`. Определяет подписи вкладок. */
    kind: {
      type: String,
      required: true,
      validator: (value) => value === 'employees' || value === 'cars',
    },
    /** Ответ `ownership-info`: есть ли у пользователя организация и компания. */
    ownership: {
      type: Object,
      default: null,
    },
    modelValue: {
      type: String,
      required: true,
    },
    /** Приставка `data-testid`: в шапке одна, в листе фильтров другая. */
    testidPrefix: {
      type: String,
      default: 'filter-tab-',
    },
  },
  emits: ['update:modelValue'],
  computed: {
    tabs() {
      const can = (key) => usePermissionsStore().hasPermission(`section.registry.${key}`);
      const label = LABELS[this.kind];
      const title = TITLES[this.kind];
      return [
        {
          key: 'organization',
          label: label.organization,
          title: title.organization,
          visible: Boolean(this.ownership?.has_organization) && can('organization'),
        },
        {
          key: 'company',
          label: label.company,
          title: title.company,
          visible: Boolean(this.ownership?.has_company) && can('company'),
        },
        { key: 'user', label: label.user, title: title.user },
        {
          key: 'all_system',
          // Ключ с подчёркиванием, а testid через дефис: так он назывался в
          // разметке до выноса, и на него смотрят замки обеих вью.
          testid: 'all-system',
          label: label.all_system,
          title: title.all_system,
          visible: can('all_system'),
        },
      ];
    },
  },
};
</script>
