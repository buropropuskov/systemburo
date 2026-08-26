import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';

// Блокер код-ревью (эпик blank-import, срез E2E3): isSubmitting гасился в finally
// submitApplication() сразу после открытия UniversalBindingModal (showBindingModal=true),
// хотя confirmBinding()/skipBinding() ещё не позвали sendCompleteApplication. Кнопка
// "Отправить заявку" оживала на всё время, пока модалка открыта, и в первую очередь -
// пока после confirmBinding/skipBinding закрывшая модалку логика ждёт тяжёлый POST
// /applications/submit-complete-application. Повторный клик уходил вторым параллельным
// запросом без идемпотентности на бэке - дубль заявки. CreateApplicationDraftAndSubmitTimeout.spec.js
// этот путь не ловит - там подача идёт с isExisting:true, то есть прямиком в
// sendCompleteApplication, минуя модалку привязки.

const notifyMock = vi.fn();
vi.mock('@/stores/deletions', () => ({
    useDeletionsStore: vi.fn(() => ({ notify: notifyMock, enqueue: vi.fn() })),
}));

const apiRequestMock = vi.fn();
const createExtendedTimeoutSignalMock = vi.fn(() => 'FAKE_EXTENDED_SIGNAL');
vi.mock('@/api/client', () => ({
    apiRequest: (...args) => apiRequestMock(...args),
    createExtendedTimeoutSignal: (...args) => createExtendedTimeoutSignalMock(...args),
}));

vi.mock('@/stores/auth', () => ({
    useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));

function okEmptyJson() {
    return { ok: true, json: vi.fn().mockResolvedValue([]) };
}

beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    notifyMock.mockReset();
    apiRequestMock.mockReset();
    apiRequestMock.mockResolvedValue(okEmptyJson());
    createExtendedTimeoutSignalMock.mockClear();
});

async function mountApp() {
    const w = shallowMount(CreateApplication);
    await flushPromises();
    return w;
}

// Сотрудник не существующий и не "по факту" - collectNewDataForBinding() (по дефолту
// /unique-employees отдаёт пустой список) сочтёт его новым, и подача пойдёт через
// UniversalBindingModal, а не прямиком в sendCompleteApplication.
function fillStateWithNewEmployee(w) {
    w.vm.attachments = [{ local_id: 'p1', attachment_type: 'people', display_name: 'Люди' }];
    w.vm.employeesByAttachment = {
        p1: [{
            id: 1,
            lastName: 'Иванов',
            firstName: 'Иван',
            middleName: '',
            passportSeriesNumber: '1234 567890',
            position: 'Инженер',
            // Привязка к реестру идёт только для людей с отметкой согласия субъекта:
            // без неё запись реестра не создаётся, и этот путь просто не запустится.
            pdConsent: true,
            isExisting: false,
        }],
    };
    w.vm.attachmentDatesByAttachment = {
        p1: { isOneDay: true, singleDate: '02.07.2026', startTime: '09:00', endTime: '18:00', errors: {} },
    };
    // .create__form (и кнопка подачи внутри) рендерится только при выбранном вложении.
    w.vm.selectedAttachment = w.vm.attachments[0];
    w.vm.organization = 'ООО Ромашка';
    w.vm.responsiblePerson = 'Иванов И.И.';
    w.vm.phoneNumber = '+7 999 123-45-67';
    w.vm.consentGiven = true;
}

// Подменяет apiRequest так, что submit-complete-application зависает, пока тест
// сам не решит его отпустить - остальные запросы (в т.ч. привязка) отвечают сразу.
function hangSubmitCompleteApplication() {
    let resolveSubmit;
    apiRequestMock.mockImplementation((path) => {
        if (path === '/applications/submit-complete-application') {
            return new Promise((resolve) => { resolveSubmit = resolve; });
        }
        return Promise.resolve(okEmptyJson());
    });
    return {
        resolve: () => resolveSubmit({
            ok: true,
            json: vi.fn().mockResolvedValue({ application_number: '№ 1' }),
        }),
    };
}

