import { getApps, initializeApp } from 'firebase/app'
import {
  deleteToken,
  getMessaging,
  getToken,
  isSupported,
  onMessage,
  type MessagePayload,
  type Messaging,
} from 'firebase/messaging'

import { env } from '@/shared/config/env'
import * as api from '../api/push.api'

const deviceIDKey = 'bookstore.admin.push-device-id.v1'
let messagingPromise: Promise<Messaging | null> | undefined

export const pushConfigured = Boolean(
  env.firebase.apiKey &&
  env.firebase.projectId &&
  env.firebase.messagingSenderId &&
  env.firebase.appId &&
  env.firebase.vapidKey,
)

function deviceID(): string {
  const existing = window.localStorage.getItem(deviceIDKey)
  if (existing) return existing
  const created = window.crypto.randomUUID()
  window.localStorage.setItem(deviceIDKey, created)
  return created
}

async function messaging(): Promise<Messaging | null> {
  if (!pushConfigured || !(await isSupported())) return null
  if (!messagingPromise) {
    messagingPromise = Promise.resolve(
      getMessaging(
        getApps()[0] ??
          initializeApp({
            apiKey: env.firebase.apiKey,
            authDomain: env.firebase.authDomain,
            projectId: env.firebase.projectId,
            storageBucket: env.firebase.storageBucket,
            messagingSenderId: env.firebase.messagingSenderId,
            appId: env.firebase.appId,
          }),
      ),
    )
  }
  return messagingPromise
}

async function serviceWorker(): Promise<ServiceWorkerRegistration> {
  return navigator.serviceWorker.register('/firebase-messaging-sw.js', { scope: '/' })
}

export async function supportsPush(): Promise<boolean> {
  return pushConfigured && 'Notification' in window && 'serviceWorker' in navigator && isSupported()
}

export async function synchronizePush(): Promise<boolean> {
  if (Notification.permission !== 'granted') return false
  const instance = await messaging()
  if (!instance) return false
  const registrationToken = await getToken(instance, {
    vapidKey: env.firebase.vapidKey,
    serviceWorkerRegistration: await serviceWorker(),
  })
  if (!registrationToken) return false
  await api.registerPushDevice({
    device_id: deviceID(),
    registration_token: registrationToken,
    application: 'admin',
    platform: 'web',
  })
  return true
}

export async function requestAndEnablePush(): Promise<boolean> {
  if (!(await supportsPush())) return false
  if ((await Notification.requestPermission()) !== 'granted') return false
  return synchronizePush()
}

export async function unregisterCurrentPushDevice(): Promise<void> {
  const id = window.localStorage.getItem(deviceIDKey)
  if (id) await api.unregisterPushDevice(id).catch(() => undefined)
  await revokeLocalPushToken()
}

export async function revokeLocalPushToken(): Promise<void> {
  const instance = await messaging().catch(() => null)
  if (instance) await deleteToken(instance).catch(() => false)
}

export async function onForegroundPush(
  listener: (payload: MessagePayload) => void,
): Promise<() => void> {
  const instance = await messaging()
  return instance ? onMessage(instance, listener) : () => undefined
}
