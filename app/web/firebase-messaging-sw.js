// Firebase Cloud Messaging Service Worker
// This handles push notifications when the app is in the background

importScripts('https://www.gstatic.com/firebasejs/9.0.0/firebase-app-compat.js');
importScripts('https://www.gstatic.com/firebasejs/9.0.0/firebase-messaging-compat.js');

// Initialize Firebase (replace with your config)
firebase.initializeApp({
  apiKey: "YOUR_API_KEY",
  authDomain: "YOUR_PROJECT.firebaseapp.com",
  projectId: "YOUR_PROJECT",
  storageBucket: "YOUR_PROJECT.appspot.com",
  messagingSenderId: "YOUR_SENDER_ID",
  appId: "YOUR_APP_ID"
});

const messaging = firebase.messaging();

// Handle background messages
messaging.onBackgroundMessage((payload) => {
  console.log('Received background message:', payload);

  const notificationTitle = payload.notification?.title || 'New Message';
  const notificationOptions = {
    body: payload.notification?.body || 'You have a new message',
    icon: '/icons/Icon-192.png',
    badge: '/icons/Icon-192.png',
    tag: payload.data?.conversation_id || 'default',
    data: payload.data,
    actions: [
      { action: 'reply', title: 'Reply' },
      { action: 'mark_read', title: 'Mark as Read' }
    ],
    requireInteraction: true,
    vibrate: [200, 100, 200]
  };

  return self.registration.showNotification(notificationTitle, notificationOptions);
});

// Handle notification click
self.addEventListener('notificationclick', (event) => {
  console.log('Notification clicked:', event);
  event.notification.close();

  const conversationId = event.notification.data?.conversation_id;
  const action = event.action;

  if (action === 'reply') {
    // Open app to conversation
    event.waitUntil(
      clients.matchAll({ type: 'window' }).then((clientList) => {
        // Focus existing window or open new one
        for (const client of clientList) {
          if ('focus' in client) {
            client.focus();
            client.postMessage({
              type: 'NOTIFICATION_CLICK',
              action: 'reply',
              conversationId: conversationId
            });
            return;
          }
        }
        // No window open, open new one
        if (clients.openWindow) {
          return clients.openWindow(`/?conversation=${conversationId}`);
        }
      })
    );
  } else if (action === 'mark_read') {
    // Send API request to mark as read
    event.waitUntil(
      fetch('/api/receipts/read', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ conversation_id: conversationId })
      })
    );
  } else {
    // Default: open app
    event.waitUntil(
      clients.matchAll({ type: 'window' }).then((clientList) => {
        for (const client of clientList) {
          if ('focus' in client) {
            return client.focus();
          }
        }
        if (clients.openWindow) {
          const url = conversationId ? `/?conversation=${conversationId}` : '/';
          return clients.openWindow(url);
        }
      })
    );
  }
});

// Handle push subscription change
self.addEventListener('pushsubscriptionchange', (event) => {
  console.log('Push subscription changed');
  event.waitUntil(
    self.registration.pushManager.subscribe({ userVisibleOnly: true })
      .then((subscription) => {
        // Send new subscription to server
        return fetch('/api/notifications/register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            token: JSON.stringify(subscription),
            platform: 'web'
          })
        });
      })
  );
});
