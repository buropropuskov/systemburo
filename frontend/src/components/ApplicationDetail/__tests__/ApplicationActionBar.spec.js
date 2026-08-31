import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve({}) })),
}));

// Отзыв своего решения спрашивает подтверждение через ui-стор.
const confirmMock = vi.fn(() => Promise.resolve(true));
vi.mock('@/stores/ui', () => ({ useUiStore: () => ({ confirm: confirmMock }) }));

import ApplicationActionBar from '../ApplicationActionBar.vue';

const APPROVE = '[data-testid="app-detail-button-approve"]';
const REJECT = '[data-testid="app-detail-button-reject"]';
const HINT = '[data-testid="app-detail-blacklist-gate-hint"]';
const REVOKE = '[data-testid="app-detail-button-revoke-approval"]';
const TAKE = '[data-testid="app-detail-button-take-to-work"]';

function mountBar(props = {}) {
  return mount(ApplicationActionBar, {
    props: {
      application: { id: 1, confirmation: 'Согласование', status: 'Непрочитано' },
      currentUserId: 1,
      responsibleUsers: [{ id: 1, approval_status: 'pending' }],
      approvers: [],
      ...props,
    },
  });
}

describe('ApplicationActionBar - гейт ЧС (#481, срез 6b)', () => {
  it('при непереопределённом флаге кнопка "Согласовать" заблокирована и видна подсказка', () => {
    const wrapper = mountBar({ hasUnoverriddenBlacklistFlags: true });
    expect(wrapper.find(APPROVE).attributes('disabled')).toBeDefined();
    expect(wrapper.find(HINT).exists()).toBe(true);
  });

  it('без флагов кнопка "Согласовать" активна и подсказки нет', () => {
    const wrapper = mountBar({ hasUnoverriddenBlacklistFlags: false });
    expect(wrapper.find(APPROVE).attributes('disabled')).toBeUndefined();
    expect(wrapper.find(HINT).exists()).toBe(false);
  });

  it('гейт не блокирует "Отказать" - отклонить помеченную заявку можно сразу', () => {
    const wrapper = mountBar({ hasUnoverriddenBlacklistFlags: true });
    expect(wrapper.find(REJECT).attributes('disabled')).toBeUndefined();
  });

  it('для совмещённой роли блокируется "Согласовать и принять"', () => {
    const wrapper = mountBar({
      hasUnoverriddenBlacklistFlags: true,
      approvers: [{ user_id: 1 }],
    });
    const approve = wrapper.find(APPROVE);
    expect(approve.text()).toContain('Согласовать и принять');
    expect(approve.attributes('disabled')).toBeDefined();
    expect(wrapper.find(HINT).exists()).toBe(true);
  });

  it('после голоса approve-кнопки нет - гейт и подсказка не показываются', () => {
    const wrapper = mountBar({
      hasUnoverriddenBlacklistFlags: true,
      responsibleUsers: [{ id: 1, approval_status: 'approved' }],
    });
    expect(wrapper.find(APPROVE).exists()).toBe(false);
    expect(wrapper.find(HINT).exists()).toBe(false);
  });
});

describe('ApplicationActionBar - отозванная заявка (#951)', () => {
  it('approver/responsible-действия скрыты, виден бейдж "Отозвана инициатором"', () => {
    const wrapper = mountBar({
      application: { id: 1, confirmation: 'Согласование', status: 'Отозвана' },
      responsibleUsers: [{ id: 1, approval_status: 'pending' }],
      approvers: [{ user_id: 1 }],
    });
    expect(wrapper.find(APPROVE).exists()).toBe(false);
    expect(wrapper.find(REJECT).exists()).toBe(false);
    expect(wrapper.text()).toContain('Отозвана инициатором');
  });
});

