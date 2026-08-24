// The fake publisher for the browser seam: one site page advertising one Feed,
// served over real HTTP so that the real binary fetches it the way it fetches
// anything else. Node's own http server, so the e2e run needs no extra
// dependency.
import { createServer } from 'node:http';

const publisherPort = Number(process.env.PUBLISHER_PORT);
const publisherURL = `http://127.0.0.1:${publisherPort}`;

const feed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>The Daily Cave</title>
    <link>${publisherURL}/</link>
    <item>
      <title>Fire, and how to keep it</title>
      <link>${publisherURL}/fire</link>
      <guid isPermaLink="false">fire</guid>
      <pubDate>Thu, 01 Jan 2026 10:00:00 +0000</pubDate>
      <description>Keeping a fire alive overnight.</description>
    </item>
    <item>
      <title>Wheels: a review</title>
      <link>${publisherURL}/wheels</link>
      <guid isPermaLink="false">wheels</guid>
      <pubDate>Fri, 02 Jan 2026 10:00:00 +0000</pubDate>
      <description>Round, and it rolls.</description>
    </item>
  </channel>
</rss>
`;

const page = `<!doctype html>
<html>
  <head>
    <title>The Daily Cave</title>
    <link rel="alternate" type="application/rss+xml" href="/feed.xml">
  </head>
  <body><p>A page, not a Feed.</p></body>
</html>
`;

const documents = {
  '/feed.xml': { type: 'application/rss+xml; charset=utf-8', body: feed },
  '/': { type: 'text/html; charset=utf-8', body: page },
};

createServer((request, response) => {
  const path = new URL(request.url, publisherURL).pathname;
  const document = documents[path];
  if (!document) {
    response.writeHead(404, { 'Content-Type': 'text/plain' }).end('not here\n');
    return;
  }
  response.writeHead(200, { 'Content-Type': document.type }).end(document.body);
}).listen(publisherPort, '127.0.0.1');
