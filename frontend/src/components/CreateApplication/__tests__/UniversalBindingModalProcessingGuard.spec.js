import { describe, it, expect, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import UniversalBindingModal from '../UniversalBindingModal.vue';

// Блокер код-ревью (эпик blank-import, срез E2E3, второй заход): confirmBinding() держит
// модалку открытой, пока mapWithConcurrency создаёт новые unique-cars/unique-employees
// (на массовом импорте - сотни запросов, окно широкое), но сама модалка не блокировала
// ни свои кнопки на повторный клик, ни крестик/оверлей/Escape/свайп - родитель получал
// "отмену" посреди фонового confirmBinding, а второй клик по "Привязать и отправить"
// уходил вторым набором привязок и вторым submit. Проп processing - единственный
// источник правды, которым родитель (isBindingActionInProgress) блокирует оба пути.

// Содержимое модалки телепортируется в body - искать через document.querySelector,
// не через wrapper.find (тот видит только внутри wrapper.element, эталон паттерна -
// SchedulePlaceWarningPanel.spec.js).
function mountModal(props = {}) {
    return mount(UniversalBindingModal, {
        props: {
            show: true,
            newVehiclesToBind: [],
            newEmployeesToBind: [],
            organization: '',
            company: '',
            hasOrganization: false,
            hasCompany: false,
            processing: false,
            ...props,
        },
        attachTo: document.body,
    });
}

afterEach(() => {
    document.body.innerHTML = '';
});

describe('UniversalBindingModal - гард processing блокирует повторный клик и закрытие', () => {
    it('кнопки и крестик задизейблены, пока идёт обработка', () => {
        mountModal({ processing: true });

        expect(document.querySelector('.modal-close').disabled).toBe(true);
        expect(document.querySelector('.skip-btn').disabled).toBe(true);
        expect(document.querySelector('.confirm-btn').disabled).toBe(true);
    });

    it('кнопки активны, когда обработка не идёт', () => {
        mountModal({ processing: false });

        expect(document.querySelector('.modal-close').disabled).toBe(false);
        expect(document.querySelector('.skip-btn').disabled).toBe(false);
        expect(document.querySelector('.confirm-btn').disabled).toBe(false);
    });

    it('повторный клик по "Привязать и отправить" во время processing не эмитит confirm-binding второй раз', async () => {
        const w = mountModal();

        document.querySelector('.confirm-btn').click();
        await w.vm.$nextTick();
        expect(w.emitted('confirm-binding')).toHaveLength(1);

        // Родитель выставляет processing=true сразу по первому клику (тот же тик,
        // что и клик, в реальном флоу) - имитируем это следующим кликом уже "во время".
        await w.setProps({ processing: true });
        document.querySelector('.confirm-btn').click();
        await w.vm.$nextTick();

        expect(w.emitted('confirm-binding')).toHaveLength(1);
    });

    it('повторный клик по "Отправить без привязки" во время processing не эмитит skip-binding второй раз', async () => {
        const w = mountModal();

        document.querySelector('.skip-btn').click();
        await w.vm.$nextTick();
        expect(w.emitted('skip-binding')).toHaveLength(1);

        await w.setProps({ processing: true });
        document.querySelector('.skip-btn').click();
        await w.vm.$nextTick();

        expect(w.emitted('skip-binding')).toHaveLength(1);
    });

    it('крестик и оверлей не закрывают модалку, пока идёт обработка', async () => {
        const w = mountModal({ processing: true });

        document.querySelector('.modal-close').click();
        document.querySelector('.modal-overlay').click();
        await w.vm.$nextTick();

        expect(w.emitted('close')).toBeUndefined();
    });

    it('Escape не закрывает модалку, пока идёт обработка', async () => {
        const w = mountModal({ processing: true });

        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
        await w.vm.$nextTick();

        expect(w.emitted('close')).toBeUndefined();
    });

    it('крестик закрывает модалку штатно, когда обработка не идёт', async () => {
        const w = mountModal({ processing: false });

        document.querySelector('.modal-close').click();
        await w.vm.$nextTick();

        expect(w.emitted('close')).toHaveLength(1);
    });

    it('на кликнутой кнопке во время обработки текст меняется на "Отправляем...", вторая кнопка не трогается', async () => {
        const w = mountModal();

        document.querySelector('.confirm-btn').click();
        await w.setProps({ processing: true });
        await w.vm.$nextTick();

        expect(document.querySelector('.confirm-btn').textContent).toContain('Отправляем...');
        expect(document.querySelector('.skip-btn').textContent.trim()).toBe('Отправить без привязки');
    });
});
