export function ItemHeader(props) {
  return (
    <div class="item-config-header">
      <h5>{props.title}</h5>
      <button class="item-config-close-btn" onClick={props.onClose}>×</button>
    </div>
  );
}
