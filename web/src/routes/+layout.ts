import { isSignedIn } from '$lib/api';
import type { LayoutLoad } from './$types';

// A static SPA, per ADR-0006: no server-side rendering, no framework server.
export const ssr = false;
export const prerender = false;

export const load: LayoutLoad = async () => {
  return { signedIn: await isSignedIn() };
};
