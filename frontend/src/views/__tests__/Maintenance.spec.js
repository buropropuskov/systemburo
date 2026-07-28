import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

import Maintenance from '../Maintenance.vue';
import { useMaintenanceStore } from '@/stores/maintenance';

const HOUR = 60 * 60 * 1000;

function iso(offsetMs) {
  return new Date(Date.now() + offsetMs).toISOString();
}

function mountPage(payload) {
  useMaintenanceStore().setFromPayload({ enabled: true, ...payload });
  return mount(Maintenance);
}

describe('Maintenance - страница технических работ', () => {
  let wrapper;

  beforeEach(() => {
    setActivePinia(createPinia());
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('выделяет сообщение администратора', () => {
    wrapper = mountPage({ message: 'Переносим базу на новый сервер' });
    const message = wrapper.get('[data-testid="maintenance-message"]');
    expect(message.text()).toBe('Переносим базу на новый сервер');
    expect(message.classes()).toContain('mt__lede--announced');
  });

  it('без объявления показывает текст по умолчанию обычным начертанием', () => {
    wrapper = mountPage({ message: '' });
    const message = wrapper.get('[data-testid="maintenance-message"]');
    expect(message.text()).toContain('Обновляем систему пропусков');
    expect(message.classes()).not.toContain('mt__lede--announced');
  });

  it('считает прогресс и остаток по объявленному окну', () => {
    wrapper = mountPage({ planned_start: iso(-HOUR), planned_end: iso(HOUR) });
    expect(wrapper.vm.progressPercent).toBeGreaterThanOrEqual(49);
    expect(wrapper.vm.progressPercent).toBeLessThanOrEqual(51);
    expect(wrapper.vm.remainingText).toMatch(/^(1 ч|60 мин|59 мин)/);
    expect(wrapper.get('[data-testid="maintenance-window"]').text()).toContain('Осталось');
  });

  it('прогресс не выходит за 100% после окончания окна', () => {
    wrapper = mountPage({ planned_start: iso(-3 * HOUR), planned_end: iso(-HOUR) });
    expect(wrapper.vm.progressPercent).toBe(100);
    expect(wrapper.vm.remainingText).toBe('');
  });

  it('без окна не рисует полосу прогресса и пишет, что срок уточняется', () => {
    wrapper = mountPage({ started_at: iso(-HOUR) });
    expect(wrapper.find('.mt__progress').exists()).toBe(false);
    expect(wrapper.get('[data-testid="maintenance-window"]').text()).toContain('уточняется');
  });

  it('показывает оба контакта ссылками, телефон - без разделителей в href', () => {
    wrapper = mountPage({
      support_email: 'help@example.com',
      support_phone: '+7 495 123-45-67',
    });
    const links = wrapper.findAll('.mt__meta a');
    expect(links).toHaveLength(2);
    expect(links[0].attributes('href')).toBe('mailto:help@example.com');
    expect(links[1].attributes('href')).toBe('tel:+74951234567');
    expect(links[1].text()).toBe('+7 495 123-45-67');
  });

  it('не выдумывает контакты, когда они не заданы', () => {
    wrapper = mountPage({ message: 'работы' });
    expect(wrapper.find('.mt__meta').exists()).toBe(false);
  });
});
