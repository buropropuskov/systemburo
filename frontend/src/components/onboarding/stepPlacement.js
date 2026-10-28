/** Side follows available layout, both in the config and the active Driver copy. */
export function getStepPlacement(step, narrow) {
  const placement = {};
  if (step.side) {
    placement.side = narrow && ['left', 'right'].includes(step.side) ? 'bottom' : step.side;
  }
  if (step.align) placement.align = step.align;
  return placement;
}

export function updateActiveStepPlacement(driver, sourceSteps, configSteps, narrow) {
  const index = driver.getActiveIndex() ?? 0;
  const source = sourceSteps[index], step = configSteps[index];
  if (!source || !step?.popover) return;
  const placement = getStepPlacement(source, narrow);
  Object.assign(step.popover, placement);
  const activeStep = driver.getActiveStep?.();
  if (activeStep?.popover) Object.assign(activeStep.popover, placement);
}
