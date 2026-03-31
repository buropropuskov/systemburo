import { ref, onMounted, onBeforeUnmount } from 'vue'

export function useDropdownState(containerSelector) {
  const isOpen = ref(false)

  function toggle() { isOpen.value = !isOpen.value }
  function close() { isOpen.value = false }

  function handleClickOutside(event) {
    if (containerSelector && !event.target.closest(containerSelector)) {
      close()
    }
  }

  onMounted(() => document.addEventListener('click', handleClickOutside))
  onBeforeUnmount(() => document.removeEventListener('click', handleClickOutside))

  return { isOpen, toggle, close }
}
