// Taskig service worker — network-first with offline fallback.
// We don't aggressively cache because all data comes from Google Tasks API.

const CACHE_NAME = 'taskig-shell-v1';
const SHELL_ASSETS = [
  '/static/css/dist.css',
  '/static/js/htmx.min.js',
  '/static/js/sortable.min.js',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(SHELL_ASSETS))
  );
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)))
    )
  );
  self.clients.claim();
});

self.addEventListener('fetch', (event) => {
  const { request } = event;

  // Only handle GET requests
  if (request.method !== 'GET') return;

  // Static assets: cache-first
  if (request.url.includes('/static/')) {
    event.respondWith(
      caches.match(request).then((cached) => cached || fetch(request))
    );
    return;
  }

  // Everything else: network-first (don't cache API/HTML responses)
  event.respondWith(
    fetch(request).catch(() => caches.match(request))
  );
});
