import { useEffect, useRef, useState, useSyncExternalStore } from 'react'
import { MOCK } from './api'

// One clip at a time, like a phone speaker.
let current: HTMLAudioElement | null = null
export function play(url: string): Promise<HTMLAudioElement> {
  current?.pause()
  const a = (current = new Audio(url))
  return a.play().then(() => a)
}
/** Recorded Urdu prompts from docs/4-UI_UX_BRIEF.md, served from public/audio/. */
export const playPrompt = (name: string) => play(`/audio/${name}.mp3`)

// Toast: a short, non-blocking status line shown at the bottom of the page.
let toastText: string | null = null
let timer: ReturnType<typeof setTimeout> | undefined
const listeners = new Set<() => void>()
export function toast(text: string) {
  toastText = text
  listeners.forEach((l) => l())
  clearTimeout(timer)
  timer = setTimeout(() => { toastText = null; listeners.forEach((l) => l()) }, 6000)
}
export const useToast = () => useSyncExternalStore((l) => (listeners.add(l), () => void listeners.delete(l)), () => toastText)

export type RecorderState = 'idle' | 'recording' | 'denied'
/** Tap to start, tap again to stop; `onDone` receives the clip. */
export function useRecorder(onDone: (clip: Blob) => void) {
  const [state, setState] = useState<RecorderState>('idle')
  const rec = useRef<MediaRecorder | null>(null)
  const done = useRef(onDone)
  useEffect(() => { done.current = onDone })
  // Leaving the page discards a recording in progress; without clearing onstop, stopping the tracks would send it.
  useEffect(() => () => {
    const r = rec.current
    if (r) { r.onstop = null; r.stream.getTracks().forEach((t) => t.stop()) }
  }, [])

  async function toggle() {
    if (rec.current?.state === 'recording') return rec.current.stop()
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const r = new MediaRecorder(stream), parts: Blob[] = []
      r.ondataavailable = (e) => parts.push(e.data)
      r.onstop = () => {
        stream.getTracks().forEach((t) => t.stop())
        setState('idle')
        done.current(new Blob(parts, { type: r.mimeType }))
      }
      rec.current = r
      r.start()
      setState('recording')
    } catch {
      // Demo mode has no speech service, so an empty clip lets the flow continue without a microphone.
      if (MOCK) done.current(new Blob())
      else setState('denied')
    }
  }
  return { state, toggle, reset: () => setState('idle') }
}
