import { ref, onMounted, onBeforeUnmount } from 'vue';

/**
 * Реактивно отслеживает ориентацию viewport.
 * isPortrait: высота больше ширины (вертикальный монитор).
 * isCompact: портрет + узкая ширина (< 1100px) - переключает таблицу
 * в режим приоритезации столбцов и stacked-фильтров.
 *
 * @returns {{ isPortrait: import('vue').Ref<boolean>, isCompact: import('vue').Ref<boolean> }}
 */
export function useOrientation(compactWidth = 1100) {
  const isPortrait = ref(false);
  const isCompact = ref(false);

  let mqlPortrait = null;

  const hasMatchMedia = () =>
    typeof window !== 'undefined' && typeof window.matchMedia === 'function';

  const evaluate = () => {
    if (!hasMatchMedia()) return;
    isPortrait.value = window.matchMedia('(orientation: portrait)').matches;
    isCompact.value = isPortrait.value && window.innerWidth < compactWidth;
  };

  const onChange = () => evaluate();
  const onResize = () => evaluate();

  onMounted(() => {
    if (!hasMatchMedia()) return;
    mqlPortrait = window.matchMedia('(orientation: portrait)');
    if (mqlPortrait.addEventListener) {
      mqlPortrait.addEventListener('change', onChange);
    } else if (mqlPortrait.addListener) {
      mqlPortrait.addListener(onChange);
    }
    window.addEventListener('resize', onResize, { passive: true });
    evaluate();
  });

  onBeforeUnmount(() => {
    if (mqlPortrait) {
      if (mqlPortrait.removeEventListener) {
        mqlPortrait.removeEventListener('change', onChange);
      } else if (mqlPortrait.removeListener) {
        mqlPortrait.removeListener(onChange);
      }
    }
    if (typeof window !== 'undefined') {
      window.removeEventListener('resize', onResize);
    }
  });

  return { isPortrait, isCompact };
}
