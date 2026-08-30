/* global self */
self.addEventListener('push', (event) => {
  if (!event.data) return
  let payload
  try {
    payload = event.data.json()
  } catch {
    return
  }
  const notification = payload.notification || {}
  const data = payload.data || {}
  const title = notification.title || 'Book Store Admin'
  const link = payload.fcmOptions?.link || data.link || '/'
  event.waitUntil(
    self.registration.showNotification(title, {
      body: notification.body || '',
      icon: '/favicon.svg',
      badge: '/favicon.svg',
      tag: data.notification_id || undefined,
      data: { link },
    }),
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  event.waitUntil(
    (async () => {
      const requested = new URL(event.notification.data?.link || '/', self.location.origin)
      const target =
        requested.origin === self.location.origin ? requested.href : self.location.origin
      const clients = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      for (const client of clients) {
        if ('focus' in client) {
          await client.navigate(target)
          return client.focus()
        }
      }
      return self.clients.openWindow(target)
    })(),
  )
})
