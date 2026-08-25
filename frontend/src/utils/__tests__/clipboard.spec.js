import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { copyText } from '../clipboard';

/**
 * Копирование живёт в четырёх местах (список заявок, центр, модалка отправки,
 * лента журнала) и везде должно вести себя одинаково - в том числе на стенде без
 * TLS, где navigator.clipboard недоступен вовсе.
 */

const originalClipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard');

function setClipboard(value) {
  Object.defineProperty(navigator, 'clipboard', { value, configurable: true });
}

beforeEach(() => {
  document.execCommand = vi.fn(() => true);
});

afterEach(() => {
  if (originalClipboard) Object.defineProperty(navigator, 'clipboard', originalClipboard);
  else setClipboard(undefined);
});

describe('copyText', () => {
  it('кладёт текст через clipboard, когда он доступен', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    setClipboard({ writeText });

    await expect(copyText('№ 20260815/001')).resolves.toBe(true);
    expect(writeText).toHaveBeenCalledWith('№ 20260815/001');
  });

  it('число приводится к строке - номера приходят и числом', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    setClipboard({ writeText });

    await copyText(42);
    expect(writeText).toHaveBeenCalledWith('42');
  });

  it('без clipboard (http-стенд) копирует через скрытое поле', async () => {
    setClipboard(undefined);

    await expect(copyText('А123ВС')).resolves.toBe(true);
    expect(document.execCommand).toHaveBeenCalledWith('copy');
    // Поле не остаётся в документе после копирования.
    expect(document.querySelector('textarea')).toBeNull();
  });

  it('отказ буфера возвращает false, а не проглатывается', async () => {
    setClipboard({ writeText: vi.fn().mockRejectedValue(new Error('denied')) });

    await expect(copyText('А123ВС')).resolves.toBe(false);
  });

  it('пустое значение не трогает буфер', async () => {
    const writeText = vi.fn();
    setClipboard({ writeText });

    await expect(copyText('')).resolves.toBe(false);
    await expect(copyText(null)).resolves.toBe(false);
    await expect(copyText(undefined)).resolves.toBe(false);
    expect(writeText).not.toHaveBeenCalled();
  });
});
