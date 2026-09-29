import { describe, it, expect, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import OwnershipFilterTabs from '../OwnershipFilterTabs.vue';
import { usePermissionsStore } from '@/stores/permissions';

/**
 * Область реестра: чьи записи показывать. Ряд вынесен из «Моих сотрудников» и
 * «Моих автомобилей», где лежал дважды - в шапке и в мобильном листе фильтров.
 *
 * Видимость вкладок держится на двух независимых условиях: право на раздел
 * реестра и наличие у пользователя организации/компании. Разъехаться они могут
 * молча - вкладка просто перестанет показываться тому, кому положена.
 */
const ALL = { has_organization: true, has_company: true };

const seed = (allow) => {
  const perms = usePermissionsStore();
  perms.mode = 'normal';
  perms.effective = Object.fromEntries(allow.map((key) => [key, { value: 'allow', source: 'role' }]));
};

const mountTabs = (props = {}) => mount(OwnershipFilterTabs, {
  props: { kind: 'employees', ownership: ALL, modelValue: 'user', ...props },
});

const keys = (wrapper) => wrapper.findAll('.filter-tab').map((b) => b.attributes('data-testid'));

describe('OwnershipFilterTabs', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('без прав на разделы остаётся только «Мои»', () => {
    seed([]);
    expect(keys(mountTabs())).toEqual(['filter-tab-user']);
  });

  it('право на раздел открывает свою вкладку', () => {
    seed(['section.registry.organization', 'section.registry.all_system']);
    expect(keys(mountTabs())).toEqual([
      'filter-tab-organization', 'filter-tab-user', 'filter-tab-all-system',
    ]);
  });

  it('без организации вкладка организации не показывается даже с правом', () => {
    seed(['section.registry.organization', 'section.registry.company']);
    const wrapper = mountTabs({ ownership: { has_organization: false, has_company: true } });
    expect(keys(wrapper)).toEqual(['filter-tab-company', 'filter-tab-user']);
  });

  it('подписи зависят от реестра', () => {
    seed([]);
    expect(mountTabs({ kind: 'employees' }).text()).toContain('Мои сотрудники');
    expect(mountTabs({ kind: 'cars' }).text()).toContain('Мои машины');
  });

  it('приставка testid задаётся снаружи - на неё смотрят замки листа фильтров', () => {
    seed(['section.registry.organization']);
    const wrapper = mountTabs({ testidPrefix: 'employees-scope-' });
    expect(keys(wrapper)).toEqual(['employees-scope-organization', 'employees-scope-user']);
  });

  it('выбор вкладки уходит наружу ключом области', async () => {
    seed(['section.registry.organization']);
    const wrapper = mountTabs();
    await wrapper.find('[data-testid="filter-tab-organization"]').trigger('click');
    expect(wrapper.emitted('update:modelValue')).toEqual([['organization']]);
  });

  it('в вертикальной укладке ряд не сворачивается в список', async () => {
    seed(['section.registry.organization', 'section.registry.company', 'section.registry.all_system']);
    const wrapper = mountTabs({ stacked: true });
    const tabs = wrapper.findComponent({ name: 'FilterTabs' });
    const row = tabs.vm.$refs.row;
    Object.defineProperty(row, 'clientWidth', { value: 100, configurable: true });
    [...row.children].forEach((el) => {
      Object.defineProperty(el, 'offsetWidth', { value: 300, configurable: true });
    });
    tabs.vm.measure();
    await wrapper.vm.$nextTick();

    expect(tabs.vm.collapsed).toBe(false);
    expect(wrapper.find('[data-testid="filter-tabs-select"]').exists()).toBe(false);
  });
});
