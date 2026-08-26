export interface AuthResponse {
  access_token: string
  token_type: string
  user_id: string
  expires_in: number
}

export interface LoginPayload {
  email: string
  password: string
}

export interface RegisterPayload extends LoginPayload {
  display_name: string
}

export type IdentityProvider = 'google' | 'facebook'

export interface ProviderStatePayload {
  provider: IdentityProvider
  create_account: boolean
}

export interface ProviderStateResponse {
  state: string
  expires_in: number
}

export interface GoogleLoginPayload {
  credential: string
  state: string
  create_account: boolean
}

export interface FacebookLoginPayload {
  access_token: string
  state: string
  create_account: boolean
}

export interface UserProfile {
  id: string
  email: string
  display_name: string
  created_at: string
  updated_at: string
}
