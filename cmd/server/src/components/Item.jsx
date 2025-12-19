import { Switch, Match } from 'solid-js';
import { NoteItem } from './NoteItem';
import { LinkItem } from './LinkItem';

export function Item(props) {
  return (
    <Switch fallback={<div>Unknown item type: {props.item.type}</div>}>
      <Match when={props.item.type === 'note'}>
        <NoteItem {...props} />
      </Match>
      <Match when={props.item.type === 'bookmark'}>
        <LinkItem {...props} />
      </Match>
    </Switch>
  );
}
