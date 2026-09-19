/**
 * Service Worker for Social Protection Platform PWA
 * Provides offline-first capabilities with background sync
 */

const CACHE_NAME = 'sp-platform-v1';
const STATIC_CACHE = 'sp-static-v1';
const API_CACHE = 'sp-api-v1';
const OFFLINE_QUEUE = 'sp-offline-queue';

// Static assets to cache on install
const STATIC_ASSETS = [
  '/',
  '/index.html',
  '/manifest.json',
  '/offline.html',
];

// API endpoints to cache
const CACHEABLE_API_PATTERNS = [
  /\/api\/beneficiaries/,
  /\/api\/programs/,
  /\/api\/households/,
  /\/api\/disbursements/,
  /\/api\/journeys/,
  /\/api\/journey-contracts/,
  /\/api\/offline/,
  /\/api\/federation/,
  /\/api\/interop/,
  /\/api\/grievances/,
];

// Journey-specific cache for workflow data
const JOURNEY_CACHE = 'sp-journeys-v1';

// Journey definitions for offline support
const JOURNEY_CATEGORIES = [
  'enrollment',
  'payments',
  'grievance',
  'lifecycle',
  'admin',
  'reporting',
  'fraud',
];

// Install event - cache static assets
self.addEventListener('install', (event) => {
  console.log('[SW] Installing service worker...');
  event.waitUntil(
    caches.open(STATIC_CACHE)
      .then((cache) => {
        console.log('[SW] Caching static assets');
        return cache.addAll(STATIC_ASSETS);
      })
      .then(() => self.skipWaiting())
  );
});

// Activate event - clean up old caches
self.addEventListener('activate', (event) => {
  console.log('[SW] Activating service worker...');
  event.waitUntil(
    caches.keys()
      .then((cacheNames) => {
        return Promise.all(
          cacheNames
            .filter((name) => name !== STATIC_CACHE && name !== API_CACHE)
            .map((name) => caches.delete(name))
        );
      })
      .then(() => self.clients.claim())
  );
});

// Fetch event - network first with cache fallback for API, cache first for static
self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Skip non-GET requests for caching (but queue them for offline)
  // Note: In service workers, we can't reliably use navigator.onLine
  // Instead, we try the fetch and queue on network failure
  if (request.method !== 'GET') {
    event.respondWith(
      fetch(request.clone()).catch(() => queueOfflineRequest(request))
    );
    return;
  }

  // API requests - network first, cache fallback
  if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/trpc/')) {
    event.respondWith(networkFirstStrategy(request));
    return;
  }

  // Static assets - cache first, network fallback
  event.respondWith(cacheFirstStrategy(request));
});

// Network first strategy for API calls
async function networkFirstStrategy(request) {
  try {
    const response = await fetch(request);
    
    // Cache successful GET responses
    if (response.ok && isCacheableAPI(request.url)) {
      const cache = await caches.open(API_CACHE);
      cache.put(request, response.clone());
    }
    
    return response;
  } catch (error) {
    console.log('[SW] Network failed, trying cache:', request.url);
    const cachedResponse = await caches.match(request);
    
    if (cachedResponse) {
      return cachedResponse;
    }
    
    // Return offline response for API calls
    return new Response(
      JSON.stringify({ 
        error: 'offline', 
        message: 'You are offline. Data will sync when connection is restored.',
        cached: false 
      }),
      { 
        status: 503, 
        headers: { 'Content-Type': 'application/json' } 
      }
    );
  }
}

// Cache first strategy for static assets
async function cacheFirstStrategy(request) {
  const cachedResponse = await caches.match(request);
  
  if (cachedResponse) {
    // Refresh cache in background
    fetch(request).then((response) => {
      if (response.ok) {
        caches.open(STATIC_CACHE).then((cache) => {
          cache.put(request, response);
        });
      }
    }).catch(() => {});
    
    return cachedResponse;
  }
  
  try {
    const response = await fetch(request);
    
    if (response.ok) {
      const cache = await caches.open(STATIC_CACHE);
      cache.put(request, response.clone());
    }
    
    return response;
  } catch (error) {
    // Return offline page for navigation requests
    if (request.mode === 'navigate') {
      return caches.match('/offline.html');
    }
    throw error;
  }
}

