import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';

// Эпик blank-import, срез E2E3: черновик не давится импортом (saveToLocalStorage не
// сериализуется на каждую строку, переполнение квоты не роняет форму), а подача
// большого списка не рвётся по глобальному таймауту apiRequest и не дублируется
// повторным кликом.

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

// Строка формы, готовая пройти submitValidation без обращения к справочникам:
// isExisting:true исключает её из "новых для привязки" - подача идёт прямиком
// в sendCompleteApplication, минуя модалку UniversalBindingModal.
function fillSubmittableState(w) {
    w.vm.attachments = [{ local_id: 'p1', attachment_type: 'people', display_name: 'Люди' }];
    w.vm.employeesByAttachment = {
        p1: [{ id: 1, lastName: 'Иванов', firstName: 'Иван', middleName: '', isExisting: true }],
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

describe('CreateApplication - черновик при массовом добавлении (E2)', () => {
    it('handleEmployeesAdded сохраняет черновик один раз на N сотрудников, а не на каждого', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'p1', attachment_type: 'people', display_name: 'Люди' }];
        w.vm.employeesByAttachment = { p1: [] };
        w.vm.selectedAttachment = w.vm.attachments[0];

        const saveSpy = vi.spyOn(w.vm, 'saveToLocalStorage');
        const bulk = Array.from({ length: 500 }, (_, i) => ({ lastName: `Сотрудник ${i}` }));
        w.vm.handleEmployeesAdded(bulk);

        expect(saveSpy).toHaveBeenCalledTimes(1);
        expect(w.vm.employeesByAttachment.p1).toHaveLength(500);
    });

    it('handleVehiclesAdded сохраняет черновик один раз на N машин, а не на каждую', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'c1', attachment_type: 'cars', display_name: 'Машины' }];
        w.vm.vehiclesByAttachment = { c1: [] };
        w.vm.selectedAttachment = w.vm.attachments[0];

        const saveSpy = vi.spyOn(w.vm, 'saveToLocalStorage');
        const bulk = Array.from({ length: 300 }, (_, i) => ({ plateNumber: `А${i}00АА777` }));
        w.vm.handleVehiclesAdded(bulk);

        expect(saveSpy).toHaveBeenCalledTimes(1);
        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(300);
    });

    it('переполнение квоты localStorage не роняет форму и показывает понятное уведомление', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'p1', attachment_type: 'people', display_name: 'Люди' }];

        const quotaError = new DOMException('quota exceeded', 'QuotaExceededError');
        const setItemSpy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
            throw quotaError;
        });

        expect(() => w.vm.saveToLocalStorage()).not.toThrow();

        expect(notifyMock).toHaveBeenCalledTimes(1);
        const call = notifyMock.mock.calls[0][0];
        expect(call.type).toBe('error');
        const text = `${call.prefix || ''}${call.bold || ''}${call.suffix || ''}`;
        expect(text).toMatch(/квот|хранилищ|места/i);

        // форма продолжает работать после сбоя автосохранения
        expect(w.vm.attachments).toHaveLength(1);

        setItemSpy.mockRestore();
    });

    it('обычная ошибка сохранения (не квота) не отправляет пользователю уведомление о квоте', async () => {
        const w = await mountApp();
        const setItemSpy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
            throw new Error('другая ошибка записи');
        });

        expect(() => w.vm.saveToLocalStorage()).not.toThrow();
        expect(notifyMock).not.toHaveBeenCalled();

        setItemSpy.mockRestore();
    });
});

describe('CreateApplication - подача большого списка не рвётся по таймауту (E3)', () => {
    it('submit-complete-application уходит со своим сигналом из createExtendedTimeoutSignal', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'p1', attachment_type: 'people', display_name: 'Люди' }];
        w.vm.employeesByAttachment = {
            p1: [{ id: 1, lastName: 'Иванов', isExisting: true }],
        };
        w.vm.attachmentDatesByAttachment = {
            p1: { isOneDay: true, singleDate: '02.07.2026', startTime: '09:00', endTime: '18:00', errors: {} },
        };
        w.vm.responsiblePerson = 'Иванов И.И.';
        w.vm.phoneNumber = '+7 999 123-45-67';

        await w.vm.sendCompleteApplication();

        expect(createExtendedTimeoutSignalMock).toHaveBeenCalledTimes(1);
        const [ms] = createExtendedTimeoutSignalMock.mock.calls[0];
        expect(ms).toBeGreaterThan(10000); // куда щедрее дефолтного таймаута apiRequest

        const submitCall = apiRequestMock.mock.calls.find(
            ([path]) => path === '/applications/submit-complete-application'
        );
        expect(submitCall).toBeTruthy();
        expect(submitCall[1].signal).toBe('FAKE_EXTENDED_SIGNAL');
    });

    it('повторное нажатие "Отправить заявку" во время подачи не шлёт второй запрос', async () => {
        const w = await mountApp();
        fillSubmittableState(w);

        let resolveSubmit;
        apiRequestMock.mockImplementation((path) => {
            if (path === '/applications/submit-complete-application') {
                return new Promise((resolve) => {
                    resolveSubmit = resolve;
                });
            }
            return Promise.resolve(okEmptyJson());
        });

        const first = w.vm.submitApplication();
        await flushPromises();
        expect(w.vm.isSubmitting).toBe(true);

        // Повторный клик, пока первая подача ещё летит - должен быть no-op.
        const second = w.vm.submitApplication();
        await flushPromises();

        resolveSubmit({
            ok: true,
            json: vi.fn().mockResolvedValue({ application_number: '№ 1' }),
        });
        await Promise.all([first, second]);
        await flushPromises();

        const submitCalls = apiRequestMock.mock.calls.filter(
            ([path]) => path === '/applications/submit-complete-application'
        );
        expect(submitCalls).toHaveLength(1);
        expect(w.vm.isSubmitting).toBe(false);
    });

    it('кнопка подачи блокируется и меняет текст, пока идёт отправка', async () => {
        const w = await mountApp();
        fillSubmittableState(w);

        let resolveSubmit;
        apiRequestMock.mockImplementation((path) => {
            if (path === '/applications/submit-complete-application') {
                return new Promise((resolve) => {
                    resolveSubmit = resolve;
                });
            }
            return Promise.resolve(okEmptyJson());
        });

        const submitPromise = w.vm.submitApplication();
        await flushPromises();
        await w.vm.$nextTick();

        const button = w.find('[data-testid="create-app-button-submit"]');
        expect(button.attributes('disabled')).toBeDefined();
        expect(button.text()).not.toBe('Отправить заявку');
        expect(w.find('[data-testid="create-app-submit-progress"]').exists()).toBe(true);

        resolveSubmit({
            ok: true,
            json: vi.fn().mockResolvedValue({ application_number: '№ 1' }),
        });
        await submitPromise;
        await flushPromises();
    });
});
