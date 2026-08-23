import { describe, it, expect, afterEach } from 'vitest';
import { createDemoResponder, syncDemoBackend } from '../demoBackend';
import { DEMO_APPLICATION_ID } from '../demoApplication';
import { interceptRead } from '@/api/readInterceptor';

/**
 * Тур одинаков для всех: человеку без своих заявок список кабинета и карточка на
 * время обучения показывают примерную заявку. Замки стерегут две вещи - что
 * пример действительно приходит туда, где его ждут шаги, и что он исчезает
 * вместе с туром, не задев ни одной записи.
 */
describe('примерные данные тура', () => {
  afterEach(() => syncDemoBackend(false));

  const respond = createDemoResponder();

  it('список кабинета отдаёт одну примерную заявку', () => {
    const body = respond('/applications/user?page=1&per_page=30');
    expect(body.success).toBe(true);
    expect(body.data).toHaveLength(1);
    expect(body.data[0].id).toBe(DEMO_APPLICATION_ID);
    expect(body.meta.total).toBe(1);
  });

  it('у примерной заявки есть согласующие и состав - шаги про них не пустуют', () => {
    const users = respond(`/applications/${DEMO_APPLICATION_ID}/responsible-users`);
    const attachments = respond(`/applications/${DEMO_APPLICATION_ID}/attachments`);
    expect(users.data.length).toBeGreaterThan(1);
    expect(users.data.some((u) => u.approval_status === 'approved')).toBe(true);
    expect(users.data.some((u) => u.approval_status === 'pending')).toBe(true);
    expect(attachments.data[0].attachment_display_name).toBeTruthy();
  });

  it('чужие пути не трогаем - живой бэкенд отвечает сам', () => {
    expect(respond('/applications/157/responsible-users')).toBe(null);
    expect(respond('/cars')).toBe(null);
    expect(respond('/applications/center?page=1')).toBe(null);
  });

  it('подмена работает только на чтение', () => {
    syncDemoBackend(true, false);
    expect(interceptRead('/applications/user?page=1', { method: 'GET' })).not.toBe(null);
    expect(interceptRead('/applications/user', { method: 'POST' })).toBe(null);
    expect(interceptRead('/applications/user', { method: 'DELETE' })).toBe(null);
  });

  it('отметка о прочтении примера не уходит на сервер - там такой заявки нет', () => {
    syncDemoBackend(true, false);
    const read = interceptRead(`/applications/${DEMO_APPLICATION_ID}/read`, { method: 'POST' });
    expect(read).not.toBe(null);
    expect(read.status).toBe(200);
    // чужая заявка запись не перехватывает
    expect(interceptRead('/applications/157/read', { method: 'POST' })).toBe(null);
  });

  it('по концу тура подмена снимается - следующий запрос идёт живым', () => {
    syncDemoBackend(true, false);
    expect(interceptRead('/applications/user?page=1', {})).not.toBe(null);

    syncDemoBackend(false);
    expect(interceptRead('/applications/user?page=1', {})).toBe(null);
  });

  it('ответ - настоящий Response с телом в форме конверта', async () => {
    syncDemoBackend(true, false);
    const response = interceptRead('/applications/user?page=1', {});
    expect(response.status).toBe(200);
    const body = await response.json();
    expect(body).toMatchObject({ success: true });
    expect(Array.isArray(body.data)).toBe(true);
  });

  it('со своей заявкой подмена не поднимается вовсе', () => {
    syncDemoBackend(true, true);
    expect(interceptRead('/applications/user?page=1', {})).toBe(null);
  });

  it('примерная заявка помечена как пример - интерфейс может это показать', () => {
    const body = respond('/applications/user?page=1');
    expect(body.data[0].is_demo).toBe(true);
    // Идентификатор заведомо вне живого диапазона: в логах не спутать
    expect(body.data[0].id).toBeGreaterThan(900000000);
  });
});
