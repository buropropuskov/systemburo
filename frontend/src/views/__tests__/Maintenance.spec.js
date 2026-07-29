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
    expect(wrapper.get('[data-testid="maintenance-window"]').text()).toContain('осталось');
    expect(wrapper.get('[data-testid="maintenance-window"]').text()).toContain('окончание');
  });

  it('прогресс не выходит за 100% после окончания окна', () => {
    wrapper = mountPage({ planned_start: iso(-3 * HOUR), planned_end: iso(-HOUR) });
    expect(wrapper.vm.progressPercent).toBe(100);
    expect(wrapper.vm.remainingText).toBe('');
  });

  it('терминал подписывает сроки словами, а не голыми метками времени', () => {
    wrapper = mountPage({ planned_start: iso(-HOUR), planned_end: iso(HOUR) });
    const log = wrapper.get('[data-testid="maintenance-window"]');
    expect(log.text()).toContain('начало');
    expect(log.text()).toContain('окончание');
    expect(log.text()).toContain('осталось');
    // Дата и время целиком, чтобы срок читался без догадок.
    expect(log.text()).toMatch(/\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}/);
    expect(log.text()).toContain('maintenance@buropropuskov:~$');
    expect(log.find('.mt__caret').exists()).toBe(true);
  });

  it('после истечения срока статус меняется на завершение', () => {
    wrapper = mountPage({ planned_start: iso(-3 * HOUR), planned_end: iso(-HOUR) });
    expect(wrapper.vm.statusText).toBe('завершаем, проверяем систему');
  });

  it('без окна не рисует ни полосу прогресса, ни строку завершения', () => {
    wrapper = mountPage({ started_at: iso(-HOUR) });
    expect(wrapper.find('.mt__progress').exists()).toBe(false);
    const log = wrapper.get('[data-testid="maintenance-window"]');
    expect(log.text()).not.toContain('окончание');
    expect(log.text()).toContain('работы идут');
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
    // Номер из настроек приводится к маске проекта при показе.
    expect(links[1].text()).toBe('+7 (495) 123 45-67');
  });

  it('приводит к маске телефон, сохранённый цифрами подряд', () => {
    wrapper = mountPage({ support_phone: '79100830055' });
    const link = wrapper.get('.mt__meta a');
    expect(link.text()).toBe('+7 (910) 083 00-55');
    expect(link.attributes('href')).toBe('tel:+79100830055');
  });

  it('не выдумывает контакты, когда они не заданы', () => {
    wrapper = mountPage({ message: 'работы' });
    expect(wrapper.find('.mt__meta').exists()).toBe(false);
  });
});
