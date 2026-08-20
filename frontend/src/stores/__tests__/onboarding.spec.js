import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useOnboardingStore } from '../onboarding';
import { useAuthStore } from '../auth';
import { usePermissionsStore } from '../permissions';
import { usePDConsentStore } from '../pdConsent';
import { onboardingSteps, ONBOARDING_VERSION } from '@/components/onboarding/onboardingSteps';
import {
  securityOnboardingSteps,
  SECURITY_ONBOARDING_VERSION,
} from '@/components/onboarding/securityOnboardingSteps';
import { getOnboardingStatus, markOnboardingComplete, getSecurityFactRoute } from '@/api/onboarding';
import { getMyApprovalRole } from '@/api/approvers';

vi.mock('@/api/onboarding', () => ({
  getOnboardingStatus: vi.fn(),
  markOnboardingComplete: vi.fn(),
  getSecurityFactRoute: vi.fn(),
}));

vi.mock('@/api/approvers', () => ({
  getMyApprovalRole: vi.fn(),
}));

function createMockJWT(payload, expiresInSeconds = 3600) {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const body = btoa(JSON.stringify({
    ...payload,
    exp: Math.floor(Date.now() / 1000) + expiresInSeconds,
  }));
  return `${header}.${body}.fake-signature`;
}

/** Выдать перечисленные права текущему пользователю (режим normal). */
function grant(...keys) {
  const permissions = usePermissionsStore();
  permissions.mode = 'normal';
  permissions.effective = Object.fromEntries(keys.map((k) => [k, { value: 'allow', source: 'base' }]));
}

// Гейтящие шаги и права берём из самой конфигурации: числа в ассертах («минус два
// шага») протухают с каждым новым permission-шагом и краснеют не по делу.
const USER_REQUIRES_STEPS = onboardingSteps.filter((s) => s.requires);
const USER_TOUR_RIGHTS = [...new Set(USER_REQUIRES_STEPS.map((s) => s.requires))];

