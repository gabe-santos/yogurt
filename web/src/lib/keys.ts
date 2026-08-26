// The single keyboard-binding table this app has: every shortcut is a row
// here, and the `?` help dialog is a straight render of these rows, so a
// binding and its documentation cannot drift apart. The action id is the same
// single source for dispatch: a caller maps each id to a handler once, rather
// than re-listing the key literals in a second switch.

/** Action is what a binding does, named rather than the key that triggers it. */
export type Action =
  | 'next'
  | 'prev'
  | 'close'
  | 'toggleRead'
  | 'help'
  | 'search'
  | 'addFeed';

/** Binding is one keyboard shortcut: the key a reader presses, what it does. */
export interface Binding {
  /** key is what KeyboardEvent.key reports, e.g. "j", "Enter", "Escape". */
  key: string;
  description: string;
  action: Action;
}

export const bindings: Binding[] = [
  { key: 'j', description: 'Next Entry', action: 'next' },
  { key: 'k', description: 'Previous Entry', action: 'prev' },
  { key: 'Escape', description: 'Back to the Entry List', action: 'close' },
  { key: 'm', description: 'Toggle Read / unread', action: 'toggleRead' },
  { key: '/', description: 'Search', action: 'search' },
  { key: 'a', description: 'Add a Feed', action: 'addFeed' },
  { key: '?', description: 'Show this help', action: 'help' },
];

/** matches reports whether a KeyboardEvent is this binding's key, ignoring a
 * held modifier that would otherwise change what the key means (e.g. Cmd+R). */
export function matches(binding: Binding, event: KeyboardEvent): boolean {
  return (
    event.key === binding.key &&
    !event.metaKey &&
    !event.ctrlKey &&
    !event.altKey
  );
}
