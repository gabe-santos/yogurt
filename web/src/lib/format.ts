// Shared presentation formatting, so an Entry's published date reads the same
// wherever it appears.

const published = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'short',
});

const dayAndMonth = new Intl.DateTimeFormat(undefined, {
  month: 'short',
  day: 'numeric',
});

const dayMonthAndYear = new Intl.DateTimeFormat(undefined, {
  month: 'short',
  day: 'numeric',
  year: 'numeric',
});

/** formatPublished renders an Entry's published_at in full, for the one place
 * that has room for it: the Reading Pane's own header. */
export function formatPublished(publishedAt: string): string {
  return published.format(new Date(publishedAt));
}

const minute = 60_000;
const hour = 60 * minute;
const day = 24 * hour;

/** formatEntryAge renders an Entry's age for an Entry List row, where the
 * timestamp shares one 12px line with the Feed name and has to survive a
 * phone's width. A newest-first list is read by distance from now, so recent
 * Entries are relative and older ones fall back to a date; the full form
 * stays available as the row's tooltip. `now` is a parameter so the result is
 * a function of its inputs rather than of the clock. */
export function formatEntryAge(
  publishedAt: string,
  now: number = Date.now(),
): string {
  const at = new Date(publishedAt);
  const elapsed = now - at.getTime();
  // A publisher dating an Entry in the future is not worth a second vocabulary:
  // it reads as having just arrived, which is what it did.
  if (elapsed < minute) {
    return 'now';
  }
  if (elapsed < hour) {
    return `${Math.floor(elapsed / minute)}m`;
  }
  if (elapsed < day) {
    return `${Math.floor(elapsed / hour)}h`;
  }
  if (elapsed < 7 * day) {
    return `${Math.floor(elapsed / day)}d`;
  }
  return at.getFullYear() === new Date(now).getFullYear()
    ? dayAndMonth.format(at)
    : dayMonthAndYear.format(at);
}

/** feedMonogram is the fallback Feed Icon: the first letter of a Feed's
 * title, uppercased. "?" covers the pathological case of a blank title. */
export function feedMonogram(feedTitle: string): string {
  return feedTitle.trim().charAt(0).toUpperCase() || '?';
}

/** fromCodePoint decodes one numeric character reference, leaving anything
 * outside Unicode's range as the literal text it already was: an Excerpt comes
 * from a publisher, so a malformed reference must not throw where a row is
 * being rendered. */
function fromCodePoint(reference: string, code: number): string {
  return Number.isInteger(code) && code >= 0 && code <= 0x10ffff
    ? String.fromCodePoint(code)
    : reference;
}

/** entryExcerpt reduces an Entry's content — publisher-supplied HTML — to a
 * short plain-text Excerpt, so an Entry List row is scannable without
 * opening the Entry. The content can never be trusted as text, so this is
 * pure string work with no DOM: format.ts is imported during server-side
 * rendering, where there is no document to parse HTML with. This is an
 * Excerpt of the Entry's own body, distinct from SearchEntry.snippet, which
 * is a search-match fragment the server produces. */
export function entryExcerpt(content: string, limit = 180): string {
  const withoutScriptsAndStyles = content.replace(
    /<(script|style)\b[^>]*>[\s\S]*?<\/\1\s*>/gi,
    ' ',
  );
  const withoutTags = withoutScriptsAndStyles.replace(/<[^>]*>/g, ' ');
  // Undo the handful of named and numeric character references that survive
  // tag-stripping, so an Excerpt reads as plain text rather than leaking
  // markup escapes like "&amp;" into the Entry List. Named entities are
  // decoded before "&amp;" itself, so a double-encoded "&amp;lt;" resolves
  // to the literal text "&lt;" rather than "<".
  const decoded = withoutTags
    .replace(/&nbsp;/gi, ' ')
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
    .replace(/&quot;/gi, '"')
    .replace(/&#0*39;/g, "'")
    .replace(/&#x0*27;/gi, "'")
    .replace(/&#(\d+);/g, (match, dec: string) =>
      fromCodePoint(match, Number(dec)),
    )
    .replace(/&#x([0-9a-f]+);/gi, (match, hex: string) =>
      fromCodePoint(match, parseInt(hex, 16)),
    )
    .replace(/&amp;/gi, '&');
  const collapsed = decoded.replace(/\s+/g, ' ').trim();
  if (!collapsed) {
    return '';
  }
  if (collapsed.length <= limit) {
    return collapsed;
  }
  // Cut at the last word boundary at or before the limit, so a truncated
  // Excerpt never ends mid-word. Falling back to a hard cut covers the
  // pathological case of a single word longer than the whole limit.
  const withinLimit = collapsed.slice(0, limit);
  const lastSpace = withinLimit.lastIndexOf(' ');
  const truncated =
    lastSpace > 0 ? withinLimit.slice(0, lastSpace) : withinLimit;
  return `${truncated}…`;
}
