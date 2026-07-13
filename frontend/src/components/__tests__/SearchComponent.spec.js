import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import SearchComponent from '../SearchComponent.vue';

// Регресс на #1157: SearchComponent раньше держал собственную (более бедную) таблицу
// транслита без раскладки клавиатуры. Теперь варианты строятся общим
// buildSearchVariants (utils/searchVariants.js) - тот же канон, что и в
// справочниках (CitizenshipManagement/UserControl/...).

describe('SearchComponent', () => {
    it('эмитит update:modelValue с сырым текстом ввода', async () => {
        const wrapper = mount(SearchComponent, { props: { title: 'Поиск' } });
        await wrapper.find('input').setValue('иванов');

        expect(wrapper.emitted('update:modelValue')[0]).toEqual(['иванов']);
    });

    it('эмитит search с вариантами из общего buildSearchVariants (раскладка+транслит)', async () => {
        const wrapper = mount(SearchComponent, { props: { title: 'Поиск' } });
        // "bdfyjd" на физических клавишах = "иванов".
        await wrapper.find('input').setValue('bdfyjd');

        const [variants] = wrapper.emitted('search')[0];
        expect(variants).toContain('иванов');
    });

    it('пустой ввод даёт пустой массив вариантов (нет собственного [""]-сентинела)', async () => {
        const wrapper = mount(SearchComponent, { props: { title: 'Поиск' } });
        await wrapper.find('input').setValue('');

        expect(wrapper.emitted('search')[0]).toEqual([[]]);
    });
});
