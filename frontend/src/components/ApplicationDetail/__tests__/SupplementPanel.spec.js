import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import SupplementPanel from '../SupplementPanel.vue';

const ME = 7;

// Форма раунда - как её отдаёт GET /applications/:id/supplements (services.SupplementInfo).
function round(overrides = {}) {
  return {
    id: 11,
    application_id: 1,
    number: 2,
    status: 'pending',
    comment: 'Подвозим ещё две машины',
    created_by_user_id: 5,
    created_by_name: 'Сидоров Пётр Иванович',
    created_at: '2026-08-05T09:30:00Z',
    confirmation_datetime: null,
    decided_by_user_id: null,
    decision_comment: null,
    decided_at: null,
    counts: { vehicles: 2, employees: 1, items: 0 },
    approvals: [
      {
        user_id: ME,
        username: 'ivanov',
        full_name: 'Иванов Иван Иванович',
        required_approval: true,
        approval_status: 'approved',
        approval_comment: 'Пропустить',
        approval_datetime: '2026-08-05T10:00:00Z',
      },
      {
        user_id: 9,
        username: 'petrov',
        full_name: 'Петров Пётр Петрович',
        required_approval: false,
        approval_status: 'pending',
        approval_comment: null,
        approval_datetime: null,
      },
    ],
    ...overrides,
  };
}

function mountPanel(props = {}) {
  return mount(SupplementPanel, {
    props: { supplements: [round()], currentUserId: ME, ...props },
  });
}

describe('SupplementPanel - раунд и его голосующие (#1685)', () => {
  it('показывает номер раунда, статус, автора и состав добавленного', () => {
    const text = mountPanel().text();
    expect(text).toContain('Дополнение №2');
    expect(text).toContain('На согласовании');
    expect(text).toContain('Сидоров Пётр Иванович');
    expect(text).toContain('2 машины, 1 сотрудник');
    expect(text).toContain('Подвозим ещё две машины');
  });

  it('перечисляет голосующих с их решениями и комментариями', () => {
    const text = mountPanel().text();
    expect(text).toContain('Согласующие дополнения (2)');
    expect(text).toContain('Иванов Иван Иванович');
    expect(text).toContain('Согласовано');
    expect(text).toContain('Пропустить');
    expect(text).toContain('Петров Пётр Петрович');
    expect(text).toContain('Ожидание');
    expect(text).toContain('Обязательно');
  });

  it('пустой approval_status читается как ожидание, а не как «неизвестно»', () => {
    const wrapper = mountPanel({
      supplements: [round({
        approvals: [{ user_id: 9, username: 'petrov', full_name: 'Петров П. П.', required_approval: false, approval_status: null }],
      })],
    });
    expect(wrapper.text()).toContain('Ожидание');
    expect(wrapper.text()).not.toContain('Неизвестно');
  });

  it('решение принимающего показывается со временем и комментарием', () => {
    const wrapper = mountPanel({
      supplements: [round({
        status: 'accepted',
        decided_by_user_id: 3,
        decision_comment: 'Строки на пост',
        decided_at: '2026-08-05T12:00:00Z',
      })],
    });
    const decision = wrapper.find('[data-testid="supplement-decision-11"]');
    expect(decision.exists()).toBe(true);
    expect(decision.text()).toContain('Принято');
    expect(decision.text()).toContain('Строки на пост');
  });

  it('закрытые раунды остаются в списке вместе с идущим - по ним видно, почему строки не встали', () => {
    const wrapper = mountPanel({
      supplements: [round({ id: 11, number: 2, status: 'pending' }), round({ id: 10, number: 1, status: 'refused' })],
    });
    expect(wrapper.find('[data-testid="supplement-round-11"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="supplement-round-10"]').exists()).toBe(true);
    expect(wrapper.text()).toContain('Отказано');
  });

  it('влитый раунд не рисует заголовок голосующих - отдельного круга по нему не было', () => {
    const wrapper = mountPanel({
      supplements: [round({ status: 'merged', approvals: [] })],
    });
    expect(wrapper.text()).toContain('Влито в заявку');
    expect(wrapper.text()).not.toContain('Согласующие дополнения');
  });

  it('ошибка загрузки показывается человеческим текстом, панель остаётся на месте', () => {
    const wrapper = mountPanel({ supplements: [], error: 'Не удалось загрузить дополнения заявки' });
    expect(wrapper.find('[data-testid="supplement-panel"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="supplement-panel-error"]').text())
      .toBe('Не удалось загрузить дополнения заявки');
    // Пустышка «Дополнений по заявке нет» на ошибке врала бы: раунды могут быть, их не отдали.
    expect(wrapper.text()).not.toContain('Дополнений по заявке нет');
  });
});
