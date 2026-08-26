import { ref, onMounted, onBeforeUnmount } from 'vue';

/**
 * Реактивный признак «узкий экран» - тот же порог 768px, что и у мобильных `@media`.
 *
 * Нужен там, где мобильное поведение нельзя выразить одним CSS: показать подсказку,
 * которая на десктопе живёт на hover, свернуть список в «Ещё N», не переносить фокус
 * после выбора даты. Матчер держим вне реактивного состояния - слушателю реактивность
 * не нужна.
 *
 * @param {number} [maxWidth=768] порог в пикселях
 * @returns {{ isNarrow: import('vue').Ref<boolean> }}
 */
export function useNarrowScreen(maxWidth = 768) {
  const isNarrow = ref(false);
  let mql = null;
  let onChange = null;

  onMounted(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
    mql = window.matchMedia(`(max-width: ${maxWidth}px)`);
    isNarrow.value = mql.matches;
    onChange = (e) => { isNarrow.value = e.matches; };
    if (mql.addEventListener) mql.addEventListener('change', onChange);
    else if (mql.addListener) mql.addListener(onChange);
  });

  onBeforeUnmount(() => {
    if (!mql || !onChange) return;
    if (mql.removeEventListener) mql.removeEventListener('change', onChange);
    else if (mql.removeListener) mql.removeListener(onChange);
  });

  return { isNarrow };
}
