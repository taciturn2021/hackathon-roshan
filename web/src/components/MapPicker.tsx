import { useEffect, useRef, type ReactNode } from 'react'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { Icon } from './Icon'

export type Point = { lat: number; lng: number }

// The service covers Islamabad and Rawalpindi: the map cannot be dragged far outside them.
const TWIN_CITIES = L.latLngBounds([33.35, 72.75], [33.9, 73.4])

/**
 * A map with a fixed centre pin. The customer moves the map under the pin to say where the job is;
 * the circle shows the area searched for workers. Tiles are OpenStreetMap's (credit shown on the map).
 * `at` moves the map from outside (for example after "use my location").
 */
export default function MapPicker({ at, radiusKm, onMove, onFail, children }: {
  at: Point; radiusKm: number; onMove: (p: Point) => void; onFail: () => void; children?: ReactNode
}) {
  const box = useRef<HTMLDivElement>(null)
  const map = useRef<L.Map | null>(null)
  const handlers = useRef({ onMove, onFail })
  useEffect(() => { handlers.current = { onMove, onFail } })

  useEffect(() => {
    const m = L.map(box.current!, {
      center: [at.lat, at.lng], zoom: 12, minZoom: 11, maxZoom: 18, // 12 shows the whole 5 km search circle on a phone
      maxBounds: TWIN_CITIES, maxBoundsViscosity: 1,
      scrollWheelZoom: false, // the wheel zooms only after a click, so the map never traps page scrolling
    })
    const tiles = L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19, attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    }).addTo(m)
    // No tiles at all (offline, or the tile server is blocked): hand back to the plain buttons.
    let loaded = 0, failed = 0
    tiles.on('tileload', () => { loaded++ })
    tiles.on('tileerror', () => { if (++failed >= 6 && loaded === 0) handlers.current.onFail() })

    const circle = L.circle([at.lat, at.lng], {
      radius: radiusKm * 1000, color: '#102A1B', weight: 1, fillColor: '#C8F169', fillOpacity: 0.14, interactive: false,
    }).addTo(m)
    m.on('move', () => circle.setLatLng(m.getCenter()))
    m.on('moveend', () => { const c = m.getCenter(); handlers.current.onMove({ lat: c.lat, lng: c.lng }) })
    m.on('click', () => m.scrollWheelZoom.enable())
    map.current = m
    return () => { m.remove(); map.current = null }
    // The map is created once; later changes to `at` are handled by the effect below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    const m = map.current
    // Only a real jump moves the map; the centre it reported itself comes back here unchanged.
    if (m && m.getCenter().distanceTo([at.lat, at.lng]) > 5) m.setView([at.lat, at.lng], Math.max(m.getZoom(), 15))
  }, [at.lat, at.lng])

  return (
    <div className="map-picker" dir="ltr">
      <div ref={box} className="map-canvas" />
      <span className="map-pin" aria-hidden="true"><Icon name="MapPin" /></span>
      {children}
    </div>
  )
}
