// The client's whole view of the server. Every call goes to the same origin,
// carries the session cookie, and speaks JSON.

/** ApiError is a response the server refused, carrying its status. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

/** Feed is one subscription. last_checked_at and last_success_at are null
 * until the Feed's first check, so silence before any check is
 * distinguishable from a Feed that keeps failing. icon_stored_at is null
 * until the Feed has a Feed Icon, and doubles as a cache-busting version for
 * feedIconUrl. icon_checked_at is null until the Feed's first icon check. */
export interface Feed {
  id: number;
  url: string;
  title: string;
  site_url: string;
  group_id: number;
  unread_count: number;
  created_at: string;
  last_checked_at: string | null;
  last_success_at: string | null;
  last_error: string;
  consecutive_failures: number;
  icon_stored_at: string | null;
  icon_checked_at: string | null;
}

/** feedIconUrl is where a Feed's stored Feed Icon is served from, versioned
 * by icon_stored_at so a changed icon invalidates the browser's cache and an
 * unchanged one is never refetched. Absent when the Feed has none. */
export function feedIconUrl(feed: Feed): string | undefined {
  if (!feed.icon_stored_at) return undefined;
  return `/api/feeds/${feed.id}/icon?v=${encodeURIComponent(feed.icon_stored_at)}`;
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
  starred: boolean;
  archived: boolean;
}

/** EntryPage is one page of the publish-date ordered reading list. */
export interface EntryPage {
  entries: Entry[];
  /** next_cursor is empty once the list is exhausted. */
  next_cursor: string;
}

/** SearchEntry is one Entry a search matched, with a short excerpt of where
 * it matched. */
export interface SearchEntry extends Entry {
  snippet: string;
}

/** SearchResults is what one search returns: matching Entries and matching
 * Feeds, so a search doubles as navigation to a Feed. */
export interface SearchResults {
  entries: SearchEntry[];
  feeds: Feed[];
}

/** Article is a publisher's page, reduced to its main text: Reader View. */
export interface Article {
  title: string;
  html: string;
  /** embeddable reports whether the publisher's page permits Original View
   * to embed it. */
  embeddable: boolean;
}

/** Original is the publisher's own page for an Entry: what Original View
 * embeds, and whether the publisher allows it to be embedded at all. */
export interface Original {
  url: string;
  /** embeddable is false when the publisher's framing headers refuse it, and
   * the reader is offered a new tab instead of a blank frame. */
  embeddable: boolean;
}

/** EntryView is which view an Entry opens in: the text the Feed itself
 * carried, the Article reduced to its main text, or the publisher's own page
 * embedded. */
export type EntryView = 'feed' | 'reader' | 'original';

/** Settings are the reader's own preferences. */
export interface Settings {
  /** mark_on_open is on by default: opening an Entry marks it Read. */
  mark_on_open: boolean;
  /** entry_view is the view an Entry opens in, remembered across Entries. */
  entry_view: EntryView;
}

