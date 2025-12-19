import { onMount, onCleanup } from 'solid-js';

export function useDragScroll(ref) {
  let isDown = false;
  let startX;
  let scrollLeft;

  const onMouseDown = (e) => {
    // Don't activate on input elements, buttons, or other interactive elements
    let element = e.target;
    while (element && element !== ref()) {
      const tagName = element.tagName.toUpperCase();
      if (['INPUT', 'BUTTON', 'A', 'TEXTAREA', 'SELECT', 'LABEL', 'OPTION'].includes(tagName)) {
        return;
      }
      element = element.parentElement;
    }

    // Only activate on container background or specific wrappers
    const isValidTarget = e.target === ref() ||
                         e.target.id === 'lists-container' ||
                         e.target.classList.contains('lists-wrapper') ||
                         e.target.classList.contains('bookmarks-container');

    if (!isValidTarget) return;

    isDown = true;
    ref().style.cursor = 'grabbing';
    ref().style.userSelect = 'none';
    startX = e.pageX - ref().offsetLeft;
    scrollLeft = ref().scrollLeft;
  };

  const onMouseLeave = () => {
    isDown = false;
    if (ref()) {
      ref().style.cursor = 'grab';
      ref().style.userSelect = '';
    }
  };

  const onMouseUp = () => {
    isDown = false;
    if (ref()) {
      ref().style.cursor = 'grab';
      ref().style.userSelect = '';
    }
  };

  const onMouseMove = (e) => {
    if (!isDown) return;

    const tagName = e.target.tagName.toUpperCase();
    if (!['INPUT', 'BUTTON', 'A', 'TEXTAREA', 'SELECT', 'OPTION'].includes(tagName)) {
      e.preventDefault();
    }

    const x = e.pageX - ref().offsetLeft;
    const walk = (x - startX) * 2;
    ref().scrollLeft = scrollLeft - walk;
  };

  onMount(() => {
    const el = ref();
    if (el) {
      el.style.cursor = 'grab';
      el.addEventListener('mousedown', onMouseDown);
      el.addEventListener('mouseleave', onMouseLeave);
      el.addEventListener('mouseup', onMouseUp);
      el.addEventListener('mousemove', onMouseMove);
    }
  });

  onCleanup(() => {
    const el = ref();
    if (el) {
      el.removeEventListener('mousedown', onMouseDown);
      el.removeEventListener('mouseleave', onMouseLeave);
      el.removeEventListener('mouseup', onMouseUp);
      el.removeEventListener('mousemove', onMouseMove);
    }
  });
}
