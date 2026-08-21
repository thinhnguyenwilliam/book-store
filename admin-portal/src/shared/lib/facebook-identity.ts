interface FacebookAuthResponse {
  accessToken: string
  userID: string
  expiresIn: number
}

interface FacebookLoginResponse {
  status?: 'connected' | 'not_authorized' | 'unknown'
  authResponse?: FacebookAuthResponse
}

interface FacebookSDK {
  init(options: {
    appId: string
    cookie: boolean
    xfbml: boolean
    version: string
    autoLogAppEvents: boolean
  }): void
  login(
    callback: (response: FacebookLoginResponse) => void,
    options: { scope: string; return_scopes: boolean },
  ): void
}

type FacebookWindow = Window & {
  FB?: FacebookSDK
  fbAsyncInit?: () => void
}

const scriptID = 'facebook-javascript-sdk'
const scriptSource = 'https://connect.facebook.net/en_US/sdk.js'

let loadPromise: Promise<FacebookSDK> | undefined
let initializedKey = ''
let initializedSDK: FacebookSDK | undefined

function facebookWindow(): FacebookWindow {
  return window as FacebookWindow
}

function loadFacebookSDK(): Promise<FacebookSDK> {
  const loaded = facebookWindow().FB
  if (loaded) return Promise.resolve(loaded)
  if (loadPromise) return loadPromise

  loadPromise = new Promise((resolve, reject) => {
    const browser = facebookWindow()

    if (!document.getElementById('fb-root')) {
      const root = document.createElement('div')
      root.id = 'fb-root'
      document.body.prepend(root)
    }

    const previousInitializer = browser.fbAsyncInit
    browser.fbAsyncInit = () => {
      previousInitializer?.()
      if (browser.FB) {
        resolve(browser.FB)
        return
      }
      reject(new Error('Facebook SDK did not initialize.'))
    }

    const existing = document.getElementById(scriptID) as HTMLScriptElement | null
    if (existing) return

    const script = document.createElement('script')
    script.id = scriptID
    script.src = scriptSource
    script.async = true
    script.defer = true
    script.crossOrigin = 'anonymous'
    script.addEventListener(
      'error',
      () => {
        loadPromise = undefined
        reject(new Error('Could not load Facebook SDK.'))
      },
      { once: true },
    )
    document.head.append(script)
  })

  return loadPromise
}

export async function initializeFacebook(appID: string, graphVersion: string): Promise<void> {
  const sdk = await loadFacebookSDK()
  const key = `${appID}:${graphVersion}`
  if (initializedKey !== key) {
    sdk.init({
      appId: appID,
      cookie: false,
      xfbml: false,
      version: graphVersion,
      autoLogAppEvents: false,
    })
    initializedKey = key
  }
  initializedSDK = sdk
}

export function requestFacebookLogin(): Promise<string> {
  const sdk = initializedSDK
  if (!sdk) {
    return Promise.reject(new Error('Facebook SDK is not ready.'))
  }
  return new Promise((resolve, reject) => {
    sdk.login(
      (response) => {
        const accessToken = response.authResponse?.accessToken?.trim()
        if (response.status === 'connected' && accessToken) {
          resolve(accessToken)
          return
        }
        reject(new Error('Facebook login was cancelled or not authorized.'))
      },
      { scope: 'public_profile,email', return_scopes: true },
    )
  })
}