// Check if API endpoint should be cached
function isCacheableAPI(url) {
  return CACHEABLE_API_PATTERNS.some((pattern) => pattern.test(url));
}

// Queue offline requests for later sync
// Generate UUID for idempotency keys
function generateUUID() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0;
    const v = c === 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

async function queueOfflineRequest(request) {
  const queue = await getOfflineQueue();
  
  // Generate idempotency key for this request to prevent duplicates on retry
  const idempotencyKey = generateUUID();
  const headers = Object.fromEntries(request.headers.entries());
  headers['X-Idempotency-Key'] = idempotencyKey;
  
  const requestData = {
    id: Date.now().toString(),
    idempotencyKey: idempotencyKey,
    url: request.url,
    method: request.method,
    headers: headers,
    body: await request.text(),
    timestamp: new Date().toISOString(),
  };
  
  queue.push(requestData);
  await saveOfflineQueue(queue);
  
  // Notify clients about queued request
  self.clients.matchAll().then((clients) => {
    clients.forEach((client) => {
      client.postMessage({
        type: 'OFFLINE_REQUEST_QUEUED',
        data: requestData,
      });
    });
  });
  
  return new Response(
    JSON.stringify({ 
      queued: true, 
      message: 'Request queued for sync when online',
      queueId: requestData.id 
    }),
    { 
      status: 202, 
      headers: { 'Content-Type': 'application/json' } 
    }
  );
}

// Background sync event
self.addEventListener('sync', (event) => {
  console.log('[SW] Background sync triggered:', event.tag);
  
  if (event.tag === 'offline-sync') {
    event.waitUntil(
      Promise.all([
        syncOfflineRequests(),
        syncQueuedJourneys(),
      ])
    );
  }
  
  // Also support journey-specific sync tag
  if (event.tag === 'journey-sync') {
    event.waitUntil(syncQueuedJourneys());
  }
});

// Sync queued offline requests
async function syncOfflineRequests() {
  const queue = await getOfflineQueue();
  
  if (queue.length === 0) {
    console.log('[SW] No offline requests to sync');
    return;
  }
  
  console.log('[SW] Syncing', queue.length, 'offline requests');
  
  const results = [];
  const failedRequests = [];
  const authBlockedRequests = [];
  
  for (const requestData of queue) {
    try {
      const response = await fetch(requestData.url, {
        method: requestData.method,
        headers: requestData.headers,
        body: requestData.body || undefined,
      });
      
      // Handle auth failures - keep in queue but mark as blocked
      if (response.status === 401 || response.status === 403) {
        console.log('[SW] Auth blocked request:', requestData.id);
        authBlockedRequests.push({ ...requestData, authBlocked: true });
        results.push({
          id: requestData.id,
          success: false,
          status: response.status,
          authBlocked: true,
        });
        continue;
      }
      
      results.push({
        id: requestData.id,
        success: response.ok,
        status: response.status,
      });
      
      if (!response.ok) {
        failedRequests.push(requestData);
      }
    } catch (error) {
      console.error('[SW] Failed to sync request:', requestData.id, error);
      failedRequests.push(requestData);
      results.push({
        id: requestData.id,
        success: false,
        error: error.message,
      });
    }
  }
  
  // Save failed and auth-blocked requests back to queue
  await saveOfflineQueue([...failedRequests, ...authBlockedRequests]);
  
  // Notify clients about sync results
  const syncedCount = queue.length - failedRequests.length - authBlockedRequests.length;
  self.clients.matchAll().then((clients) => {
    clients.forEach((client) => {
      client.postMessage({
        type: 'OFFLINE_SYNC_COMPLETE',
        data: {
          total: queue.length,
          synced: syncedCount,
          failed: failedRequests.length,
          authBlocked: authBlockedRequests.length,
          results,
        },
      });
      
      // Send separate auth required message if any requests are blocked
      if (authBlockedRequests.length > 0) {
        client.postMessage({
          type: 'AUTH_REQUIRED_FOR_SYNC',
          data: {
            blockedCount: authBlockedRequests.length,
            message: 'Sign in to sync your offline changes',
          },
        });
      }
    });
  });
}

