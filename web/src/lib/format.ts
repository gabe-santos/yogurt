// Shared presentation formatting, so an Entry's published date reads the same
// wherever it appears.

const published = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'short',
});

/** formatPublished renders an Entry's published_at for display. */
export function formatPublished(publishedAt: string): string {
  return published.format(new Date(publishedAt));
}

/** feedMonogram is the fallback Feed Icon: the first letter of a Feed's
 * title, uppercased. "?" covers the pathological case of a blank title. */
export function feedMonogram(feedTitle: string): string {
  return feedTitle.trim().charAt(0).toUpperCase() || '?';
}
