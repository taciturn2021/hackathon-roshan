// Firebase Auth: the browser signs in here and sends the ID token to the API on every call.
import { initializeApp } from 'firebase/app'
import { getAuth, type Auth } from 'firebase/auth'

const env = import.meta.env

/** Local testing against an API started with AUTH_DEV=1: VITE_DEV_AUTH=<name> sends `dev:<role>-<name>` and skips Firebase. */
export const DEV: string | undefined = env.VITE_DEV_AUTH || undefined
const DEV_KEY = 'rr:dev'
export const setDevUser = (role: string) => sessionStorage.setItem(DEV_KEY, `dev:${role}-${DEV}`)

let cached: Auth | undefined
/** Created on first use, so mock and dev modes run without Firebase settings. */
export function auth() {
  if (!cached) {
    cached = getAuth(initializeApp({
      apiKey: env.VITE_FIREBASE_API_KEY,
      authDomain: env.VITE_FIREBASE_AUTH_DOMAIN,
      projectId: env.VITE_FIREBASE_PROJECT_ID,
      appId: env.VITE_FIREBASE_APP_ID,
    }))
    // ponytail: no reCAPTCHA. Firebase then accepts only the test phone numbers registered in the project
    // (the demo logins); real numbers are refused. To take real numbers, delete this line: the captcha returns.
    cached.settings.appVerificationDisabledForTesting = true
  }
  return cached
}

/** A fresh ID token (Firebase renews it when it is close to its one-hour expiry), or null when signed out. */
export async function idToken(): Promise<string | null> {
  if (DEV) return sessionStorage.getItem(DEV_KEY)
  await auth().authStateReady()
  return (await auth().currentUser?.getIdToken()) ?? null
}
