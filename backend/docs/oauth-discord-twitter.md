# Discord / X (Twitter) login and registration

The storefront can create a customer account on first OAuth login. The admin
portal passes create_account=false and checks the existing admin role. No
provider grants admin automatically. Accounts are identified by provider + stable
user ID, never by username.

## Configure providers

Discord: https://discord.com/developers/applications → select/create application →
OAuth2. Copy Client ID and Client Secret, and register BOTH redirect URLs:

- http://localhost:5173/auth/callback/discord
- http://localhost:5174/auth/callback/discord

Scopes are identify and email. No bot installation/token is required. The Discord
account must have a verified email.

X: https://developer.x.com → application → User authentication settings → OAuth2.
Choose a confidential **Web App**, obtain OAuth2 Client ID/Client Secret (not
OAuth1 API keys), and register:

- http://localhost:5173/auth/callback/twitter
- http://localhost:5174/auth/callback/twitter

This uses Authorization Code + PKCE S256, requesting tweet.read, users.read and
users.email. The app needs access to GET /2/users/me with user.fields=confirmed_email.
Actual API access depends on your X developer application entitlements. If the
scope/API is not available, fix that in X or use another login provider; this app
does not invent an email address. The verified/blue-check profile flag is NOT
used as proof of email verification.

Copy the sections from config/oauth.secret.yml.example into the existing,
gitignored config/local.secret.yml (merge in your editor; do not overwrite other
Google/Facebook/payment secrets). Fill IDs/secrets only on the backend. In each
frontend .env enable:

```dotenv
VITE_DISCORD_ENABLED=true
VITE_TWITTER_ENABLED=true
```

These are visibility flags, not secrets. Restart Vite after changing them.
Auth Service reads the YAML override with -secrets config/local.secret.yml;
root make local already supplies it when present. To run Auth Service individually
from backend/ (stop any existing Auth Service first):

```sh
go run ./cmd/auth-service -config config/local.yml -secrets config/local.secret.yml
```

Apply migration 019 using make migrate before using the new endpoints.
Production must replace localhost with exact HTTPS frontend callback URLs in
both provider consoles and the backend allowlists, and configure trusted CORS
origins plus Secure HttpOnly refresh cookies.

## API and security

- POST /api/v1/auth/oauth/discord/start (or twitter)
  body: {"redirect_uri":"http://localhost:5173/auth/callback/discord","create_account":true}
- Response: authorization_url, state, expires_in; browser receives an HttpOnly
  state cookie. Navigate to authorization_url.
- Provider redirects browser back to the frontend callback route.
- POST /api/v1/auth/oauth/discord/finish (or twitter)
  body: redirect_uri, create_account, state, code.
- Response: the app's short-lived access token; refresh token stays in HttpOnly
  cookie as with existing Google/Facebook login. Provider tokens never reach
  the frontend and are not persisted. No offline.access scope is requested;
  provider refresh tokens are unnecessary for sign-in only.

State is random, hashed at rest, bound to provider/redirect/account intent, expires
after 10 minutes, and is consumed atomically by PostgreSQL DELETE RETURNING.
PKCE verifier stays server-side. Failed or interrupted exchange requires a new
login attempt, not replaying the authorization code. Concurrent tabs for the same
provider may invalidate the older attempt; start again if necessary.

Errors have stable codes: invalid_oauth_state, provider_email_required,
provider_not_configured, external_identity_conflict and provider_unavailable.
Frontend cancellation is handled without exchanging a token.

If the verified provider email matches an existing account that is not already
linked to that provider, login is rejected with external_identity_conflict.
Do not auto-link Discord/X to existing accounts based solely on matching email.
Use the existing login method; a future explicit account-linking flow must require
authentication of both accounts. To use OAuth for admin, an account first created
through that provider must be granted admin through the existing authorized
administrative process.

The callback clears the code from browser history. Serve frontend with a
no-referrer policy and avoid logging callback query strings at reverse proxies.
Expired abandoned transactions are cleaned on the next OAuth start.

## Verification

Unit tests mock token/profile HTTP responses; no real Discord/X credentials
are included. Live end-to-end provider login must be tested after configuring
your own app IDs, secrets and allowed redirects.

Official references:
- https://docs.discord.com/developers/topics/oauth2
- https://docs.x.com/fundamentals/authentication/oauth-2-0/authorization-code
- https://docs.x.com/x-api/users/get-my-user
