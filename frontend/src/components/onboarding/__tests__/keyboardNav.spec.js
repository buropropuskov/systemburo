import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createKeyboardNav } from '../keyboardNav';

/**
 * Клавиши тура ведёт хост, потому что driver снимает свои обработчики на время
 * переезда подсветки и подъёма нового сегмента. Нажатие в это окно раньше
 * пропадало: человек жал стрелку, ничего не происходило, и он ждал, считая, что
 * тур завис. Замки стерегут ровно это - команда не теряется и не дублируется.
 */
describe('клавиатурная навигация тура', () => {
  let actions;
  let nav;

  beforeEach(() => {
    actions = {
      isActive: () => true,
      next: vi.fn(),
      prev: vi.fn(),
      close: vi.fn(),
    };
    nav = createKeyboardNav(actions);
    nav.attach();
  });

  afterEach(() => nav.detach());

  const press = (key, target = document.body) => {
    const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true });
    Object.defineProperty(event, 'target', { value: target });
    document.dispatchEvent(event);
    return event;
  };

  it('стрелки ведут тур вперёд и назад', () => {
    press('ArrowRight');
    press('ArrowLeft');
    expect(actions.next).toHaveBeenCalledTimes(1);
    expect(actions.prev).toHaveBeenCalledTimes(1);
  });

  it('нажатие во время подготовки шага не теряется - срабатывает по готовности', () => {
    nav.setBusy(true);
    press('ArrowRight');
    expect(actions.next).not.toHaveBeenCalled();

    nav.setBusy(false);
    expect(actions.next).toHaveBeenCalledTimes(1);
  });

  it('копится ровно одно нажатие - очередь из стрелок не пролистает полтура', () => {
    nav.setBusy(true);
    press('ArrowRight');
    press('ArrowRight');
    press('ArrowRight');
    nav.setBusy(false);
    expect(actions.next).toHaveBeenCalledTimes(1);
  });

  it('последнее направление побеждает: передумал на «Назад» - идём назад', () => {
    nav.setBusy(true);
    press('ArrowRight');
    press('ArrowLeft');
    nav.setBusy(false);
    expect(actions.next).not.toHaveBeenCalled();
    expect(actions.prev).toHaveBeenCalledTimes(1);
  });

  it('busyWhile снимает занятость даже при ошибке внутри', async () => {
    const failing = nav.busyWhile(() => Promise.reject(new Error('шаг не собрался')));
    await expect(failing).rejects.toThrow('шаг не собрался');
    press('ArrowRight');
    expect(actions.next).toHaveBeenCalledTimes(1);
  });

  it('Escape закрывает тур', () => {
    press('Escape');
    expect(actions.close).toHaveBeenCalledTimes(1);
  });

  it('в поле ввода стрелки принадлежат тексту, а не туру', () => {
    const input = document.createElement('input');
    document.body.appendChild(input);
    press('ArrowRight', input);
    expect(actions.next).not.toHaveBeenCalled();
    input.remove();
  });

  it('неактивный тур клавиши не слушает', () => {
    actions.isActive = () => false;
    press('ArrowRight');
    expect(actions.next).not.toHaveBeenCalled();
  });

  it('detach снимает обработчик и забывает отложенное', () => {
    nav.setBusy(true);
    press('ArrowRight');
    nav.detach();
    press('ArrowRight');
    nav.attach();
    nav.setBusy(false);
    expect(actions.next).not.toHaveBeenCalled();
  });
});
