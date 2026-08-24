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

/** logIn starts a session, or throws ApiError. */
export async function logIn(password: string): Promise<void> {
	const response = await request('POST', '/session', { password });
	if (!response.ok) {
		throw new ApiError(response.status, await errorMessage(response, 'Could not sign in'));
	}
}

/** logOut ends the current session. */
export async function logOut(): Promise<void> {
	const response = await request('DELETE', '/session');
	if (!response.ok) {
		throw new ApiError(response.status, await errorMessage(response, 'Could not sign out'));
	}
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