describe('ApplicationActionBar - busy-лоадер вместо старых кнопок (#1097 R4-7)', () => {
  const LOADER = '.actions-ready-loader';

  it('пока идёт действие (processing) - показываем лоадер, кнопки скрыты', () => {
    const wrapper = mountBar({ processing: true });
    expect(wrapper.find(LOADER).exists()).toBe(true);
    expect(wrapper.find(APPROVE).exists()).toBe(false);
    expect(wrapper.find(REJECT).exists()).toBe(false);
  });

  it('пока идёт смена согласования (updatingConfirmation) - лоадер, кнопки скрыты', () => {
    const wrapper = mountBar({ updatingConfirmation: true });
    expect(wrapper.find(LOADER).exists()).toBe(true);
    expect(wrapper.find(APPROVE).exists()).toBe(false);
  });

  it('пока идёт рефетч после действия (ready=false) - лоадер, кнопки скрыты', () => {
    const wrapper = mountBar({ ready: false });
    expect(wrapper.find(LOADER).exists()).toBe(true);
    expect(wrapper.find(APPROVE).exists()).toBe(false);
  });

  it('в спокойном состоянии (не busy) - кнопки есть, лоадера нет', () => {
    const wrapper = mountBar();
    expect(wrapper.find(LOADER).exists()).toBe(false);
    expect(wrapper.find(APPROVE).exists()).toBe(true);
  });
});

describe('ApplicationActionBar - инвариант barKey для cross-fade (#1097 R5-S4)', () => {
  it('ключ меняется при смене статуса (набор кнопок пересобирается -> cross-fade)', async () => {
    const wrapper = mountBar({
      application: { id: 1, confirmation: 'Согласовано', status: 'В работе' },
      responsibleUsers: [{ id: 1, approval_status: 'approved' }],
    });
    const before = wrapper.vm.barKey;
    await wrapper.setProps({ application: { id: 1, confirmation: 'Согласовано', status: 'Завершено' } });
    expect(wrapper.vm.barKey).not.toBe(before);
  });

  it('busy -> ключ "busy" (кнопки свапаются на лоадер)', async () => {
    const wrapper = mountBar();
    expect(wrapper.vm.barKey).not.toBe('busy');
    await wrapper.setProps({ processing: true });
    expect(wrapper.vm.barKey).toBe('busy');
  });

  it('флаг ЧС ключ НЕ меняет - остаёмся в той же ветке (без лишнего cross-fade)', async () => {
    const wrapper = mountBar({ hasUnoverriddenBlacklistFlags: false });
    const before = wrapper.vm.barKey;
    await wrapper.setProps({ hasUnoverriddenBlacklistFlags: true });
    expect(wrapper.vm.barKey).toBe(before);
  });
});

describe('ApplicationActionBar - принять можно только после согласования (#1227 follow-up)', () => {
  const withMandatoryPending = {
    approvers: [{ user_id: 1 }],
    currentUserId: 1,
    responsibleUsers: [
      { id: 1, required_approval: false, approval_status: 'pending' },
      { id: 2, required_approval: true, approval_status: 'pending' },
    ],
  };
  const soloNonRequired = {
    approvers: [{ user_id: 1 }],
    currentUserId: 1,
    responsibleUsers: [{ id: 1, required_approval: false, approval_status: 'pending' }],
  };

  it('обязательный другой согласующий pending -> голос НЕ завершает согласование, лейбл "Согласовать"', () => {
    const wrapper = mountBar(withMandatoryPending);
    expect(wrapper.vm.approvingCompletesConfirmation).toBe(false);
    const btn = wrapper.find(APPROVE);
    expect(btn.text()).toContain('Согласовать');
    expect(btn.text()).not.toContain('Согласовать и принять');
  });

  it('единственный необязательный (текущий) -> голос завершает, лейбл "Согласовать и принять"', () => {
    const wrapper = mountBar(soloNonRequired);
    expect(wrapper.vm.approvingCompletesConfirmation).toBe(true);
    expect(wrapper.find(APPROVE).text()).toContain('Согласовать и принять');
  });

  it('заявка без согласующих -> голос завершает (принять можно)', () => {
    const wrapper = mountBar({ approvers: [{ user_id: 1 }], currentUserId: 1, responsibleUsers: [] });
    expect(wrapper.vm.approvingCompletesConfirmation).toBe(true);
  });

  it('комбо-клик при незавершённом согласовании: approve есть, take-to-work НЕ вызывается', async () => {
    const { apiRequest } = await import('@/api/client');
    apiRequest.mockClear();
    const wrapper = mountBar(withMandatoryPending);
    await wrapper.find(APPROVE).trigger('click');
    await flushPromises();
    const urls = apiRequest.mock.calls.map(c => c[0]);
    expect(urls.some(u => u.includes('/approve'))).toBe(true);
    expect(urls.some(u => u.includes('/take-to-work'))).toBe(false);
  });

  it('комбо-клик при завершающем голосе: approve + take-to-work', async () => {
    const { apiRequest } = await import('@/api/client');
    apiRequest.mockClear();
    const wrapper = mountBar(soloNonRequired);
    await wrapper.find(APPROVE).trigger('click');
    await flushPromises();
    const urls = apiRequest.mock.calls.map(c => c[0]);
    expect(urls.some(u => u.includes('/approve'))).toBe(true);
    expect(urls.some(u => u.includes('/take-to-work'))).toBe(true);
  });
});

