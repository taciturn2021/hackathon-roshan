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
export const playPrompt = (name: string) => play(`/audio/${name}.${name === 'ring' ? 'wav' : 'mp3'}`) // the ringtone is a tone, not speech

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

/** 16 kHz mono 16-bit WAV. The main speech-to-text model does not take the browser's own webm recordings. */
export async function toWav(clip: Blob): Promise<File> {
  const ctx = new AudioContext()
  const src = await ctx.decodeAudioData(await clip.arrayBuffer()).finally(() => void ctx.close())
  const rate = 16000
  const off = new OfflineAudioContext(1, Math.ceil(src.duration * rate), rate)
  const node = off.createBufferSource()
  node.buffer = src
  node.connect(off.destination)
  node.start()
  const pcm = (await off.startRendering()).getChannelData(0)
  const view = new DataView(new ArrayBuffer(44 + pcm.length * 2))
  const text = (at: number, s: string) => { for (let i = 0; i < s.length; i++) view.setUint8(at + i, s.charCodeAt(i)) }
  text(0, 'RIFF'); view.setUint32(4, 36 + pcm.length * 2, true); text(8, 'WAVEfmt ')
  view.setUint32(16, 16, true); view.setUint16(20, 1, true); view.setUint16(22, 1, true)
  view.setUint32(24, rate, true); view.setUint32(28, rate * 2, true); view.setUint16(32, 2, true); view.setUint16(34, 16, true)
  text(36, 'data'); view.setUint32(40, pcm.length * 2, true)
  pcm.forEach((s, i) => view.setInt16(44 + i * 2, Math.max(-1, Math.min(1, s)) * 0x7fff, true))
  return new File([view], 'clip.wav', { type: 'audio/wav' })
}

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
        // If the browser cannot decode its own recording, send it as it is: the fallback model takes webm.
        const raw = new File(parts, r.mimeType.includes('mp4') ? 'clip.m4a' : 'clip.webm', { type: r.mimeType })
        toWav(raw).catch(() => raw).then((clip) => done.current(clip))
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