// IndexedDB helpers for offline queue
async function getOfflineQueue() {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('sp-offline-db', 1);
    
    request.onerror = () => reject(request.error);
    
    request.onupgradeneeded = (event) => {
      const db = event.target.result;
      if (!db.objectStoreNames.contains('queue')) {
        db.createObjectStore('queue', { keyPath: 'id' });
      }
    };
    
    request.onsuccess = () => {
      const db = request.result;
      const tx = db.transaction('queue', 'readonly');
      const store = tx.objectStore('queue');
      const getAllRequest = store.getAll();
      
      getAllRequest.onsuccess = () => resolve(getAllRequest.result || []);
      getAllRequest.onerror = () => reject(getAllRequest.error);
    };
  });
}

async function saveOfflineQueue(queue) {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('sp-offline-db', 1);
    
    request.onerror = () => reject(request.error);
    
    request.onsuccess = () => {
      const db = request.result;
      const tx = db.transaction('queue', 'readwrite');
      const store = tx.objectStore('queue');
      
      // Clear existing queue
      store.clear();
      
      // Add new items
      queue.forEach((item) => store.add(item));
      
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
    };
  });
}

// Push notification event
self.addEventListener('push', (event) => {
  console.log('[SW] Push notification received');
  
  let data = { title: 'Social Protection Platform', body: 'New update available' };
  
  if (event.data) {
    try {
      data = event.data.json();
    } catch (e) {
      data.body = event.data.text();
    }
  }
  
  const options = {
    body: data.body,
    icon: '/icons/icon-192x192.png',
    badge: '/icons/badge-72x72.png',
    vibrate: [100, 50, 100],
    data: data.data || {},
    actions: data.actions || [],
  };
  
  event.waitUntil(
    self.registration.showNotification(data.title, options)
  );
});

// Notification click event
self.addEventListener('notificationclick', (event) => {
  console.log('[SW] Notification clicked:', event.action);
  
  event.notification.close();
  
  const urlToOpen = event.notification.data?.url || '/';
  
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      .then((clientList) => {
        // Focus existing window if available
        for (const client of clientList) {
          if (client.url === urlToOpen && 'focus' in client) {
            return client.focus();
          }
        }
        // Open new window
        if (self.clients.openWindow) {
          return self.clients.openWindow(urlToOpen);
        }
      })
  );
});

// Message event for communication with main thread
self.addEventListener('message', (event) => {
  console.log('[SW] Message received:', event.data);
  
  switch (event.data.type) {
    case 'SKIP_WAITING':
      self.skipWaiting();
      break;
      
    case 'GET_OFFLINE_QUEUE':
      getOfflineQueue().then((queue) => {
        event.ports[0].postMessage({ queue });
      });
      break;
      
    case 'CLEAR_OFFLINE_QUEUE':
      saveOfflineQueue([]).then(() => {
        event.ports[0].postMessage({ success: true });
      });
      break;
      
    case 'TRIGGER_SYNC':
      syncOfflineRequests().then(() => {
        event.ports[0].postMessage({ success: true });
      });
      break;
      
    case 'CACHE_URLS':
      caches.open(STATIC_CACHE).then((cache) => {
        return cache.addAll(event.data.urls);
      }).then(() => {
        event.ports[0].postMessage({ success: true });
      });
      break;
      
    // Journey-specific message handlers
    case 'CACHE_JOURNEY_CONTRACTS':
      cacheJourneyContracts().then(() => {
        event.ports[0].postMessage({ success: true });
      });
      break;
      
    case 'GET_CACHED_JOURNEYS':
      getCachedJourneys().then((journeys) => {
        event.ports[0].postMessage({ journeys });
      });
      break;
      
    case 'QUEUE_JOURNEY_START':
      queueJourneyStart(event.data.journeyKey, event.data.input).then((result) => {
        event.ports[0].postMessage(result);
      });
      break;
      
    case 'GET_JOURNEY_QUEUE':
      getJourneyQueue().then((queue) => {
        event.ports[0].postMessage({ queue });
      });
      break;
      
    case 'SYNC_JOURNEYS':
      syncQueuedJourneys().then((result) => {
        event.ports[0].postMessage(result);
      });
      break;
  }
});

