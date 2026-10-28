/** Responsive sizing only: Driver and the host keep ownership of position/target. */
export function touchPresentationLayout({ width, height, top = 0, left = 0, target, coarse = false }) {
  const touch = width < 768 || (coarse && width <= 1366);
  if (!touch) return { touch: false };
  const margin = width < 768 ? 16 : 24;
  const wide = width >= 900 && width > height;
  const cardWidth = Math.max(160, wide ? width - margin * 2 : Math.min(400, width - margin * 2));
  let available = height - margin * 2;
  if (target && target.width > 0 && target.height > 0) {
    const above = Math.max(0, target.top - top - margin - 16);
    const below = Math.max(0, top + height - target.bottom - margin - 16);
    const hasSide = target.left - left >= cardWidth + margin + 16 || left + width - target.right >= cardWidth + margin + 16;
    available = hasSide ? available : Math.min(available, Math.max(above, below));
  }
  return { touch: true, wide, gutter: margin, width: cardWidth, maxHeight: Math.max(120, available), fits: available >= 120 };
}
