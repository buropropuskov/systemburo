import { mount } from '@vue/test-utils';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import BaseModal from '../BaseModal.vue';
import { resetModalStack } from '@/utils/modalStack';

/**
 * Escape закрывает верхнее окно стопки, а не все открытые разом.
 *
 * Обработчик `keydown` висит у каждого окна на document, поэтому до этого замка один
 * Escape схлопывал и карточку участника, и список получателей под ней (поймано руками
 * на стенде, jsdom-тесты одиночного окна такое не видят).
 */
// Окно живёт, пока не размонтировано, и продолжает слушать document. Пока нажатие
// закрывало все слои разом, чужие окна кейсу не мешали; теперь слой забирает Escape
// себе, и окно из прошлого кейса перехватывало бы нажатие следующего.
const opened = [];

function openModal(zIndex) {
  const wrapper = mount(BaseModal, {
    props: { show: true, title: 'Окно', zIndex },
    global: { stubs: { teleport: true } },
  });
  opened.push(wrapper);
  return wrapper;
}

function pressEscape() {
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
}

describe('BaseModal: Escape в стопке окон', () => {
  beforeEach(() => resetModalStack());
  afterEach(() => opened.splice(0).forEach((w) => w.unmount()));

  it('закрывает окно с большим слоем, нижнее не трогает', () => {
    const bottom = openModal(12000);
    const top = openModal(12500);

    pressEscape();

    expect(top.emitted('close'), 'верхнее окно должно закрыться').toHaveLength(1);
    expect(bottom.emitted('close'), 'нижнее окно закрываться не должно').toBeUndefined();
  });

  it('при равных слоях закрывает открытое последним', () => {
    const first = openModal(1000);
    const second = openModal(1000);

    pressEscape();

    expect(second.emitted('close')).toHaveLength(1);
    expect(first.emitted('close')).toBeUndefined();
  });

  it('после закрытия верхнего Escape доходит до нижнего', async () => {
    const bottom = openModal(12000);
    const top = openModal(12500);

    pressEscape();
    top.unmount();
    pressEscape();

    expect(bottom.emitted('close'), 'освободившееся нижнее окно закрывается своим Escape').toHaveLength(1);
  });

  it('единственное окно закрывается как раньше', () => {
    const only = openModal(1000);

    pressEscape();

    expect(only.emitted('close')).toHaveLength(1);
  });
});