// Cache journey contracts for offline access
async function cacheJourneyContracts() {
  try {
    const response = await fetch('/api/journey-contracts');
    if (response.ok) {
      const cache = await caches.open(JOURNEY_CACHE);
      await cache.put('/api/journey-contracts', response.clone());
      console.log('[SW] Journey contracts cached');
    }
  } catch (error) {
    console.error('[SW] Failed to cache journey contracts:', error);
  }
}

// Get cached journeys
async function getCachedJourneys() {
  try {
    const cache = await caches.open(JOURNEY_CACHE);
    const response = await cache.match('/api/journey-contracts');
    if (response) {
      return response.json();
    }
  } catch (error) {
    console.error('[SW] Failed to get cached journeys:', error);
  }
  return null;
}

// Queue a journey start request for offline execution
async function queueJourneyStart(journeyKey, input) {
  const queue = await getJourneyQueue();
  
  const queuedJourney = {
    id: `journey-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
    journeyKey,
    input,
    status: 'queued',
    queuedAt: new Date().toISOString(),
    attempts: 0,
  };
  
  queue.push(queuedJourney);
  await saveJourneyQueue(queue);
  
  // Notify clients
  self.clients.matchAll().then((clients) => {
    clients.forEach((client) => {
      client.postMessage({
        type: 'JOURNEY_QUEUED',
        data: queuedJourney,
      });
    });
  });
  
  return { queued: true, id: queuedJourney.id };
}

// Get journey queue from IndexedDB
async function getJourneyQueue() {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('sp-journeys-db', 1);
    
    request.onerror = () => reject(request.error);
    
    request.onupgradeneeded = (event) => {
      const db = event.target.result;
      if (!db.objectStoreNames.contains('journeyQueue')) {
        db.createObjectStore('journeyQueue', { keyPath: 'id' });
      }
    };
    
    request.onsuccess = () => {
      const db = request.result;
      const tx = db.transaction('journeyQueue', 'readonly');
      const store = tx.objectStore('journeyQueue');
      const getAllRequest = store.getAll();
      
      getAllRequest.onsuccess = () => resolve(getAllRequest.result || []);
      getAllRequest.onerror = () => reject(getAllRequest.error);
    };
  });
}

// Save journey queue to IndexedDB
async function saveJourneyQueue(queue) {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('sp-journeys-db', 1);
    
    request.onerror = () => reject(request.error);
    
    request.onsuccess = () => {
      const db = request.result;
      const tx = db.transaction('journeyQueue', 'readwrite');
      const store = tx.objectStore('journeyQueue');
      
      store.clear();
      queue.forEach((item) => store.add(item));
      
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
    };
  });
}

// Sync queued journeys when online
async function syncQueuedJourneys() {
  const queue = await getJourneyQueue();
  
  if (queue.length === 0) {
    return { synced: 0, failed: 0, results: [] };
  }
  
  console.log('[SW] Syncing', queue.length, 'queued journeys');
  
  const results = [];
  const remainingQueue = [];
  
  for (const item of queue) {
    try {
      const response = await fetch(`/api/journeys/${item.journeyKey}/start`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(item.input),
      });
      
      if (response.ok) {
        const result = await response.json();
        results.push({
          id: item.id,
          journeyKey: item.journeyKey,
          success: true,
          journeyRunId: result.journeyRunId,
        });
      } else {
        item.attempts++;
        item.lastError = `Server returned ${response.status}`;
        if (item.attempts < 3) {
          remainingQueue.push(item);
        }
        results.push({
          id: item.id,
          journeyKey: item.journeyKey,
          success: false,
          error: item.lastError,
        });
      }
    } catch (error) {
      item.attempts++;
      item.lastError = error.message;
      if (item.attempts < 3) {
        remainingQueue.push(item);
      }
      results.push({
        id: item.id,
        journeyKey: item.journeyKey,
        success: false,
        error: error.message,
      });
    }
  }
  
  await saveJourneyQueue(remainingQueue);
  
  // Notify clients
  self.clients.matchAll().then((clients) => {
    clients.forEach((client) => {
      client.postMessage({
        type: 'JOURNEYS_SYNCED',
        data: {
          synced: results.filter(r => r.success).length,
          failed: results.filter(r => !r.success).length,
          results,
        },
      });
    });
  });
  
  return {
    synced: results.filter(r => r.success).length,
    failed: results.filter(r => !r.success).length,
    results,
  };
}

console.log('[SW] Service worker loaded with journey support');
