import { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Paper, Typography } from '@mui/material';
import { useTranslation } from 'react-i18next';
import * as maplibregl from 'maplibre-gl';
import type { StyleSpecification } from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import type { Position, Unit } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';
import { useUnits } from '../hooks/useUnits';
import { fmtDate } from '../lib/format';
import { formatTacticalName } from '../lib/tacticalName';
import { symbolDataUrl } from '../lib/unitSymbol';
import './MapPage.scss';

type PlacedUnit = Unit & { position: Position };

// OpenStreetMap's raster tiles, as a MapLibre style.
const MAP_STYLE: StyleSpecification = {
  version: 8,
  sources: {
    osm: {
      type: 'raster',
      tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
      tileSize: 256,
      maxzoom: 19,
      attribution:
        '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    },
  },
  layers: [{ id: 'osm', type: 'raster', source: 'osm' }],
};

// Shown until the units are loaded, and when none has a position.
const DEFAULT_CENTER: [number, number] = [10.45, 51.16];
const DEFAULT_ZOOM = 5;
const MAX_FIT_ZOOM = 15;

// Symbols are 3:2, like in the unit list.
const SYMBOL_HEIGHT = 40;
const DOT_SIZE = 16;

export default function MapPage() {
  const { t } = useTranslation();
  const { units, loading, error } = useUnits();
  const containerRef = useRef<HTMLDivElement>(null);
  const [map, setMap] = useState<maplibregl.Map | null>(null);

  const placed = useMemo(() => units.filter((u): u is PlacedUnit => u.position !== null), [units]);

  useEffect(() => {
    const m = new maplibregl.Map({
      container: containerRef.current!,
      style: MAP_STYLE,
      center: DEFAULT_CENTER,
      zoom: DEFAULT_ZOOM,
    });
    m.addControl(new maplibregl.NavigationControl());
    m.addControl(new maplibregl.ScaleControl());
    setMap(m);
    return () => {
      m.remove();
      setMap(null);
    };
  }, []);

  // Fits the view to the units once, after they are first loaded; later
  // updates leave the view alone so the map doesn't jump while in use.
  const fitted = useRef(false);
  useEffect(() => {
    if (!map || loading || fitted.current || placed.length === 0) return;
    fitted.current = true;
    const bounds = new maplibregl.LngLatBounds();
    for (const u of placed) bounds.extend([u.position.lon, u.position.lat]);
    map.fitBounds(bounds, { padding: 60, maxZoom: MAX_FIT_ZOOM, animate: false });
  }, [map, loading, placed]);

  return (
    <Paper className="map-page">
      <ErrorBanner message={error} />
      <div ref={containerRef} className="map-page__map" />
      {map && placed.map((u) => <UnitMarker key={u.id} map={map} unit={u} />)}
      {!loading && placed.length === 0 && (
        <Typography variant="body2" color="text.secondary" className="map-page__empty">
          {t('map.empty')}
        </Typography>
      )}
    </Paper>
  );
}

/** A MapLibre marker whose element and popup are rendered by React. */
function UnitMarker({ map, unit }: { map: maplibregl.Map; unit: PlacedUnit }) {
  const [element] = useState(() => document.createElement('div'));
  const [popupContent] = useState(() => document.createElement('div'));
  const [popup] = useState(() => new maplibregl.Popup({ className: 'map-page__popup' }));
  const [marker] = useState(() => new maplibregl.Marker({ element }).setPopup(popup));
  const src = useMemo(
    () => symbolDataUrl(unit.symbol, unit.tacticalName),
    [unit.symbol, unit.tacticalName],
  );
  const { lat, lon } = unit.position;

  // Declared before the effect that adds the marker, so it has a position
  // by the time it's on the map.
  useEffect(() => {
    marker.setLngLat([lon, lat]);
  }, [marker, lat, lon]);

  useEffect(() => {
    popup.setDOMContent(popupContent);
  }, [popup, popupContent]);

  useEffect(() => {
    popup.setOffset((src ? SYMBOL_HEIGHT : DOT_SIZE) / 2);
  }, [popup, src]);

  useEffect(() => {
    marker.addTo(map);
    return () => {
      marker.remove();
    };
  }, [map, marker]);

  return (
    <>
      {createPortal(
        // Units without a symbol get a plain dot.
        src ? (
          <img className="map-page__symbol" src={src} alt={unit.name} title={unit.name} />
        ) : (
          <span className="map-page__dot" title={unit.name} />
        ),
        element,
      )}
      {createPortal(<UnitPopupContent unit={unit} />, popupContent)}
    </>
  );
}

function UnitPopupContent({ unit }: { unit: PlacedUnit }) {
  const { t, i18n } = useTranslation();
  const tacticalName = formatTacticalName(unit.tacticalName);
  const { lat, lon, height, timestamp } = unit.position;
  return (
    <>
      <strong>{unit.name}</strong>
      {tacticalName && <div>{tacticalName}</div>}
      <div>
        {lat.toFixed(5)}, {lon.toFixed(5)}
        {height !== null && ` · ${t('unitRow.height', { height })}`}
      </div>
      <div className="map-page__popup-time">{fmtDate(timestamp, i18n.language)}</div>
    </>
  );
}
