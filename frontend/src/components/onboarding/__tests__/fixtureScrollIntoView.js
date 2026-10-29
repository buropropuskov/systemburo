/** jsdom has no native scrolling; host tests place targets inside the viewport. */
export function installScrollIntoView() {
  const prototype = Element.prototype;
  const previous = Object.getOwnPropertyDescriptor(prototype, 'scrollIntoView');
  Object.defineProperty(prototype, 'scrollIntoView', { configurable: true, writable: true, value() {} });
  return () => {
    if (previous) Object.defineProperty(prototype, 'scrollIntoView', previous);
    else delete prototype.scrollIntoView;
  };
}
