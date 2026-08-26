export type GoogleButtonText = 'signin_with' | 'signup_with' | 'continue_with'

interface GoogleCredentialResponse {
  credential: string
  select_by: string
  state?: string
}

interface GoogleIdentityAPI {
  initialize(config: {
    client_id: string
    callback: (response: GoogleCredentialResponse) => void
    auto_select?: boolean
    cancel_on_tap_outside?: boolean
    nonce?: string
  }): void
  renderButton(
    parent: HTMLElement,
    options: {
      type: 'standard'
      theme: 'outline'
      size: 'large'
      shape: 'pill'
      text: GoogleButtonText
      logo_alignment: 'left'
      width: number
      state?: string
    },
  ): void
  disableAutoSelect(): void
}

interface GoogleNamespace {
  accounts: {
    id: GoogleIdentityAPI
  }
}

type GoogleWindow = Window & {
  google?: GoogleNamespace
}

const scriptID = 'google-identity-services'
const scriptSource = 'https://accounts.google.com/gsi/client'

let loadPromise: Promise<GoogleNamespace> | undefined
let initializedClientID = ''
let credentialHandler: ((credential: string, state: string) => void) | undefined

function googleNamespace(): GoogleNamespace | undefined {
  return (window as GoogleWindow).google
}

function loadGoogleIdentity(): Promise<GoogleNamespace> {
  const loaded = googleNamespace()
  if (loaded) return Promise.resolve(loaded)
  if (loadPromise) return loadPromise

  loadPromise = new Promise((resolve, reject) => {
    const onLoad = () => {
      const google = googleNamespace()
      if (google) {
        resolve(google)
        return
      }
      reject(new Error('Google Identity Services did not initialize.'))
    }
    const onError = () => reject(new Error('Could not load Google Identity Services.'))

    const existing = document.getElementById(scriptID) as HTMLScriptElement | null
    if (existing) {
      existing.addEventListener('load', onLoad, { once: true })
      existing.addEventListener('error', onError, { once: true })
      return
    }

    const script = document.createElement('script')
    script.id = scriptID
    script.src = scriptSource
    script.async = true
    script.defer = true
    script.addEventListener('load', onLoad, { once: true })
    script.addEventListener('error', onError, { once: true })
    document.head.append(script)
  })

  return loadPromise
}

export async function renderGoogleButton(
  parent: HTMLElement,
  clientID: string,
  text: GoogleButtonText,
  state: string,
  onCredential: (credential: string, state: string) => void,
): Promise<void> {
  const google = await loadGoogleIdentity()
  credentialHandler = onCredential

  const initializationKey = `${clientID}:${state}`
  if (initializedClientID !== initializationKey) {
    google.accounts.id.initialize({
      client_id: clientID,
      callback: (response) => credentialHandler?.(response.credential, response.state ?? ''),
      auto_select: false,
      cancel_on_tap_outside: true,
      nonce: state,
    })
    initializedClientID = initializationKey
  }

  parent.replaceChildren()
  google.accounts.id.renderButton(parent, {
    type: 'standard',
    theme: 'outline',
    size: 'large',
    shape: 'pill',
    text,
    logo_alignment: 'left',
    width: Math.min(Math.max(parent.clientWidth, 240), 400),
    state,
  })
}

export function disableGoogleAutoSelect(): void {
  googleNamespace()?.accounts.id.disableAutoSelect()
}