// Заявка от организации, которую завели из самой заявки, согласующих не имеет: брать их
// неоткуда, пользователей у такой организации ещё нет. Сервер такую заявку принять
// позволяет (application_workflow_service, ветка accept), а интерфейс показывал кнопки
// только при confirmation='Согласовано' - и заявка застревала навсегда.
describe('ApplicationActionBar - приём заявки без согласующих', () => {
  it('без согласующих кнопки принять и отказать показываются при confirmation=Согласование', () => {
    const wrapper = mountBar({
      application: { id: 1, confirmation: 'Согласование', status: 'Непрочитано' },
      responsibleUsers: [],
      approvers: [{ user_id: 1 }],
    });

    expect(wrapper.find(TAKE).exists()).toBe(true);
    expect(wrapper.find(REJECT).exists()).toBe(true);
  });

  it('с согласующими, которые ещё не проголосовали, кнопки принять нет', () => {
    const wrapper = mountBar({
      application: { id: 1, confirmation: 'Согласование', status: 'Непрочитано' },
      responsibleUsers: [{ id: 7, approval_status: 'pending' }],
      approvers: [{ user_id: 1 }],
      currentUserId: 1,
    });

    expect(wrapper.find(TAKE).exists()).toBe(false);
  });

  it('после завершённого согласования кнопка принять есть и с согласующими', () => {
    const wrapper = mountBar({
      application: { id: 1, confirmation: 'Согласовано', status: 'Непрочитано' },
      responsibleUsers: [{ id: 7, approval_status: 'approved' }],
      approvers: [{ user_id: 1 }],
      currentUserId: 1,
    });

    expect(wrapper.find(TAKE).exists()).toBe(true);
  });
});

