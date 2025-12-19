import { createSignal, For, Show } from 'solid-js';

const PRESET_COLORS = [
  '#3D6D95', // Blue (Default)
  '#2E7D32', // Green
  '#C62828', // Red
  '#F57C00', // Orange
  '#6A1B9A', // Purple
  '#00838F', // Teal
  '#4E342E', // Brown
  '#37474F'  // Blue Grey
];

export function ColorPicker(props) {
  const customId = () => props.listId ? `color-custom-${props.listId}` : 'color-custom';
  return (
    <div class="config-color-section">
      <div class="config-color-label">Color</div>
      <div class="config-color-presets">
        <For each={PRESET_COLORS}>
          {(color) => (
            <button
              class="color-preset-btn"
              style={{ background: color, "border-color": props.value === color ? 'var(--pico-contrast)' : 'transparent' }}
              onClick={() => props.onChange(color)}
              title={color}
            />
          )}
        </For>
      </div>
      <div class="config-color-custom">
        <label for={customId()}>Custom:</label>
        <input
          id={customId()}
          type="color"
          class="color-picker-input"
          value={props.value}
          onInput={(e) => props.onChange(e.currentTarget.value)}
        />
      </div>
    </div>
  );
}
