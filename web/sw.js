/* Loom rebuild app shell cache. Versioned; old caches purged on activate. */
const CACHE = 'loom-shell-v2';
const SHELL = ['/', '/styles.css', '/app.js', '/manifest.json'];
self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
});
self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys().then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});
self.addEventListener('fetch', (e) => {
  const url = new URL(e.request.url);
  if (e.request.method !== 'GET' || url.origin !== location.origin) return;
  // API reads: stale-while-revalidate. Mutations always hit network.
  if (url.pathname.startsWith('/api/')) {
    e.respondWith(
      caches.open(CACHE).then(async (cache) => {
        const cached = await cache.match(e.request);
        const fresh = fetch(e.request).then((res) => {
          if (res.ok) cache.put(e.request, res.clone());
          return res;
        }).catch(() => cached);
        return cached || fresh;
      })
    );
    return;
  }
  // Shell: stale-while-revalidate so deploys self-propagate without an
  // SW byte-change gate. Images: cache-first with LRU cap (below).
  if (SHELL.includes(url.pathname)) {
    e.respondWith(
      caches.open(CACHE).then(async (cache) => {
        const cached = await cache.match(e.request);
        const fresh = fetch(e.request).then((res) => {
          if (res.ok) cache.put(e.request, res.clone());
          return res;
        }).catch(() => cached);
        return cached || fresh;
      })
    );
    return;
  }
  // Images + other same-origin GETs: cache-first, network fills in behind.
  // Image LRU cap: keep at most ~300 image responses (~50-100MB at
  // thumbnail sizes) so the cache never grows unbounded. Eviction is
  // oldest-first; misses refetch from network. Never blocks first paint.
  e.respondWith(
    caches.match(e.request).then((hit) => hit || fetch(e.request).then((res) => {
      if (res.ok) {
        const copy = res.clone();
        caches.open(CACHE).then((c) => {
          c.put(e.request, copy).then(() => {
            if (url.pathname.startsWith('/images/')) trimImages(c);
          });
        });
      }
      return res;
    }))
  );
});

const MAX_IMAGES = 300;
async function trimImages(cache) {
  const keys = await cache.keys();
  const imgs = keys.filter((r) => new URL(r.url).pathname.startsWith('/images/'));
  for (let i = 0; i + MAX_IMAGES < imgs.length; i++) {
    await cache.delete(imgs[i]);
  }
}
