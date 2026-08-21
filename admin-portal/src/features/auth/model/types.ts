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

export interface GoogleLoginPayload {
  credential: string
  create_account: boolean
}

export interface FacebookLoginPayload {
  access_token: string
  create_account: boolean
}

export interface UserProfile {
  id: string
  email: string
  display_name: string
  created_at: string
  updated_at: string
}

export interface JwtClaims {
  sub?: string
  email?: string
  roles?: string[]
  exp?: number
}
