// Settings shared by the Playwright config and the journeys it runs.
export const port = 4271;
export const password = 'e2e password';
export const baseURL = `http://127.0.0.1:${port}`;

// The fake publisher the binary subscribes to, standing in for a website.
export const publisherPort = 4272;
export const publisherURL = `http://localhost:${publisherPort}`;
