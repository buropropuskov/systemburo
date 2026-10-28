import { STAGE_PADDING, STAGE_RADIUS } from './stageShape';

/** Shared visual/keyboard contract for learning and its recovery presenter. */
export function driverDefaults(reducedMotion) {
  return {
    animate: !reducedMotion,
    allowClose: true,
    allowKeyboardControl: false,
    overlayClickBehavior: () => {},
    overlayOpacity: 0.78,
    stagePadding: STAGE_PADDING,
    stageRadius: STAGE_RADIUS,
    popoverOffset: 16,
    popoverClass: 'ob-popover',
    nextBtnText: 'Далее',
    prevBtnText: 'Назад',
    doneBtnText: 'Готово',
  };
}
