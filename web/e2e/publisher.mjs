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

// The Articles the Feed's items link to. Both carry enough prose for
// extraction to recognise an Article, so Reader View and Original View can be
// told apart by what they render rather than by whether anything loaded.
const article = (title) => `<!doctype html>
<html>
  <head><title>${title}</title></head>
  <body>
    <nav>Site navigation the reader does not want</nav>
    <article>
      <h1>${title}</h1>
      <p>The publisher's own paragraph about ${title.toLowerCase()}, written at
      enough length that extraction recognises it as the body of an Article
      rather than the navigation chrome wrapped around it, and padded with
      further clauses to satisfy the same density heuristics a real page would
      have to satisfy.</p>
      <p>A second paragraph continues in the same vein, adding detail nobody
      asked for, so that the extracted text is unmistakably this page and not
      the summary the Feed carried, and so the parser has the length it wants
      before it will keep the block at all.</p>
    </article>
    <footer>Footer junk the reader does not want either</footer>
  </body>
</html>
`;

const documents = {
  '/feed.xml': { type: 'application/rss+xml; charset=utf-8', body: feed },
  '/': { type: 'text/html; charset=utf-8', body: page },
  // Fire allows being framed: Original View embeds it.
  '/fire': {
    type: 'text/html; charset=utf-8',
    body: article('Fire, and how to keep it'),
  },
  // Wheels refuses, the way a quarter of popular domains do: Original View has
  // to say so and offer a tab instead.
  '/wheels': {
    type: 'text/html; charset=utf-8',
    headers: { 'X-Frame-Options': 'DENY' },
    body: article('Wheels: a review'),
  },
};

createServer((request, response) => {
  const path = new URL(request.url, publisherURL).pathname;
  const document = documents[path];
  if (!document) {
    response.writeHead(404, { 'Content-Type': 'text/plain' }).end('not here\n');
    return;
  }
  response
    .writeHead(200, { 'Content-Type': document.type, ...document.headers })
    .end(document.body);
}).listen(publisherPort, '127.0.0.1');
