import { AxiosHeaders, type AxiosAdapter } from 'axios'
import { afterEach, describe, expect, it } from 'vitest'

import { apiRequest, setApiAccessToken } from './http-client'

describe('Axios API client', () => {
  afterEach(() => setApiAccessToken(null))

  it('sends the in-memory access token and enables credentialed cookies', async () => {
    const adapter: AxiosAdapter = async (config) => {
      expect(config.headers.get('Authorization')).toBe('Bearer access-token')
      expect(config.withCredentials).toBe(true)
      return {
        data: { ok: true },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }
    setApiAccessToken('access-token')

    await expect(apiRequest<{ ok: boolean }>('/test', { adapter })).resolves.toEqual({ ok: true })
  })
})
