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

export interface GoogleLoginPayload {
  credential: string
  create_account: boolean
}

export interface UserProfile {
  id: string
  email: string
  display_name: string
  created_at: string
  updated_at: string
}