// Подменяет apiRequest так, что сам POST привязки (/unique-employees) зависает, пока
// тест не отпустит - остальные запросы (в т.ч. GET-списки существующих и итоговый submit)
// отвечают сразу. confirmBinding() делает этот POST ДО closeBindingModal(), поэтому пока
// он висит, модалка формально ещё открыта - именно это окно и проверяем.
function hangUniqueEmployeesBindPost() {
    let resolveBind;
    apiRequestMock.mockImplementation((path, options = {}) => {
        if (path === '/unique-employees' && options.method === 'POST') {
            return new Promise((resolve) => { resolveBind = resolve; });
        }
        return Promise.resolve(okEmptyJson());
    });
    return { resolve: () => resolveBind(okEmptyJson()) };
}

function submitCompleteApplicationCalls() {
    return apiRequestMock.mock.calls.filter(
        ([path]) => path === '/applications/submit-complete-application'
    );
}

function uniqueEmployeesBindCalls() {
    return apiRequestMock.mock.calls.filter(
        ([path, options]) => path === '/unique-employees' && options?.method === 'POST'
    );
}

// Условие для confirmBinding, при котором привязка сотрудника реально запускается
// (совпадает с тем, что реальная модалка эмитит для нового не-"по факту" сотрудника).
const bindingDataWithEmployeeBinding = {
    vehicles: { bindToOrganization: false, bindToCompany: false, hasVehiclesForBinding: false },
    employees: { bindToOrganization: false, bindToCompany: false, hasEmployeesForBinding: true },
};