// Пользователь из справочника принимающих, назначенный согласующим по заявке, попадал в
// ветку совмещённой роли - а в ней кнопки отзыва своего решения не было вовсе. На заявке,
// ждущей обязательного согласующего, у него оставался один бейдж без единого действия.
describe('ApplicationActionBar - отзыв своего решения при совмещённой роли (#1550)', () => {
  const dualRole = (props = {}) => ({
    currentUserId: 1,
    approvers: [{ user_id: 1 }],
    responsibleUsers: [
      { id: 1, required_approval: false, approval_status: 'approved' },
      { id: 2, required_approval: true, approval_status: 'pending' },
    ],
    ...props,
  });

  it('ждём обязательного согласующего: решение отзывается, свой голос виден', () => {
    const wrapper = mountBar(dualRole({
      application: { id: 1, confirmation: 'Согласование', status: 'В обработке' },
    }));

    expect(wrapper.find(REVOKE).exists()).toBe(true);
    expect(wrapper.text()).toContain('Вы согласовали (ожидание других)');
  });

  it('согласование завершено: отзыв соседствует с "Принять"', () => {
    const wrapper = mountBar(dualRole({
      application: { id: 1, confirmation: 'Согласовано', status: 'В обработке' },
      responsibleUsers: [
        { id: 1, required_approval: false, approval_status: 'approved' },
        { id: 2, required_approval: true, approval_status: 'approved' },
      ],
    }));

    expect(wrapper.find(TAKE).exists()).toBe(true);
    expect(wrapper.find(REVOKE).exists()).toBe(true);
  });

  it('заявка уже в работе: решение не отзывается, доступен отзыв из работы', () => {
    const wrapper = mountBar(dualRole({
      application: { id: 1, confirmation: 'Согласовано', status: 'В работе' },
    }));

    expect(wrapper.find(REVOKE).exists()).toBe(false);
    expect(wrapper.text()).toContain('Отозвать из работы');
  });

  it('клик по кнопке отзывает решение на бэкенде', async () => {
    const { apiRequest } = await import('@/api/client');
    apiRequest.mockClear();
    confirmMock.mockResolvedValueOnce(true);

    const wrapper = mountBar(dualRole({
      application: { id: 95, confirmation: 'Согласование', status: 'В обработке' },
    }));
    await wrapper.find(REVOKE).trigger('click');
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledWith('/applications/95/revoke-approval', expect.objectContaining({ method: 'POST' }));
  });

  it('отказ в подтверждении ничего не отзывает', async () => {
    const { apiRequest } = await import('@/api/client');
    apiRequest.mockClear();
    confirmMock.mockResolvedValueOnce(false);

    const wrapper = mountBar(dualRole({
      application: { id: 95, confirmation: 'Согласование', status: 'В обработке' },
    }));
    await wrapper.find(REVOKE).trigger('click');
    await flushPromises();

    expect(apiRequest).not.toHaveBeenCalled();
  });

  it('на узком экране подпись сокращается - ряд из трёх кнопок не переносится', async () => {
    const original = window.matchMedia;
    window.matchMedia = () => ({ matches: true, addEventListener: () => {}, removeEventListener: () => {} });
    try {
      const wrapper = mountBar(dualRole({
        application: { id: 1, confirmation: 'Согласовано', status: 'В обработке' },
        responsibleUsers: [
          { id: 1, required_approval: false, approval_status: 'approved' },
          { id: 2, required_approval: true, approval_status: 'approved' },
        ],
      }));
      await flushPromises();

      expect(wrapper.find(REVOKE).text()).toBe('Отозвать');
    } finally {
      window.matchMedia = original;
    }
  });

  it('на десктопе подпись полная', () => {
    const wrapper = mountBar(dualRole({
      application: { id: 1, confirmation: 'Согласование', status: 'В обработке' },
    }));

    expect(wrapper.find(REVOKE).text()).toBe('Отозвать своё решение');
  });

  it('согласующий без роли принимающего отзыв видит по-прежнему', () => {
    const wrapper = mountBar({
      application: { id: 1, confirmation: 'Согласование', status: 'В обработке' },
      currentUserId: 1,
      approvers: [],
      responsibleUsers: [{ id: 1, required_approval: false, approval_status: 'approved' }],
    });

    expect(wrapper.find(REVOKE).exists()).toBe(true);
  });
});

