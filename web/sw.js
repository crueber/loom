/* Loom rebuild app shell cache. Versioned; old caches purged on activate. */
const CACHE = 'loom-shell-v4';
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
  // API reads: network-first, cache fallback offline (OSS-123). The old
  // cached||fresh order painted stale truth on first reload: revalidate()
  // applied the pre-mutation cached GET as state and never re-rendered
  // when the background fresh fetch landed. Network-first serves fresh
  // truth immediately; offline still falls back to the last good cache.
  // Mutations always hit network.
  if (url.pathname.startsWith('/api/')) {
    e.respondWith(
      caches.open(CACHE).then(async (cache) => {
        try {
          const res = await fetch(e.request);
          if (res.ok) cache.put(e.request, res.clone());
          return res;
        } catch (err) {
          const cached = await cache.match(e.request);
          if (cached) return cached;
          throw err;
        }
      })
    );
    return;
  }
  // Navigations: network-first so a deploy's versioned-asset page is
  // served on the very first load after it ships — a cached page could
  // otherwise pin stale unversioned refs for one more load. Offline
  // still falls back to the last cached page.
  if (e.request.mode === 'navigate') {
    e.respondWith(
      caches.open(CACHE).then(async (cache) => {
        try {
          const res = await fetch(e.request);
          if (res.ok) cache.put(e.request, res.clone());
          return res;
        } catch (err) {
          const cached = await cache.match(e.request);
          if (cached) return cached;
          throw err;
        }
      })
    );
    return;
  }
  // Shell: stale-while-revalidate. Deploy URLs carry ?v=<bytes hash>,
  // so each deploy is a cache miss that fills from network — staleness
  // is impossible while revalidations stay off the critical path.
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