describe('CreateApplication - гард isSubmitting держит путь через модалку привязки', () => {
    it('новые сущности открывают модалку привязки вместо прямой подачи, isSubmitting остаётся true', async () => {
        const w = await mountApp();
        fillStateWithNewEmployee(w);

        await w.vm.submitApplication();

        expect(w.vm.showBindingModal).toBe(true);
        expect(w.vm.isSubmitting).toBe(true);
        expect(submitCompleteApplicationCalls()).toHaveLength(0);
    });

    it('confirmBinding: повторный клик по кнопке подачи во время висящего POST не шлёт второй запрос', async () => {
        const w = await mountApp();
        fillStateWithNewEmployee(w);
        const submit = hangSubmitCompleteApplication();

        await w.vm.submitApplication();
        expect(w.vm.showBindingModal).toBe(true);

        // Реальная модалка эмитит hasEmployeesForBinding=true, когда среди новых
        // сотрудников есть хотя бы один не "по факту" - привязка к оргструктуре
        // при этом не отмечена (bindTo* false).
        const confirmPromise = w.vm.confirmBinding({
            vehicles: { bindToOrganization: false, bindToCompany: false, hasVehiclesForBinding: false },
            employees: { bindToOrganization: false, bindToCompany: false, hasEmployeesForBinding: true },
        });
        await flushPromises();

        // closeBindingModal() отрабатывает синхронно до тяжёлого POST - модалка
        // уже закрыта, а isSubmitting ещё держит форму заблокированной.
        expect(w.vm.showBindingModal).toBe(false);
        expect(w.vm.isSubmitting).toBe(true);

        const button = w.find('[data-testid="create-app-button-submit"]');
        expect(button.attributes('disabled')).toBeDefined();
        await button.trigger('click');
        await flushPromises();

        submit.resolve();
        await confirmPromise;
        await flushPromises();

        expect(submitCompleteApplicationCalls()).toHaveLength(1);
        expect(w.vm.isSubmitting).toBe(false);
    });

    it('skipBinding: повторный клик во время висящего POST после пропуска привязки не шлёт второй запрос', async () => {
        const w = await mountApp();
        fillStateWithNewEmployee(w);
        const submit = hangSubmitCompleteApplication();

        await w.vm.submitApplication();
        expect(w.vm.showBindingModal).toBe(true);

        const skipPromise = w.vm.skipBinding();
        await flushPromises();

        expect(w.vm.showBindingModal).toBe(false);
        expect(w.vm.isSubmitting).toBe(true);

        const button = w.find('[data-testid="create-app-button-submit"]');
        expect(button.attributes('disabled')).toBeDefined();
        await button.trigger('click');
        await flushPromises();

        submit.resolve();
        await skipPromise;
        await flushPromises();

        expect(submitCompleteApplicationCalls()).toHaveLength(1);
        expect(w.vm.isSubmitting).toBe(false);
    });

    it('закрытие модалки крестиком/оверлеем без подтверждения снимает isSubmitting', async () => {
        const w = await mountApp();
        fillStateWithNewEmployee(w);

        await w.vm.submitApplication();
        expect(w.vm.showBindingModal).toBe(true);
        expect(w.vm.isSubmitting).toBe(true);

        w.vm.cancelBindingModal();

        expect(w.vm.showBindingModal).toBe(false);
        expect(w.vm.isSubmitting).toBe(false);
        expect(submitCompleteApplicationCalls()).toHaveLength(0);
    });

    it('cancelBindingModal посреди работающей привязки (mapWithConcurrency ещё летит) не отменяет фоновый confirmBinding и не открывает второй submit', async () => {
        const w = await mountApp();
        fillStateWithNewEmployee(w);
        const bind = hangUniqueEmployeesBindPost();

        await w.vm.submitApplication();
        expect(w.vm.showBindingModal).toBe(true);

        const confirmPromise = w.vm.confirmBinding(bindingDataWithEmployeeBinding);
        await flushPromises();

        // POST /unique-employees ещё висит - confirmBinding не дошёл до closeBindingModal(),
        // модалка формально ещё открыта.
        expect(w.vm.showBindingModal).toBe(true);
        expect(w.vm.isBindingActionInProgress).toBe(true);

        // Крестик/оверлей/Escape/свайп на уровне родителя - это cancelBindingModal().
        w.vm.cancelBindingModal();
        await flushPromises();

        // Отмена не прошла: фоновый confirmBinding продолжает работу невидимо для юзера.
        expect(w.vm.showBindingModal).toBe(true);
        expect(w.vm.isSubmitting).toBe(true);

        // Юзер, решив что отменил, жмёт "Отправить заявку" снова - должен остаться no-op.
        await w.vm.submitApplication();
        await flushPromises();

        bind.resolve();
        await confirmPromise;
        await flushPromises();

        expect(uniqueEmployeesBindCalls()).toHaveLength(1);
        expect(submitCompleteApplicationCalls()).toHaveLength(1);
        expect(w.vm.isSubmitting).toBe(false);
        expect(w.vm.isBindingActionInProgress).toBe(false);
    });

    it('повторный вызов confirmBinding (двойной клик по "Привязать и отправить") не даёт второй набор привязок и второй submit', async () => {
        const w = await mountApp();
        fillStateWithNewEmployee(w);
        const bind = hangUniqueEmployeesBindPost();

        await w.vm.submitApplication();
        expect(w.vm.showBindingModal).toBe(true);

        const first = w.vm.confirmBinding(bindingDataWithEmployeeBinding);
        await flushPromises();
        expect(w.vm.isBindingActionInProgress).toBe(true);

        // Второй emit того же confirm-binding, пока первый ещё летит.
        const second = w.vm.confirmBinding(bindingDataWithEmployeeBinding);
        await flushPromises();

        bind.resolve();
        await Promise.all([first, second]);
        await flushPromises();

        expect(uniqueEmployeesBindCalls()).toHaveLength(1);
        expect(submitCompleteApplicationCalls()).toHaveLength(1);
        expect(w.vm.isSubmitting).toBe(false);
        expect(w.vm.isBindingActionInProgress).toBe(false);
    });
});