describe('onboarding store', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getOnboardingStatus.mockReset();
    markOnboardingComplete.mockReset();
    markOnboardingComplete.mockResolvedValue({ message: 'ok' });
    getSecurityFactRoute.mockReset();
    getSecurityFactRoute.mockResolvedValue(null);
    getMyApprovalRole.mockReset();
    getMyApprovalRole.mockResolvedValue({ is_approver: false, is_reviewer: false });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('start / stop', () => {
    it('start() активирует тур, ставит его ключ и сбрасывает индекс на 0', () => {
      const store = useOnboardingStore();
      store.setIndex(3);
      expect(store.start({ tour: 'user' })).toBe(true);

      expect(store.isActive).toBe(true);
      expect(store.activeTourKey).toBe('user');
      expect(store.currentIndex).toBe(0);
    });

    it('start() идемпотентен при уже активном туре', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      store.setIndex(2);
      expect(store.start({ tour: 'guard' })).toBe(false);

      expect(store.isActive).toBe(true);
      expect(store.activeTourKey).toBe('user');
      expect(store.currentIndex).toBe(2);
    });

    it('start() незнакомого тура не активирует ничего', () => {
      const store = useOnboardingStore();
      expect(store.start({ tour: 'nope' })).toBe(false);
      expect(store.isActive).toBe(false);
      expect(store.activeTourKey).toBe(null);
    });

    // Все пять туров реестра написаны, поэтому «ненаписанный» проверяем ключом,
    // которого в реестре нет: механика та же (нечего показывать - не стартуем),
    // а тест не умрёт от наполнения очередной заготовки.
    it('start() тура, которого нет в реестре, не активирует ничего', () => {
      const store = useOnboardingStore();
      expect(store.start({ tour: 'no-such-tour' })).toBe(false);
      expect(store.isActive).toBe(false);
    });

    it('start({ manual: true }) выставляет isManual', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'user', manual: true });
      expect(store.isManual).toBe(true);
    });

    it('start() без manual оставляет isManual false', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      expect(store.isManual).toBe(false);
    });

    it('stop() деактивирует тур', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      store.stop();
      expect(store.isActive).toBe(false);
    });
  });

  describe('setIndex / reset', () => {
    it('setIndex меняет currentIndex', () => {
      grant('header.report_problem', 'header.create_application');
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      store.setIndex(2);
      expect(store.currentIndex).toBe(2);
      expect(store.currentStep).toBe(onboardingSteps[2]);
    });

    it('reset сбрасывает активность, индекс, ключ тура и per-user статус', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      store.setIndex(2);
      store.markCompleted();
      store.reset();

      expect(store.isActive).toBe(false);
      expect(store.currentIndex).toBe(0);
      expect(store.activeTourKey).toBe(null);
      expect(store.completedByTour).toEqual({});
      expect(store.statusLoaded).toBe(false);
    });

    it('reset чистит кэш роли согласования - следующий юзер тянет свою', async () => {
      getMyApprovalRole.mockResolvedValue({ is_approver: true, is_reviewer: false });
      const store = useOnboardingStore();
      await store.ensureApprovalRole();
      expect(store.approvalRole.isApprover).toBe(true);

      store.reset();
      expect(store.approvalRole).toEqual({ isApprover: false, isReviewer: false });
      expect(store.approvalRoleLoaded).toBe(false);

      await store.ensureApprovalRole();
      expect(getMyApprovalRole).toHaveBeenCalledTimes(2);
    });
  });

  describe('steps активного тура', () => {
    it('без активного тура набор шагов пуст', () => {
      const store = useOnboardingStore();
      expect(store.steps).toEqual([]);
      expect(store.totalSteps).toBe(0);
      expect(store.currentStep).toBe(null);
    });

    it('тур заявителя отдаёт свои шаги (при наличии прав на permission-шаги)', () => {
      grant(...USER_TOUR_RIGHTS);
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      expect(store.steps).toEqual(onboardingSteps);
      expect(store.totalSteps).toBe(onboardingSteps.length);
    });

    it('тур охраны отдаёт свои шаги плюс финал празднования', () => {
      grant('header.report_problem');
      const store = useOnboardingStore();
      store.start({ tour: 'guard' });
      expect(store.steps.slice(0, securityOnboardingSteps.length)).toEqual(securityOnboardingSteps);
      expect(store.steps[store.steps.length - 1].id).toBe('sec-finish');
      expect(store.currentStep).toBe(securityOnboardingSteps[0]);
    });

    it('набор шагов не зависит от типа пользователя - только от выбранного тура', () => {
      grant(...USER_TOUR_RIGHTS);
      const auth = useAuthStore();
      auth.userTypeCode = 'security';
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      expect(store.steps).toEqual(onboardingSteps);
    });
  });

  describe('requires - шаг без права выброшен из набора', () => {
    it('без header.report_problem шага «Сообщить о проблеме» в туре нет', () => {
      grant(...USER_TOUR_RIGHTS.filter((k) => k !== 'header.report_problem'));
      const store = useOnboardingStore();
      store.start({ tour: 'user' });

      const dropped = USER_REQUIRES_STEPS.filter((s) => s.requires === 'header.report_problem').length;
      expect(store.steps.some((s) => s.id === 'header-feedback')).toBe(false);
      expect(store.steps.some((s) => s.id === 'header-submit')).toBe(true);
      expect(store.totalSteps).toBe(onboardingSteps.length - dropped);
    });

    it('без прав выброшены все permission-шаги', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'user' });

      expect(USER_REQUIRES_STEPS.length).toBeGreaterThan(0);
      expect(store.steps.some((s) => s.requires)).toBe(false);
      expect(store.totalSteps).toBe(onboardingSteps.length - USER_REQUIRES_STEPS.length);
    });

    it('выброшенный шаг не сдвигает индексацию оставшихся (нет дырки в счётчике)', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      // Шаги идут подряд без пропусков: набор без прав - это ровно конфигурация с
      // вырезанными requires-шагами, в том же порядке.
      expect(store.steps.map((s) => s.id))
        .toEqual(onboardingSteps.filter((s) => !s.requires).map((s) => s.id));
      expect(store.steps.map((s) => s.id)).not.toContain('header-feedback');
      expect(store.steps.map((s) => s.id)).not.toContain('header-submit');
    });

    // Счётчик поповера считает по той же store.steps (useOnboarding.buildProgressBlock:
    // steps без optional), поэтому выброшенный шаг не оставляет дырки в нумерации.
    it('выброшенный шаг не считается в «шаг N из M»', () => {
      const withRights = (() => {
        grant(...USER_TOUR_RIGHTS);
        const s = useOnboardingStore();
        s.start({ tour: 'user' });
        return s.steps.filter((x) => !x.optional).length;
      })();

      setActivePinia(createPinia());
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      const withoutRights = store.steps.filter((s) => !s.optional).length;

      // optional-шаги в счётчик не входят изначально, поэтому разницу дают только
      // обязательные из числа гейтящихся.
      const countedDrop = USER_REQUIRES_STEPS.filter((s) => !s.optional).length;
      expect(withoutRights).toBe(withRights - countedDrop);
    });

    it('без action.supplement.application шага «Дополнить» в туре нет (#1740)', () => {
      grant(...USER_TOUR_RIGHTS.filter((k) => k !== 'action.supplement.application'));
      const store = useOnboardingStore();
      store.start({ tour: 'user' });

      expect(store.steps.some((s) => s.id === 'detail-supplement')).toBe(false);
      // Соседи по карточке правами не закрыты и остаются - выброшен ровно один шаг.
      expect(store.steps.some((s) => s.id === 'detail-duplicate')).toBe(true);
      expect(store.steps.some((s) => s.id === 'detail-revoke')).toBe(true);
    });

    it('с правом шаг «Дополнить» возвращается на своё место в карточке (#1740)', () => {
      grant(...USER_TOUR_RIGHTS);
      const store = useOnboardingStore();
      store.start({ tour: 'user' });

      const ids = store.steps.map((s) => s.id);
      expect(ids.indexOf('detail-questions')).toBeLessThan(ids.indexOf('detail-supplement'));
      expect(ids.indexOf('detail-supplement')).toBeLessThan(ids.indexOf('detail-revoke'));
    });

    it('режим super пропускает все requires-шаги', () => {
      const permissions = usePermissionsStore();
      permissions.mode = 'super';
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      expect(store.totalSteps).toBe(onboardingSteps.length);
    });

    it('в туре охраны тот же шаг гейтится тем же правом', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'guard' });
      expect(store.steps.some((s) => s.id === 'sec-header-feedback')).toBe(false);
    });
  });

  describe('гейтинг туров (availableTours)', () => {
    const keys = (store) => store.availableTours.map((t) => t.key);

    it('обычный аутентифицированный пользователь видит только тур заявителя', () => {
      useAuthStore().setTokens(createMockJWT({ username: 'ivanov' }));
      const store = useOnboardingStore();
      expect(keys(store)).toEqual(['user']);
    });

    it('неаутентифицированный не видит ни одного тура', () => {
      const store = useOnboardingStore();
      expect(keys(store)).toEqual([]);
    });

    it('охранник по типу пользователя видит тур охраны', () => {
      const auth = useAuthStore();
      auth.setTokens(createMockJWT({ username: 'guard1' }));
      auth.userTypeCode = 'security';
      const store = useOnboardingStore();
      expect(keys(store)).toEqual(['user', 'guard']);
    });

    it('тур охраны доступен и по праву page.available без типа security', () => {
      useAuthStore().setTokens(createMockJWT({ username: 'ivanov' }));
      grant('page.available');
      const store = useOnboardingStore();
      expect(keys(store)).toContain('guard');
    });

    it('тур охраны доступен по праву page.available', () => {
      useAuthStore().setTokens(createMockJWT({ username: 'ivanov' }));
      grant('page.available');
      const store = useOnboardingStore();
      expect(keys(store)).toContain('guard');
    });

    // Одного page.tables мало: большая часть тура живёт на «Доступных мне», куда
    // это право не пускает, и тур обрывался бы на первом шаге сегмента.
    it('тур охраны не появляется по одному праву page.tables', () => {
      useAuthStore().setTokens(createMockJWT({ username: 'ivanov' }));
      grant('page.tables');
      const store = useOnboardingStore();
      expect(keys(store)).not.toContain('guard');
    });

    it('согласующий (isReviewer) получает тур approve', async () => {
      getMyApprovalRole.mockResolvedValue({ is_approver: false, is_reviewer: true });
      grant('page.center'); // чужие заявки согласуют только из центра
      useAuthStore().setTokens(createMockJWT({ username: 'ivanov' }));
      const store = useOnboardingStore();
      await store.ensureApprovalRole();

      expect(store.tourContext.approvalRole).toEqual({ isApprover: false, isReviewer: true });
      // Тур принимающего согласующему не положен: тот голосует, а не берёт в работу.
      expect(keys(store)).toEqual(['user', 'approve']);
    });

    it('принимающий (isApprover) прошёл бы гейт accept', async () => {
      getMyApprovalRole.mockResolvedValue({ is_approver: true, is_reviewer: false });
      useAuthStore().setTokens(createMockJWT({ username: 'ivanov' }));
      const store = useOnboardingStore();
      await store.ensureApprovalRole();
      expect(store.tourContext.approvalRole).toEqual({ isApprover: true, isReviewer: false });
    });

    it('супер-админ-охранник видит всё, кроме тура принимающего', () => {
      const permissions = usePermissionsStore();
      permissions.mode = 'super';
      const auth = useAuthStore();
      auth.setTokens(createMockJWT({ username: 'root', is_super_admin: true }));
      auth.userTypeCode = 'security';
      const store = useOnboardingStore();
      // Право approve супер-админу даёт режим super, а вот accept гейтится
      // членством в справочнике принимающих - его никакие права не заменяют.
      expect(keys(store)).toEqual(['user', 'guard', 'approve', 'admin']);
    });
  });

  describe('pickAutostartTour (приоритет автозапуска)', () => {
    it('охрана приоритетнее заявителя', () => {
      const auth = useAuthStore();
      auth.setTokens(createMockJWT({ username: 'guard1' }));
      auth.userTypeCode = 'security';
      const store = useOnboardingStore();
      expect(store.pickAutostartTour().key).toBe('guard');
    });

    // Замок на баг #1771: раньше автозапуск после пройденного тура находил
    // следующий непройденный, и человеку сыпало туры один за другим - завершил
    // охрану, вернулся на «Обзор», поехал тур пользователя, и так по всем.
    it('после любого показанного тура автозапуск молчит - остальные только вручную', () => {
      const auth = useAuthStore();
      auth.setTokens(createMockJWT({ username: 'guard1' }));
      auth.userTypeCode = 'security';
      const store = useOnboardingStore();
      store.completedByTour = { guard: SECURITY_ONBOARDING_VERSION };
      expect(store.pickAutostartTour()).toBeNull();
    });

    it('брошенный на середине тур автозапуск тоже гасит', () => {
      const auth = useAuthStore();
      auth.setTokens(createMockJWT({ username: 'guard1' }));
      auth.userTypeCode = 'security';
      const store = useOnboardingStore();
      // Версия записана, но до финала не дошли - finishedTours пуст.
      store.completedByTour = { guard: SECURITY_ONBOARDING_VERSION };
      store.finishedTours = [];
      expect(store.pickAutostartTour()).toBeNull();
    });

    it('все доступные туры пройдены - автозапускать нечего', () => {
      const auth = useAuthStore();
      auth.setTokens(createMockJWT({ username: 'guard1' }));
      auth.userTypeCode = 'security';
      const store = useOnboardingStore();
      store.completedByTour = {
        guard: SECURITY_ONBOARDING_VERSION,
        user: ONBOARDING_VERSION,
      };
      expect(store.pickAutostartTour()).toBe(null);
    });

    it('без доступных туров - null', () => {
      const store = useOnboardingStore();
      expect(store.pickAutostartTour()).toBe(null);
    });
  });

  // Автозапуск выбирает тур один раз, поэтому недогруженный гейтинг - это не
  // «покажем позже», а «покажем не тот тур».
  describe('ensureGatingContext (гейтинг дожидается прав и роли)', () => {
    it('тянет права, тип пользователя и роль согласования', async () => {
      const auth = useAuthStore();
      auth.loadUserTypeCode = vi.fn(async () => { auth.userTypeCode = 'security'; });
      const permissions = usePermissionsStore();
      permissions.fetchPermissions = vi.fn(async () => { permissions.mode = 'super'; });

      const store = useOnboardingStore();
      await store.ensureGatingContext();

      expect(permissions.fetchPermissions).toHaveBeenCalledOnce();
      expect(auth.loadUserTypeCode).toHaveBeenCalledOnce();
      expect(getMyApprovalRole).toHaveBeenCalledOnce();
      expect(store.tourContext.isSecurity).toBe(true);
    });

    it('уже известный тип пользователя повторно не запрашивается', async () => {
      const auth = useAuthStore();
      auth.userTypeCode = 'organization';
      auth.loadUserTypeCode = vi.fn();
      usePermissionsStore().fetchPermissions = vi.fn();

      await useOnboardingStore().ensureGatingContext();
      expect(auth.loadUserTypeCode).not.toHaveBeenCalled();
    });

    it('после ожидания автозапуск видит тур, гейтящийся правом', async () => {
      const auth = useAuthStore();
      auth.setTokens(createMockJWT({ username: 'admin1' }));
      auth.loadUserTypeCode = vi.fn();
      const permissions = usePermissionsStore();
      // Права приезжают только внутри ensureGatingContext - до него их нет.
      permissions.fetchPermissions = vi.fn(async () => {
        permissions.mode = 'normal';
        permissions.effective = { 'page.available': { value: 'allow', source: 'base' } };
      });

      const store = useOnboardingStore();
      expect(store.pickAutostartTour().key).toBe('user');
      await store.ensureGatingContext();
      expect(store.pickAutostartTour().key).toBe('guard');
    });
  });

  describe('ensureApprovalRole (роль в согласовании)', () => {
    it('тянет роль один раз за сессию', async () => {
      getMyApprovalRole.mockResolvedValue({ is_approver: true, is_reviewer: true });
      const store = useOnboardingStore();
      await store.ensureApprovalRole();
      await store.ensureApprovalRole();

      expect(getMyApprovalRole).toHaveBeenCalledOnce();
      expect(store.approvalRole).toEqual({ isApprover: true, isReviewer: true });
    });

    it('конкурентные вызовы шлют один GET (in-flight guard)', async () => {
      let resolve;
      getMyApprovalRole.mockReturnValue(new Promise((r) => { resolve = r; }));
      const store = useOnboardingStore();
      const p1 = store.ensureApprovalRole();
      const p2 = store.ensureApprovalRole();
      resolve({ is_approver: true, is_reviewer: false });
      await Promise.all([p1, p2]);

      expect(getMyApprovalRole).toHaveBeenCalledOnce();
    });

    it('ошибка сети оставляет роль пустой и позволяет повтор', async () => {
      getMyApprovalRole.mockRejectedValue(new Error('network'));
      const store = useOnboardingStore();
      await store.ensureApprovalRole();

      expect(store.approvalRole).toEqual({ isApprover: false, isReviewer: false });
      expect(store.approvalRoleLoaded).toBe(false);
      await store.ensureApprovalRole();
      expect(getMyApprovalRole).toHaveBeenCalledTimes(2);
    });
  });

  describe('ensureFactRoute (сегмент фактовой таблицы охранника)', () => {
    it('резолвит route и добавляет сегмент отметки в хвост шагов тура охраны', async () => {
      getSecurityFactRoute.mockResolvedValue('/table/kpp_1');
      const permissions = usePermissionsStore();
      permissions.mode = 'super';
      const store = useOnboardingStore();
      store.start({ tour: 'guard' });
      const baseLen = securityOnboardingSteps.length;
      await store.ensureFactRoute();

      expect(store.factTableRoute).toBe('/table/kpp_1');
      expect(store.totalSteps).toBe(baseLen + 10);
      const tail = store.steps.slice(baseLen);
      expect(tail.map((s) => s.id)).toEqual([
        'sec-table-instruction',
        'sec-pass-intro',
        'sec-pass-row',
        'sec-pass-entry',
        'sec-pass-exit',
        'sec-on-territory',
        'sec-fact-intro',
        'sec-fact-report',
        'sec-fact-report-window',
        'sec-finish',
      ]);
      // шаги фактовой таблицы - на её route, а финал всегда на «Обзоре»: он
      // достижим любому вошедшему, и там же кнопка «Обучение» для повторного прохода
      expect(tail.slice(0, -1).every((s) => s.route === '/table/kpp_1')).toBe(true);
      const finalStep = tail[tail.length - 1];
      expect(finalStep.id).toBe('sec-finish');
      expect(finalStep.route).toBe('/news');
    });

    it('без доступной фактовой таблицы (null) сегмент не добавляется', async () => {
      getSecurityFactRoute.mockResolvedValue(null);
      const permissions = usePermissionsStore();
      permissions.mode = 'super';
      const store = useOnboardingStore();
      store.start({ tour: 'guard' });
      await store.ensureFactRoute();

      expect(store.factTableRoute).toBe(null);
      expect(store.totalSteps).toBe(securityOnboardingSteps.length + 1);
      const last = store.steps[store.steps.length - 1];
      expect(last.id).toBe('sec-finish');
      expect(last.route).toBe('/news');
    });

    it('идемпотентен - резолвит один раз за сессию', async () => {
      getSecurityFactRoute.mockResolvedValue('/table/kpp_1');
      const store = useOnboardingStore();
      await store.ensureFactRoute();
      await store.ensureFactRoute();

      expect(getSecurityFactRoute).toHaveBeenCalledOnce();
    });

    it('start тура охраны запускает резолв фоном', async () => {
      getSecurityFactRoute.mockResolvedValue('/table/kpp_1');
      const store = useOnboardingStore();
      store.start({ tour: 'guard' });

      expect(getSecurityFactRoute).toHaveBeenCalledOnce();
      await vi.waitFor(() => expect(store.factTableRoute).toBe('/table/kpp_1'));
    });

    it('start тура заявителя резолв не запускает', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      expect(getSecurityFactRoute).not.toHaveBeenCalled();
    });

    it('reset очищает route и позволяет резолву повториться для следующего юзера', async () => {
      getSecurityFactRoute.mockResolvedValue('/table/kpp_1');
      const store = useOnboardingStore();
      await store.ensureFactRoute();
      store.reset();

      expect(store.factTableRoute).toBe(null);
      await store.ensureFactRoute();
      expect(getSecurityFactRoute).toHaveBeenCalledTimes(2);
    });
  });

  describe('loadStatus / hasCompleted (per-user, per-tour через API)', () => {
    it('loadStatus тянет карту пройденных версий', async () => {
      getOnboardingStatus.mockResolvedValue({
        completed: { user: ONBOARDING_VERSION, guard: null, approve: null, accept: null, admin: null },
      });
      const store = useOnboardingStore();
      await store.loadStatus();

      expect(getOnboardingStatus).toHaveBeenCalledOnce();
      expect(store.completedByTour.user).toBe(ONBOARDING_VERSION);
      expect(store.statusLoaded).toBe(true);
      expect(store.hasCompleted('user')).toBe(true);
      expect(store.hasCompleted('guard')).toBe(false);
    });

    it('конкурентные loadStatus шлют один GET (guard от гонки)', async () => {
      let resolve;
      getOnboardingStatus.mockReturnValue(new Promise((r) => { resolve = r; }));
      const store = useOnboardingStore();
      const p1 = store.loadStatus();
      const p2 = store.loadStatus();
      resolve({ completed: {} });
      await Promise.all([p1, p2]);

      expect(getOnboardingStatus).toHaveBeenCalledOnce();
    });

    it('loadStatus при ошибке сети оставляет statusLoaded=false (fail-safe)', async () => {
      getOnboardingStatus.mockRejectedValue(new Error('network'));
      const store = useOnboardingStore();
      await store.loadStatus();

      expect(store.statusLoaded).toBe(false);
      expect(store.hasCompleted('user')).toBe(false);
    });

    it('hasCompleted сверяется с версией КОНКРЕТНОГО тура', async () => {
      getOnboardingStatus.mockResolvedValue({
        completed: { user: ONBOARDING_VERSION, guard: SECURITY_ONBOARDING_VERSION - 1 },
      });
      const store = useOnboardingStore();
      await store.loadStatus();

      expect(store.hasCompleted('user')).toBe(true);
      expect(store.hasCompleted('guard')).toBe(false);
    });

    it('подъём версии одного тура не сбрасывает остальные', async () => {
      getOnboardingStatus.mockResolvedValue({
        completed: { user: ONBOARDING_VERSION, guard: SECURITY_ONBOARDING_VERSION },
      });
      const store = useOnboardingStore();
      await store.loadStatus();
      // Имитируем подъём версии тура охраны: пройденная версия отстала на 1.
      store.completedByTour = { ...store.completedByTour, guard: SECURITY_ONBOARDING_VERSION - 1 };

      expect(store.hasCompleted('guard')).toBe(false);
      expect(store.isOutdated('guard')).toBe(true);
      expect(store.hasCompleted('user')).toBe(true);
      expect(store.isOutdated('user')).toBe(false);
    });

    it('isOutdated false у непройденного тура (он не «обновлён», а новый)', () => {
      const store = useOnboardingStore();
      expect(store.isOutdated('user')).toBe(false);
      expect(store.hasCompleted('user')).toBe(false);
    });

    it('hasCompleted/isOutdated незнакомого тура - false', () => {
      const store = useOnboardingStore();
      expect(store.hasCompleted('nope')).toBe(false);
      expect(store.isOutdated('nope')).toBe(false);
    });

    it('markCompleted пишет версию АКТИВНОГО тура локально и на бэкенд', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'guard' });
      store.markCompleted();

      expect(store.completedByTour.guard).toBe(SECURITY_ONBOARDING_VERSION);
      expect(store.hasCompleted('guard')).toBe(true);
      expect(store.hasCompleted('user')).toBe(false);
      expect(markOnboardingComplete).toHaveBeenCalledWith('guard', SECURITY_ONBOARDING_VERSION, false);
      // Закрыли на середине: запись есть (автозапуск гасим), «Пройден» нет.
      expect(store.hasFinished('guard')).toBe(false);
    });

    it('markCompleted(true) отмечает тур пройденным до конца', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'guard' });
      store.markCompleted(true);

      expect(store.hasFinished('guard')).toBe(true);
      expect(markOnboardingComplete).toHaveBeenCalledWith('guard', SECURITY_ONBOARDING_VERSION, true);
    });

    it('досмотр после пропуска доводит отметку до «Пройден»', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'guard' });
      store.markCompleted(false);
      store.stop();
      store.start({ tour: 'guard' });
      store.markCompleted(true);

      expect(store.hasFinished('guard')).toBe(true);
    });

    it('markCompleted без активного тура ничего не пишет', () => {
      const store = useOnboardingStore();
      store.markCompleted();
      expect(markOnboardingComplete).not.toHaveBeenCalled();
      expect(store.completedByTour).toEqual({});
    });

    it('markCompleted идемпотентен - повторный вызов не шлёт второй POST', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      store.markCompleted();
      store.markCompleted();
      expect(markOnboardingComplete).toHaveBeenCalledOnce();
    });

    it('markCompleted не падает при ошибке записи на бэкенд (fire-and-forget)', () => {
      markOnboardingComplete.mockRejectedValue(new Error('network'));
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      expect(() => store.markCompleted()).not.toThrow();
      expect(store.hasCompleted('user')).toBe(true);
    });
  });

  describe('revealOpen (сигнал раскрытия свёрнутого узла)', () => {
    it('setRevealOpen ставит и гасит цель', () => {
      const store = useOnboardingStore();
      expect(store.revealOpen).toBe(null);
      store.setRevealOpen('admin-column');
      expect(store.revealOpen).toBe('admin-column');
      store.setRevealOpen(null);
      expect(store.revealOpen).toBe(null);
    });

    it('reset гасит сигнал', () => {
      const store = useOnboardingStore();
      store.setRevealOpen('search-panel');
      store.reset();
      expect(store.revealOpen).toBe(null);
    });
  });

  describe('canShowTour', () => {
    it('false когда юзер не аутентифицирован', () => {
      const store = useOnboardingStore();
      expect(store.canShowTour).toBe(false);
    });

    it('true когда auth.isAuthenticated и зеркалит его значение', () => {
      const auth = useAuthStore();
      auth.setTokens(createMockJWT({ username: 'admin' }, 3600));

      const store = useOnboardingStore();
      expect(store.canShowTour).toBe(auth.isAuthenticated);
      expect(store.canShowTour).toBe(true);
    });

    it('false при истёкшем токене', () => {
      const auth = useAuthStore();
      auth.setTokens(createMockJWT({ username: 'admin' }, -100));

      const store = useOnboardingStore();
      expect(store.canShowTour).toBe(false);
    });

    // #1567: тур не должен подсвечивать интерфейс под неснимаемым окном согласия.
    it('false пока не дано согласие на обработку ПД, true после подтверждения', () => {
      const auth = useAuthStore();
      auth.setTokens(createMockJWT({ username: 'admin' }, 3600));
      const consent = usePDConsentStore();
      consent.resolved = true;
      consent.required = true;

      const store = useOnboardingStore();
      expect(store.canShowTour).toBe(false);

      consent.required = false;
      expect(store.canShowTour).toBe(true);
    });
  });

  describe('cross-page переходы', () => {
    it('advanceSegment сдвигает индекс на 1 и поднимает pendingSegment', () => {
      const store = useOnboardingStore();
      store.start({ tour: 'user' });
      store.setIndex(4);
      store.advanceSegment();
      expect(store.currentIndex).toBe(5);
      expect(store.pendingSegment).toBe(true);
    });

    it('clearPending сбрасывает флаг ожидания навигации', () => {
      const store = useOnboardingStore();
      store.advanceSegment();
      expect(store.pendingSegment).toBe(true);
      store.clearPending();
      expect(store.pendingSegment).toBe(false);
    });

    it('reset чистит pendingSegment вместе с состоянием', () => {
      const store = useOnboardingStore();
      store.advanceSegment();
      store.reset();
      expect(store.pendingSegment).toBe(false);
      expect(store.isActive).toBe(false);
      expect(store.currentIndex).toBe(0);
    });
  });
});
