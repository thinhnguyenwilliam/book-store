import { apiRequest } from '@/shared/api/http-client'

export interface PushDevicePayload {
  device_id: string
  registration_token: string
  application: 'admin'
  platform: 'web'
}

export function registerPushDevice(payload: PushDevicePayload): Promise<void> {
  return apiRequest('/api/v1/notifications/devices', { method: 'POST', data: payload })
}

export function unregisterPushDevice(deviceId: string): Promise<void> {
  return apiRequest(`/api/v1/notifications/devices/${encodeURIComponent(deviceId)}`, {
    method: 'DELETE',
  })
}