// На 390 ряд действий не переносится, поэтому кнопка отзыва не должна нести свою ширину:
// с "Принять" и "Отказать" рядом (120px каждая) её 140px выталкивают тройку за вьюпорт.
describe('ApplicationActionBar - ширина отзыва решения на мобилке (#1550)', () => {
  const src = readFileSync(resolve(__dirname, '../ApplicationActionBar.vue'), 'utf8');
  const mobile = src.slice(src.indexOf('@media (max-width: 768px)'));

  it('мобильное правило снимает min-width', () => {
    const rule = mobile.match(/\.subtle-btn\.revoke-approval-btn\s*\{([\s\S]*?)\}/);
    expect(rule).not.toBeNull();
    expect(rule[1]).toContain('min-width: auto');
  });

  // Раньше здесь проверялся порядок объявлений (.subtle-btn ниже по файлу). Порядок
  // перестал быть опорой, когда мобильный @media переехал в конец блока стилей (#4), а
  // защита осталась прежней: min-width: 140px приходит от .subtle-btn, и снять его на
  // мобилке можно только правилом с большей специфичностью. Одиночный
  // .revoke-approval-btn её не даёт - он равен .subtle-btn и выигрывает лишь пока стоит
  // ниже, то есть ровно то, на что уже наступали.
  it('селектор из двух классов - одиночный .revoke-approval-btn не перебил бы min-width', () => {
    expect(mobile).toMatch(/\.subtle-btn\.revoke-approval-btn\s*\{/);
    expect(mobile).not.toMatch(/(?<![\w.-])\.revoke-approval-btn\s*\{/);
  });
});

/*
 * jsdom не считает :hover, поэтому контракт наведения сверяем по объявлениям в SFC.
 * Правило зелёных кнопок повторяло базовый var(--success) - наведение на
 * "Согласовать"/"Согласовать и принять"/"Принять" не давало отклика, хотя у соседней
 * "Отказать" он был.
 */
describe('ApplicationActionBar - hover кнопок принятия', () => {
  const src = readFileSync(resolve(__dirname, '../ApplicationActionBar.vue'), 'utf8');

  const rule = (head) => {
    const m = src.match(new RegExp(`${head}\\s*\\{([\\s\\S]*?)\\}`));
    return m && m[1].replace(/\/\*[\s\S]*?\*\//g, '').replace(/\s+/g, ' ').trim();
  };
  const base = rule('\\.confirm-btn, \\.accept-btn');
  const hover = rule('\\.confirm-btn:hover:not\\(:disabled\\), \\.accept-btn:hover:not\\(:disabled\\)');

  it('фон на наведении отличается от базового', () => {
    expect(base).toContain('background: var(--success)');
    expect(hover).not.toBe(base);
    expect(hover).not.toContain('background: var(--success);');
  });

  it('оттенок берётся от цвета текста - в тёмной теме кнопка светлеет, как у "Отказать"', () => {
    expect(hover).toContain('color-mix(in srgb, var(--success) 85%, var(--text))');
    expect(rule('\\.reject-btn:hover:not\\(:disabled\\)')).toContain('var(--text)');
  });

  it('переход анимирует цвет, а не all', () => {
    const shared = rule('\\.confirm-btn, \\.reject-btn, \\.accept-btn');
    expect(shared).toContain('transition: background-color');
    expect(shared).not.toContain('transition: all');
  });
});

describe('ApplicationActionBar - роль принимающего приходит признаком, а не списком', () => {
  const props = {
    application: { id: 1, confirmation: 'Согласовано', status: 'В обработке' },
    currentUserId: 7,
    responsibleUsers: [],
  };

  it('принимающий без прав администратора видит "Принять": состав принимающих ему не отдаётся', () => {
    const wrapper = mountBar({ ...props, approvers: [], isApprover: true });
    expect(wrapper.find(TAKE).exists()).toBe(true);
  });

  it('без признака и без состава кнопок приёма нет', () => {
    const wrapper = mountBar({ ...props, approvers: [], isApprover: false });
    expect(wrapper.find(TAKE).exists()).toBe(false);
  });

  it('администратору роль по-прежнему видна из загруженного состава', () => {
    const wrapper = mountBar({ ...props, approvers: [{ user_id: 7 }], isApprover: false });
    expect(wrapper.find(TAKE).exists()).toBe(true);
  });
});

/**
 * Подсказка гейта стоит в одном ряду со статусными бейджами. Формулировка
 * «Подтвердите пропуск по помеченным» не влезала в ширину плашки, переносилась на
 * вторую строку и распирала весь ряд по высоте (жалоба владельца). Смысл при этом
 * терять нельзя, поэтому полная фраза осталась подсказкой при наведении.
 */
describe('ApplicationActionBar - подсказка гейта ЧС держится в одну строку', () => {
  const SFC = readFileSync(resolve(__dirname, '../ApplicationActionBar.vue'), 'utf8');

  it('текст короткий', () => {
    const wrapper = mountBar({ hasUnoverriddenBlacklistFlags: true });
    const text = wrapper.find(HINT).text();
    expect(text).toBe('Подтвердите метки ЧС');
  });

  it('перенос запрещён стилем, ширина плашки его больше не режет', () => {
    const rule = SFC.match(/\.blacklist-gate-hint\s*\{([^}]*)\}/);
    expect(rule).not.toBeNull();
    expect(rule[1]).toMatch(/white-space:\s*nowrap/);
    expect(rule[1]).not.toMatch(/max-width/);
  });

  it('полная формулировка осталась в подсказке при наведении', () => {
    const wrapper = mountBar({ hasUnoverriddenBlacklistFlags: true });
    expect(wrapper.find(HINT).attributes('title'))
      .toBe('Подтвердите пропуск по всем помеченным элементам, чтобы согласовать заявку');
  });
});
