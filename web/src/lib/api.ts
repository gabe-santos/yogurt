// The client's whole view of the server. Every call goes to the same origin,
// carries the session cookie, and speaks JSON.

/** ApiError is a response the server refused, carrying its status. */
export class ApiError extends Error {
	constructor(
		readonly status: number,
		message: string
	) {
		super(message);
		this.name = 'ApiError';
	}
}

/** Feed is one subscription. */
export interface Feed {
	id: number;
	url: string;
	title: string;
	site_url: string;
	created_at: string;
}

/** Entry is one item a Feed carried. */
export interface Entry {
	id: number;
	feed_id: number;
	feed_title: string;
	title: string;
	url: string;
	published_at: string;
	content: string;
	read: boolean;
}

/** EntryPage is one page of the reading list, newest first. */
export interface EntryPage {
	entries: Entry[];
	/** next_cursor is empty once the list is exhausted. */
	next_cursor: string;
}

/** Settings are the reader's own preferences. */
export interface Settings {
	/** mark_on_open is on by default: opening an Entry marks it Read. */
	mark_on_open: boolean;
}

async function request(method: string, path: string, body?: unknown): Promise<Response> {
	return fetch(`/api${path}`, {
		method,
		headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
		body: body === undefined ? undefined : JSON.stringify(body)
	});
}

async function errorMessage(response: Response, fallback: string): Promise<string> {
	try {
		const body = (await response.json()) as { error?: string };
		return body.error ?? fallback;
	} catch {
		return fallback;
	}
}

/** send makes a request and throws ApiError unless the server accepted it. */
async function send(
	method: string,
	path: string,
	fallback: string,
	body?: unknown
): Promise<Response> {
	const response = await request(method, path, body);
	if (!response.ok) {
		throw new ApiError(response.status, await errorMessage(response, fallback));
	}
	return response;
}

/** logIn starts a session, or throws ApiError. */
export async function logIn(password: string): Promise<void> {
	await send('POST', '/session', 'Could not sign in', { password });
}

/** logOut ends the current session. */
export async function logOut(): Promise<void> {
	await send('DELETE', '/session', 'Could not sign out');
}

/** isSignedIn reports whether the browser holds a live session. */
export async function isSignedIn(): Promise<boolean> {
	const response = await request('GET', '/session');
	if (response.status === 401) {
		return false;
	}
	if (!response.ok) {
		throw new ApiError(response.status, await errorMessage(response, 'Could not read session'));
	}
	return true;
}

/** listFeeds is the reader's whole collection, by title. */
export async function listFeeds(): Promise<Feed[]> {
	const response = await send('GET', '/feeds', 'Could not load your Feeds');
	const body = (await response.json()) as { feeds: Feed[] | null };
	return body.feeds ?? [];
}

/**
 * addFeed subscribes to a Feed. The address may be the Feed itself or a page
 * that advertises one; the server discovers which.
 */
export async function addFeed(url: string): Promise<Feed> {
	const response = await send('POST', '/feeds', 'Could not add that Feed', { url });
	const body = (await response.json()) as { feed: Feed };
	return body.feed;
}

/** refreshFeeds re-reads every Feed now, and reports the ones that failed. */
export async function refreshFeeds(): Promise<{ feed_id: number; error: string }[]> {
	const response = await send('POST', '/feeds/refresh', 'Could not refresh your Feeds');
	const body = (await response.json()) as { failures: { feed_id: number; error: string }[] | null };
	return body.failures ?? [];
}

/** listEntries reads one page of the reading list. */
export async function listEntries(
	options: { feed?: number; unread?: boolean; cursor?: string; limit?: number } = {}
): Promise<EntryPage> {
	const query = new URLSearchParams();
	if (options.feed !== undefined) {
		query.set('feed', String(options.feed));
	}
	if (options.unread !== undefined) {
		query.set('unread', String(options.unread));
	}
	if (options.cursor) {
		query.set('cursor', options.cursor);
	}
	if (options.limit !== undefined) {
		query.set('limit', String(options.limit));
	}

	const path = query.size > 0 ? `/entries?${query}` : '/entries';
	const response = await send('GET', path, 'Could not load your Entries');
	const body = (await response.json()) as { entries: Entry[] | null; next_cursor: string };
	return { entries: body.entries ?? [], next_cursor: body.next_cursor ?? '' };
}

/**
 * setEntryRead declares an Entry's Read state — an idempotent declaration, not
 * a toggle — and returns the Entry as stored.
 */
export async function setEntryRead(id: number, read: boolean): Promise<Entry> {
	const response = await send('PUT', `/entries/${id}/state`, 'Could not update that Entry', { read });
	const body = (await response.json()) as { entry: Entry };
	return body.entry;
}

/** getSettings reads the reader's preferences. */
export async function getSettings(): Promise<Settings> {
	const response = await send('GET', '/settings', 'Could not load your settings');
	const body = (await response.json()) as { settings: Settings };
	return body.settings;
}

/** setSettings declares the reader's preferences, replacing whatever they held. */
export async function setSettings(settings: Settings): Promise<Settings> {
	const response = await send('PUT', '/settings', 'Could not update your settings', settings);
	const body = (await response.json()) as { settings: Settings };
	return body.settings;
}
