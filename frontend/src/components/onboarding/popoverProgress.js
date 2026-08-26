import { groupStepsBySection, chapterOf } from '@/components/onboarding/stepsFlow';

/**
 * Нижняя часть поповера: глава, счётчик шагов, полоса заполнения, подсказка о
 * следующем шаге и раскрывающийся список «перейти к шагу».
 *
 * Вынесено из движка отдельно - это разметка, а не логика тура: сюда приходят уже
 * посчитанные номера, а наружу уходит готовый узел.
 */

/**
 * Список шагов тура с переходом по клику. Тур длинный (у заявителя за сорок
 * шагов), и без него вернуться к нужному месту можно было только прокликав
 * всё заново.
 *
 * @param {number} currentGlobal глобальный индекс текущего шага
 * @param {(index: number) => void} onJump
 * @returns {HTMLElement}
 */
export function buildStepList({ steps, skipped, currentGlobal, onJump }) {
  const list = document.createElement('div');
  list.className = 'ob-popover__steps';
  list.setAttribute('data-testid', 'ob-step-list');

  groupStepsBySection(steps, skipped).forEach((group) => {
    const head = document.createElement('div');
    head.className = 'ob-popover__steps-group';
    head.textContent = group.title;
    list.appendChild(head);

    group.items.forEach((item) => {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'ob-popover__steps-item';
      if (item.index === currentGlobal) btn.classList.add('is-current');
      if (item.index < currentGlobal) btn.classList.add('is-passed');
      btn.textContent = item.title;
      btn.addEventListener('click', () => onJump(item.index));
      list.appendChild(btn);
    });
  });
  return list;
}

export function buildProgressBlock({ steps, skipped, globalIndex, total, nextTitle, currentGlobal, onJump }) {
  const block = document.createElement('div');
  block.className = 'ob-popover__progress';

  // Глава - ориентир в длинном туре: «Шаг 23 из 57» ничего не говорит о том,
  // сколько осталось до места, где удобно прерваться.
  const chapter = chapterOf(steps, currentGlobal);
  if (chapter) {
    const cap = document.createElement('div');
    cap.className = 'ob-popover__chapter';
    cap.setAttribute('data-testid', 'ob-chapter-label');
    cap.textContent = `Глава ${chapter.number} из ${chapter.total} · ${chapter.title}`;
    block.appendChild(cap);
  }

  const label = document.createElement('button');
  label.type = 'button';
  label.className = 'ob-popover__step-label';
  label.setAttribute('data-testid', 'ob-step-counter');
  label.textContent = `Шаг ${globalIndex} из ${total}`;
  if (onJump) {
    const list = buildStepList({ steps, skipped, currentGlobal, onJump });
    label.addEventListener('click', () => {
      const opening = !block.classList.contains('is-open');
      if (opening) {
        // Список - слой поверх карточки, а не её часть: иначе раскрытие
        // растит поповер, и driver не переставляет его - нижние пункты
        // оказываются за краем экрана. Сторону выбираем по свободному месту.
        const rect = block.getBoundingClientRect();
        const below = window.innerHeight - rect.bottom;
        const up = below < Math.min(rect.top, 260);
        block.classList.toggle('ob-popover__progress--up', up);
        list.style.maxHeight = `${Math.max(140, Math.min(260, (up ? rect.top : below) - 16))}px`;
        // Текущий шаг сразу в поле зрения - иначе в длинном туре список
        // открывается на первом разделе, где искать нечего.
        requestAnimationFrame(() => {
          list.querySelector('.is-current')?.scrollIntoView({ block: 'center' });
        });
      }
      block.classList.toggle('is-open', opening);
    });
    block.appendChild(list);
    label.title = 'Показать список шагов';
  } else {
    label.disabled = true;
  }

  const bar = document.createElement('div');
  bar.className = 'ob-popover__bar';
  const fill = document.createElement('div');
  fill.className = 'ob-popover__bar-fill';
  // Заполнение через scaleX (анимируем transform, не width - правило проекта).
  fill.style.transform = `scaleX(${total ? globalIndex / total : 0})`;
  bar.appendChild(fill);

  block.appendChild(label);
  block.appendChild(bar);

  if (nextTitle) {
    const hint = document.createElement('div');
    hint.className = 'ob-popover__next-hint';
    hint.textContent = `Далее: ${nextTitle}`;
    block.appendChild(hint);
  }

  return block;
}
