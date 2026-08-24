// Shared presentation formatting, so an Entry's published date reads the same
// wherever it appears.

const published = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

/** formatPublished renders an Entry's published_at for display. */
export function formatPublished(publishedAt: string): string {
	return published.format(new Date(publishedAt));
}