async function request(
  method: string,
  path: string,
  body?: unknown,
): Promise<Response> {
  return fetch(`/api${path}`, {
    method,
    headers:
      body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

async function errorMessage(
  response: Response,
  fallback: string,
): Promise<string> {
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
  body?: unknown,
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
    throw new ApiError(
      response.status,
      await errorMessage(response, 'Could not read session'),
    );
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
  const response = await send('POST', '/feeds', 'Could not add that Feed', {
    url,
  });
  const body = (await response.json()) as { feed: Feed };
  return body.feed;
}

/**
 * updateFeed changes a Feed's title. Only the fields given are changed, and
 * returns the Feed as stored.
 */
export async function updateFeed(
  id: number,
  changes: { title?: string },
): Promise<Feed> {
  const response = await send(
    'PUT',
    `/feeds/${id}`,
    'Could not update that Feed',
    changes,
  );
  const body = (await response.json()) as { feed: Feed };
  return body.feed;
}

/** deleteFeed removes a Feed and every Entry it carried. */
export async function deleteFeed(id: number): Promise<void> {
  await send('DELETE', `/feeds/${id}`, 'Could not delete that Feed');
}

/** refreshFeeds re-reads every Feed now, and reports the ones that failed. */
export async function refreshFeeds(): Promise<
  { feed_id: number; error: string }[]
> {
  const response = await send(
    'POST',
    '/feeds/refresh',
    'Could not refresh your Feeds',
  );
  const body = (await response.json()) as {
    failures: { feed_id: number; error: string }[] | null;
  };
  return body.failures ?? [];
}

export interface EntrySelectionOptions {
  feed?: number;
  unread?: boolean;
  starred?: boolean;
  archived?: boolean;
}

export type EntryOrder = 'newest' | 'oldest';

function entrySelectionQuery(options: EntrySelectionOptions): URLSearchParams {
  const query = new URLSearchParams();
  for (const key of [
    'feed',
    'unread',
    'starred',
    'archived',
  ] as const) {
    const value = options[key];
    if (value !== undefined) {
      query.set(key, String(value));
    }
  }
  return query;
}

/** listEntries reads one page of the reading list. around anchors the page
 * at that Entry id, inclusive, instead of the top of the list — how a
 * search result opens within its ordinary list rather than a standalone
 * view. */
export async function listEntries(
  options: EntrySelectionOptions & {
    cursor?: string;
    limit?: number;
    around?: number;
    order?: EntryOrder;
  } = {},
): Promise<EntryPage> {
  const query = entrySelectionQuery(options);
  if (options.cursor) {
    query.set('cursor', options.cursor);
  }
  if (options.limit !== undefined) {
    query.set('limit', String(options.limit));
  }
  if (options.around !== undefined) {
    query.set('around', String(options.around));
  }
  if (options.order !== undefined) {
    query.set('order', options.order);
  }

  const path = query.size > 0 ? `/entries?${query}` : '/entries';
  const response = await send('GET', path, 'Could not load your Entries');
  const body = (await response.json()) as {
    entries: Entry[] | null;
    next_cursor: string;
  };
  return { entries: body.entries ?? [], next_cursor: body.next_cursor ?? '' };
}

/** search finds Entries by title or Feed-supplied content, and Feeds by
 * name, so a search doubles as navigation. Blank query matches nothing. */
export async function search(query: string): Promise<SearchResults> {
  const path = `/search?q=${encodeURIComponent(query)}`;
  const response = await send('GET', path, 'Could not search your Entries');
  const body = (await response.json()) as {
    entries: SearchEntry[] | null;
    feeds: Feed[] | null;
  };
  return { entries: body.entries ?? [], feeds: body.feeds ?? [] };
}

/** setEntryState declares complete reader-owned state, never a toggle. */
export async function setEntryState(
  id: number,
  state: Pick<Entry, 'read' | 'starred' | 'archived'>,
): Promise<Entry> {
  const response = await send(
    'PUT',
    `/entries/${id}/state`,
    'Could not update that Entry',
    state,
  );
  const body = (await response.json()) as { entry: Entry };
  return body.entry;
}

/**
 * getArticle fetches Reader View for an Entry: the publisher's page, reduced
 * to its main text. Extraction happens on the server the first time this is
 * asked for an Entry's URL, and is stored and reused after that.
 */
export async function getArticle(entryId: number): Promise<Article> {
  const response = await send(
    'GET',
    `/entries/${entryId}/article`,
    'Could not extract that Article',
  );
  const body = (await response.json()) as { article: Article };
  return body.article;
}

/**
 * getOriginal fetches what Original View needs for an Entry: the publisher's
 * own address, and whether the publisher permits it to be embedded. The flag
 * is the one the server recorded when it fetched the page, so the frame is
 * only ever pointed at a page that will actually load in one.
 */
export async function getOriginal(entryId: number): Promise<Original> {
  const response = await send(
    'GET',
    `/entries/${entryId}/original`,
    'Could not reach that publisher',
  );
  const body = (await response.json()) as { original: Original };
  return body.original;
}

/** markEntriesRead declares Read for exactly one filter and scope. */
export async function markEntriesRead(
  options: EntrySelectionOptions,
): Promise<void> {
  const query = entrySelectionQuery(options);
  const path = query.size > 0 ? `/entries/state?${query}` : '/entries/state';
  await send('PUT', path, 'Could not mark those Entries Read', { read: true });
}

/** getSettings reads the reader's preferences. */
export async function getSettings(): Promise<Settings> {
  const response = await send(
    'GET',
    '/settings',
    'Could not load your settings',
  );
  const body = (await response.json()) as { settings: Settings };
  return body.settings;
}

/** setSettings declares the reader's preferences, replacing whatever they held. */
export async function setSettings(settings: Settings): Promise<Settings> {
  const response = await send(
    'PUT',
    '/settings',
    'Could not update your settings',
    settings,
  );
  const body = (await response.json()) as { settings: Settings };
  return body.settings;
}

/** DeviceToken is a named, hashed-at-rest credential for a non-browser
 * client. last_used_at is null until the token first authenticates a
 * request. */
export interface DeviceToken {
  id: number;
  name: string;
  created_at: string;
  last_used_at: string | null;
}

/** listDeviceTokens is the reader's whole set of device tokens. */
export async function listDeviceTokens(): Promise<DeviceToken[]> {
  const response = await send(
    'GET',
    '/device-tokens',
    'Could not load your device tokens',
  );
  const body = (await response.json()) as {
    device_tokens: DeviceToken[] | null;
  };
  return body.device_tokens ?? [];
}

/** createDeviceToken issues a new device token. Its raw value is returned
 * only once, in this response, and cannot be recovered again afterwards. */
export async function createDeviceToken(
  name: string,
): Promise<{ device_token: DeviceToken; token: string }> {
  const response = await send(
    'POST',
    '/device-tokens',
    'Could not create that device token',
    { name },
  );
  return (await response.json()) as {
    device_token: DeviceToken;
    token: string;
  };
}

/** revokeDeviceToken ends a device token immediately. */
export async function revokeDeviceToken(id: number): Promise<void> {
  await send(
    'DELETE',
    `/device-tokens/${id}`,
    'Could not revoke that device token',
  );
}
