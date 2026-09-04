import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import {
  syncServerTime,
  serverNow,
  moscowParts,
  moscowHour,
  formatMoscowDateTime,
  isServerTimeSynced,
} from '../serverTime';

/**
 * Время интерфейса берётся с сервера и показывается по Москве (#2298).
 *
 * На посту по этим часам сверяют срок действия пропуска и разрешённые часы въезда.
 * Ошибка здесь тихая: экран показывает правдоподобное время, а решение о пропуске
 * получается неверным - ни пользователь, ни тест на «отрисовалось» этого не поймают.
 *
 * Поэтому проверяем обе беды по отдельности: сбитые часы машины (лечатся смещением)
 * и чужой часовой пояс (лечится форматированием в Europe/Moscow). Их легко перепутать
 * и закрыть только одну.
 */

const ответСо = (dateHeader) => ({ headers: { get: (n) => (n === 'date' ? dateHeader : null) } });

describe('серверное московское время', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // Машина в Гринвиче: расхождение с Москвой будет видно как +3 часа.
    vi.setSystemTime(new Date('2026-09-04T12:00:00Z'));
  });
  afterEach(() => vi.useRealTimers());

  it('до сверки показывает время машины, а не пустоту', () => {
    expect(serverNow().getTime()).toBe(Date.now());
  });

  it('спешащие часы машины не влияют на показанное время', () => {
    // Часы машины убежали на два часа вперёд, сервер знает верное время.
    syncServerTime(ответСо('Fri, 04 Sep 2026 10:00:00 GMT'));

    expect(serverNow().toISOString()).toBe('2026-09-04T10:00:00.000Z');
    expect(isServerTimeSynced()).toBe(true);
  });

  it('отстающие часы машины тоже подтягиваются', () => {
    syncServerTime(ответСо('Fri, 04 Sep 2026 15:30:00 GMT'));
    expect(serverNow().toISOString()).toBe('2026-09-04T15:30:00.000Z');
  });

  it('ответ без заголовка и с мусором смещение не портит', () => {
    syncServerTime(ответСо('Fri, 04 Sep 2026 10:00:00 GMT'));
    const до = serverNow().getTime();

    syncServerTime({ headers: { get: () => null } });
    syncServerTime(ответСо('позавчера'));
    syncServerTime(undefined);

    expect(serverNow().getTime(), 'битый заголовок сдвинул часы').toBe(до);
  });

  it('часы показываются по Москве, а не по поясу машины', () => {
    // Машина в UTC, время 12:00 - в Москве это 15:00.
    const p = moscowParts(new Date('2026-09-04T12:00:00Z'));
    expect(`${p.hour}:${String(p.minute).padStart(2, '0')}`).toBe('15:00');
    expect(`${p.day}.${p.month}.${p.year}`).toBe('4.9.2026');
  });

  it('полночь по Москве это нулевой час, а не двадцать четвёртый', () => {
    // ru-RU отдаёт полночь как «24»; без приведения приветствие в шапке ушло бы
    // в ветку «Доброй ночи» по числу 24, которого в сутках нет.
    expect(moscowHour(new Date('2026-09-04T21:00:00Z'))).toBe(0);
  });

  it('дата пересекает сутки по московской границе', () => {
    // 21:30 UTC - это уже следующий день в Москве.
    expect(formatMoscowDateTime(new Date('2026-09-04T21:30:00Z'))).toBe('05.09.2026 00:30:00');
  });

  it('формат остаётся ДД.ММ.ГГГГ ЧЧ:ММ:СС с ведущими нулями', () => {
    expect(formatMoscowDateTime(new Date('2026-01-02T03:04:05Z'))).toBe('02.01.2026 06:04:05');
  });

  it('сверка и пояс работают вместе', () => {
    // Часы машины врут на час, а сама она в UTC: нужно и подтянуть, и показать по МСК.
    syncServerTime(ответСо('Fri, 04 Sep 2026 09:15:00 GMT'));
    expect(formatMoscowDateTime()).toBe('04.09.2026 12:15:00');
  });

  // Проверки выше зависят от зоны машины: на компьютере разработчика в Europe/Moscow
  // они пройдут даже с потерянным `timeZone` - совпадение с локальной зоной их не
  // отличит. На раннере CI (UTC) такая потеря роняет сразу пять из них, но замок
  // обязан ловить везде, поэтому зона стережётся ещё и по исходнику.
  it('зона задана явно, а не берётся у машины', () => {
    const src = readFileSync(resolve(__dirname, '..', 'serverTime.js'), 'utf8');

    expect(
      /timeZone:\s*MSK/.test(src) && /const MSK = 'Europe\/Moscow'/.test(src),
      'формат без timeZone показывает пояс машины; на компьютере в Москве это '
        + 'незаметно, а работник поста в другом поясе увидит своё время вместо общего',
    ).toBe(true);
  });
});
