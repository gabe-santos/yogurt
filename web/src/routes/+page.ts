import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
  const { signedIn } = await parent();
  if (!signedIn) {
    redirect(307, '/login');
  }
};
