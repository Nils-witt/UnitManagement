import { useEffect, useMemo, useRef, useState, type CSSProperties, type MouseEvent } from 'react';
import { createPortal } from 'react-dom';
import {
  Button,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Paper,
  Popover,
  Slider,
  TextField,
  Typography,
} from '@mui/material';
import PlaceIcon from '@mui/icons-material/Place';
import RouteIcon from '@mui/icons-material/Route';
import TuneIcon from '@mui/icons-material/Tune';
import { useTranslation } from 'react-i18next';
import * as maplibregl from 'maplibre-gl';
import type { StyleSpecification } from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import maplibreWorkerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url';
import type { Position, PositionHistoryEntry, Unit } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';
import { useApi } from '../hooks/useApi';
import { useUnitPositions } from '../hooks/useUnitPositions';
import { useUnits } from '../hooks/useUnits';
import { errorMessage } from '../lib/errors';
import { fmtDate } from '../lib/format';
import { formatTacticalName } from '../lib/tacticalName';
import { symbolDataUrl } from '../lib/unitSymbol';
import './MapPage.scss';

// MapLibre looks for its worker next to its own module, which bundling
// moves; without it, GeoJSON sources (the GPS track) never render.
maplibregl.setWorkerUrl(maplibreWorkerUrl);

type PlacedUnit = Unit & { position: Position };

/** The unit whose context menu is open, and where it was opened. */
interface MenuState {
  unit: PlacedUnit;
  x: number;
  y: number;
}

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

// Symbols are 3:2, like in the unit list; their height is adjustable and
// dots scale along with it.
const DEFAULT_SYMBOL_HEIGHT = 40;
const MIN_SYMBOL_HEIGHT = 20;
const MAX_SYMBOL_HEIGHT = 100;
const DOT_RATIO = 0.4;
const SYMBOL_HEIGHT_KEY = 'map.symbolHeight';

// The GPS track of one unit, drawn below the markers. MapLibre paints the
// layers itself, so the color can't come from the MUI theme.
const TRACK_SOURCE = 'gps-track';
const TRACK_COLOR = '#d32f2f';
const TRACK_POINTS_LAYER = `${TRACK_SOURCE}-points`;
// Including the stroke; also the popup's offset and the extra hit area.
const TRACK_POINT_RADIUS = 6;

// How far back the track reaches, in minutes; null shows all of it (as far as
// the server returns, the latest 1000 positions), 'custom' a timeframe the
// user enters.
const TRACK_SPANS = [5, 15, 30, 60, 6 * 60, 24 * 60, 7 * 24 * 60, null, 'custom'] as const;
type TrackSpan = (typeof TRACK_SPANS)[number];
type PresetSpan = Exclude<TrackSpan, 'custom'>;
const DEFAULT_TRACK_SPAN = 60 satisfies PresetSpan;
// Not 'map.trackSpan', which held hours.
const TRACK_SPAN_KEY = 'map.trackSpanMinutes';
const MINUTE_MS = 60_000;

/** A custom timeframe, as values of datetime-local inputs (local time); an
 * empty one leaves that end open. */
interface TrackRange {
  from: string;
  to: string;
}

/** The preset span the user picked last, from localStorage. A custom
 * timeframe isn't remembered, as it rarely fits a later visit. */
function loadTrackSpan(): PresetSpan {
  try {
    const stored = localStorage.getItem(TRACK_SPAN_KEY);
    const span = TRACK_SPANS.find((s) => String(s) === stored);
    if (span !== undefined && span !== 'custom') return span;
  } catch {
    // Storage may be unavailable, e.g. in a private window.
  }
  return DEFAULT_TRACK_SPAN;
}

/** The value of a datetime-local input showing `ms` in local time. */
function toLocalInput(ms: number): string {
  const offsetMs = new Date(ms).getTimezoneOffset() * 60_000;
  return new Date(ms - offsetMs).toISOString().slice(0, 16);
}

/** RFC 3339 for a datetime-local value, plus `extraMs`; null when empty or
 * incomplete. */
function fromLocalInput(value: string, extraMs = 0): string | null {
  const ms = new Date(value).getTime();
  return value && !Number.isNaN(ms) ? new Date(ms + extraMs).toISOString() : null;
}

function saveTrackSpan(span: PresetSpan) {
  try {
    localStorage.setItem(TRACK_SPAN_KEY, String(span));
  } catch {
    // Not remembered then; the span still applies until the page is left.
  }
}

/** The symbol height the user picked last, from localStorage. */
function loadSymbolHeight(): number {
  try {
    const stored = Number(localStorage.getItem(SYMBOL_HEIGHT_KEY));
    if (stored >= MIN_SYMBOL_HEIGHT && stored <= MAX_SYMBOL_HEIGHT) return stored;
  } catch {
    // Storage may be unavailable, e.g. in a private window.
  }
  return DEFAULT_SYMBOL_HEIGHT;
}

function saveSymbolHeight(height: number) {
  try {
    localStorage.setItem(SYMBOL_HEIGHT_KEY, String(height));
  } catch {
    // Not remembered then; the size still applies until the page is left.
  }
}

export default function MapPage() {
  const { t } = useTranslation();
  const api = useApi();
  const { units, loading, error: loadError, reloadUnits } = useUnits();
  const pageRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [map, setMap] = useState<maplibregl.Map | null>(null);
  const [menu, setMenu] = useState<MenuState | null>(null);
  // The unit being moved: while set, the next click on the map sets its position.
  const [movingId, setMovingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [symbolHeight, setSymbolHeight] = useState(loadSymbolHeight);
  const [settingsAnchor, setSettingsAnchor] = useState<HTMLElement | null>(null);
  // Sources and layers can only be added once the style has loaded.
  const [styleLoaded, setStyleLoaded] = useState(false);
  // The unit whose GPS history is shown on the map.
  const [trackId, setTrackId] = useState<string | null>(null);
  const [trackSpan, setTrackSpan] = useState<TrackSpan>(loadTrackSpan);
  const [trackRange, setTrackRange] = useState<TrackRange>({ from: '', to: '' });

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
    // Takes the whole page fullscreen, so the overlays on the map stay visible.
    m.addControl(new maplibregl.FullscreenControl({ container: pageRef.current! }));
    m.once('style.load', () => setStyleLoaded(true));
    setMap(m);
    return () => {
      m.remove();
      setMap(null);
      setStyleLoaded(false);
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

  // Looked up on every render, so the update is based on the unit's latest
  // state; picking ends if the unit is deleted meanwhile.
  const movingUnit = movingId === null ? null : (units.find((u) => u.id === movingId) ?? null);

  useEffect(() => {
    if (!map || !movingUnit) return;
    const canvas = map.getCanvas();
    canvas.style.cursor = 'crosshair';
    const onClick = (e: maplibregl.MapMouseEvent) => {
      setMovingId(null);
      setError(null);
      const { name, symbol, tacticalName } = movingUnit;
      // The old height doesn't apply to the new place; the server stamps the
      // position with the current time.
      const position = { lat: e.lngLat.lat, lon: e.lngLat.wrap().lng, height: null };
      api
        .updateUnit(movingUnit.id, { name, position, symbol, tacticalName })
        .then(reloadUnits)
        .catch((err: unknown) => setError(errorMessage(err)));
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setMovingId(null);
    };
    map.once('click', onClick);
    window.addEventListener('keydown', onKeyDown);
    return () => {
      canvas.style.cursor = '';
      map.off('click', onClick);
      window.removeEventListener('keydown', onKeyDown);
    };
  }, [map, movingUnit, api, reloadUnits]);

  // The track is hidden if the unit is deleted meanwhile.
  const trackUnit = trackId === null ? null : (units.find((u) => u.id === trackId) ?? null);
  // For a preset, the server returns the span as of when it was picked (or
  // the track opened); from then on the window moves along with a clock
  // ticking once a minute, filtered here so the track isn't refetched every
  // minute. A custom timeframe is fixed.
  const [fetchedAt, setFetchedAt] = useState(() => Date.now());
  const [now, setNow] = useState(fetchedAt);
  const isCustom = trackSpan === 'custom';
  const rangeSince = isCustom ? fromLocalInput(trackRange.from) : null;
  // The inputs have minute precision; To includes all of its minute.
  const rangeTo = isCustom ? fromLocalInput(trackRange.to, 59_999) : null;
  // Not fetched while the end is before the start; the field shows why.
  const rangeInvalid = rangeSince !== null && rangeTo !== null && rangeTo < rangeSince;
  const since = isCustom
    ? rangeSince
    : trackSpan === null
      ? null
      : new Date(fetchedAt - trackSpan * MINUTE_MS).toISOString();
  const { data: history, error: trackError } = useUnitPositions(
    rangeInvalid ? null : (trackUnit?.id ?? null),
    since,
    rangeTo,
  );
  useEffect(() => {
    if (trackId === null || trackSpan === null || trackSpan === 'custom') return;
    const timer = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(timer);
  }, [trackId, trackSpan]);
  const track = useMemo(() => {
    if (trackSpan === null || trackSpan === 'custom') return history;
    const cutoff = now - trackSpan * MINUTE_MS;
    return history.filter((p) => Date.parse(p.timestamp) >= cutoff);
  }, [history, trackSpan, now]);

  const resetClock = () => {
    const time = Date.now();
    setFetchedAt(time);
    setNow(time);
  };

  const changeTrackSpan = (span: TrackSpan) => {
    if (span === 'custom') {
      // Starts from the timeframe shown so far, then the user adjusts it.
      const time = Date.now();
      const minutes = typeof trackSpan === 'number' ? trackSpan : DEFAULT_TRACK_SPAN;
      setTrackRange({ from: toLocalInput(time - minutes * MINUTE_MS), to: toLocalInput(time) });
    } else {
      resetClock();
      saveTrackSpan(span);
    }
    setTrackSpan(span);
  };

  const openMenu = (unit: PlacedUnit, e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setMenu({ unit, x: e.clientX, y: e.clientY });
  };

  const startMoving = () => {
    if (!menu) return;
    setMovingId(menu.unit.id);
    setMenu(null);
  };

  const toggleTrack = () => {
    if (!menu) return;
    resetClock();
    setTrackId(trackId === menu.unit.id ? null : menu.unit.id);
    setMenu(null);
  };

  // Menus render into the page rather than the body, so they still show
  // while the page is fullscreen.
  const overlayContainer = () => pageRef.current;

  return (
    <Paper
      ref={pageRef}
      className="map-page"
      style={{ '--map-symbol-height': `${symbolHeight}px` } as CSSProperties}
    >
      <ErrorBanner message={error ?? loadError ?? trackError} />
      <div ref={containerRef} className="map-page__map" />
      {map && styleLoaded && trackUnit && (
        // Remounted on a new timeframe, so the view fits the new track.
        <GpsTrack
          key={`${trackUnit.id}-${trackSpan}-${since}-${rangeTo}`}
          map={map}
          positions={track}
        />
      )}
      {map &&
        placed.map((u) => (
          <UnitMarker
            key={u.id}
            map={map}
            unit={u}
            symbolHeight={symbolHeight}
            onContextMenu={(e) => openMenu(u, e)}
          />
        ))}
      <Menu
        open={menu !== null}
        container={overlayContainer}
        onClose={() => setMenu(null)}
        anchorReference="anchorPosition"
        anchorPosition={menu ? { top: menu.y, left: menu.x } : undefined}
        // Right-clicking the backdrop closes the menu instead of opening the
        // browser's.
        slotProps={{
          root: {
            onContextMenu: (e: MouseEvent) => {
              e.preventDefault();
              setMenu(null);
            },
          },
        }}
      >
        <MenuItem onClick={startMoving}>
          <ListItemIcon>
            <PlaceIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{t('map.setPosition')}</ListItemText>
        </MenuItem>
        <MenuItem onClick={toggleTrack}>
          <ListItemIcon>
            <RouteIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>
            {menu?.unit.id === trackId ? t('map.hideHistory') : t('map.showHistory')}
          </ListItemText>
        </MenuItem>
      </Menu>
      <IconButton
        size="small"
        className="map-page__settings-button"
        aria-label={t('map.settings')}
        title={t('map.settings')}
        onClick={(e) => setSettingsAnchor(e.currentTarget)}
      >
        <TuneIcon fontSize="small" />
      </IconButton>
      <Popover
        open={settingsAnchor !== null}
        container={overlayContainer}
        anchorEl={settingsAnchor}
        onClose={() => setSettingsAnchor(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
      >
        <div className="map-page__settings">
          <Typography variant="body2" id="map-symbol-size">
            {t('map.symbolSize')}
          </Typography>
          <Slider
            size="small"
            aria-labelledby="map-symbol-size"
            min={MIN_SYMBOL_HEIGHT}
            max={MAX_SYMBOL_HEIGHT}
            step={4}
            value={symbolHeight}
            onChange={(_, value) => setSymbolHeight(value)}
            onChangeCommitted={(_, value) => saveSymbolHeight(value)}
          />
        </div>
      </Popover>
      {trackUnit && (
        <div className="map-page__hint map-page__hint--top">
          <Typography variant="body2">
            {t('map.historyOf', { name: trackUnit.name, count: track.length })}
          </Typography>
          <TextField
            select
            size="small"
            variant="standard"
            // Matches the text next to it.
            sx={{ '& .MuiInputBase-root': { typography: 'body2' } }}
            slotProps={{ select: { MenuProps: { container: overlayContainer } } }}
            aria-label={t('map.historySpan')}
            value={String(trackSpan)}
            onChange={(e) =>
              changeTrackSpan(
                TRACK_SPANS.find((s) => String(s) === e.target.value) ?? DEFAULT_TRACK_SPAN,
              )
            }
          >
            {TRACK_SPANS.map((span) => (
              <MenuItem key={String(span)} value={String(span)}>
                {span === 'custom'
                  ? t('map.spanCustom')
                  : span === null
                    ? t('map.spanAll')
                    : span < 60
                      ? t('map.spanMinutes', { count: span })
                      : span < 24 * 60
                        ? t('map.spanHours', { count: span / 60 })
                        : t('map.spanDays', { count: span / (24 * 60) })}
              </MenuItem>
            ))}
          </TextField>
          <Button size="small" onClick={() => setTrackId(null)}>
            {t('map.hideHistory')}
          </Button>
          {isCustom && (
            <div className="map-page__range">
              <TextField
                type="datetime-local"
                size="small"
                label={t('map.rangeFrom')}
                value={trackRange.from}
                onChange={(e) => setTrackRange({ ...trackRange, from: e.target.value })}
                slotProps={{ inputLabel: { shrink: true } }}
              />
              <TextField
                type="datetime-local"
                size="small"
                label={t('map.rangeTo')}
                value={trackRange.to}
                onChange={(e) => setTrackRange({ ...trackRange, to: e.target.value })}
                error={rangeInvalid}
                helperText={rangeInvalid ? t('map.rangeInvalid') : undefined}
                slotProps={{ inputLabel: { shrink: true } }}
              />
            </div>
          )}
        </div>
      )}
      {movingUnit && (
        <div className="map-page__hint">
          <Typography variant="body2">
            {t('map.pickPosition', { name: movingUnit.name })}
          </Typography>
          <Button size="small" onClick={() => setMovingId(null)}>
            {t('common.cancel')}
          </Button>
        </div>
      )}
      {!loading && placed.length === 0 && (
        <Typography variant="body2" color="text.secondary" className="map-page__empty">
          {t('map.empty')}
        </Typography>
      )}
    </Paper>
  );
}

/** Draws a unit's position history as a line through its measurements,
 * oldest to newest. The view fits the track once it has first loaded;
 * clicking a point shows its details in a popup. */
function GpsTrack({ map, positions }: { map: maplibregl.Map; positions: PositionHistoryEntry[] }) {
  const [selected, setSelected] = useState<PositionHistoryEntry | null>(null);
  const [popupContent] = useState(() => document.createElement('div'));
  // Closed by the click handler below instead, as its own would also close
  // the popup when another point is clicked.
  const [popup] = useState(
    () =>
      new maplibregl.Popup({
        className: 'map-page__popup',
        closeOnClick: false,
        offset: TRACK_POINT_RADIUS,
      }),
  );

  useEffect(() => {
    map.addSource(TRACK_SOURCE, {
      type: 'geojson',
      data: { type: 'FeatureCollection', features: [] },
    });
    map.addLayer({
      id: `${TRACK_SOURCE}-line`,
      type: 'line',
      source: TRACK_SOURCE,
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': TRACK_COLOR, 'line-width': 3, 'line-opacity': 0.8 },
    });
    map.addLayer({
      id: TRACK_POINTS_LAYER,
      type: 'circle',
      source: TRACK_SOURCE,
      filter: ['==', ['geometry-type'], 'Point'],
      paint: {
        'circle-radius': 4,
        'circle-color': '#fff',
        'circle-stroke-color': TRACK_COLOR,
        'circle-stroke-width': 2,
      },
    });
    return () => {
      map.removeLayer(TRACK_POINTS_LAYER);
      map.removeLayer(`${TRACK_SOURCE}-line`);
      map.removeSource(TRACK_SOURCE);
    };
  }, [map]);

  // The history comes newest first; sorted by measurement time so the line
  // follows the unit's actual path.
  const sorted = useMemo(
    () => positions.toSorted((a, b) => Date.parse(a.timestamp) - Date.parse(b.timestamp)),
    [positions],
  );
  const coordinates = useMemo(() => sorted.map((p) => [p.lon, p.lat]), [sorted]);

  useEffect(() => {
    map.getSource<maplibregl.GeoJSONSource>(TRACK_SOURCE)?.setData({
      type: 'FeatureCollection',
      features: [
        { type: 'Feature', properties: {}, geometry: { type: 'LineString', coordinates } },
        // The index into `sorted`, to find the entry of a clicked point.
        ...coordinates.map((c, index) => ({
          type: 'Feature' as const,
          properties: { index },
          geometry: { type: 'Point' as const, coordinates: c },
        })),
      ],
    });
  }, [map, coordinates]);

  // A click on a point selects it, anywhere else on the map closes the popup.
  // The hit area is a little larger than the point, for touch screens.
  useEffect(() => {
    const canvas = map.getCanvas();
    const pointAt = ({ x, y }: maplibregl.Point) => {
      const r = TRACK_POINT_RADIUS;
      const [feature] = map.queryRenderedFeatures(
        [
          [x - r, y - r],
          [x + r, y + r],
        ],
        { layers: [TRACK_POINTS_LAYER] },
      );
      const index: unknown = feature?.properties.index;
      return typeof index === 'number' ? (sorted[index] ?? null) : null;
    };
    const onClick = (e: maplibregl.MapMouseEvent) => setSelected(pointAt(e.point));
    // Leaves other cursors, like the crosshair while moving a unit, alone.
    let hovering = false;
    const onMouseMove = (e: maplibregl.MapMouseEvent) => {
      const over = pointAt(e.point) !== null;
      if (over === hovering) return;
      hovering = over;
      canvas.style.cursor = over ? 'pointer' : '';
    };
    map.on('click', onClick);
    map.on('mousemove', onMouseMove);
    return () => {
      map.off('click', onClick);
      map.off('mousemove', onMouseMove);
      if (hovering) canvas.style.cursor = '';
    };
  }, [map, sorted]);

  useEffect(() => {
    popup.setDOMContent(popupContent);
    const onClose = () => setSelected(null);
    popup.on('close', onClose);
    return () => {
      popup.off('close', onClose);
      popup.remove();
    };
  }, [popup, popupContent]);

  useEffect(() => {
    if (!selected) {
      popup.remove();
      return;
    }
    popup.setLngLat([selected.lon, selected.lat]);
    // Adding it again would first close it, clearing the selection.
    if (!popup.isOpen()) popup.addTo(map);
  }, [map, popup, selected]);

  const fitted = useRef(false);
  useEffect(() => {
    if (fitted.current || coordinates.length === 0) return;
    fitted.current = true;
    const bounds = new maplibregl.LngLatBounds();
    for (const c of coordinates) bounds.extend(c as [number, number]);
    // More room at the top, where the history bar covers the map.
    map.fitBounds(bounds, {
      padding: { top: 140, bottom: 60, left: 60, right: 60 },
      maxZoom: MAX_FIT_ZOOM,
    });
  }, [map, coordinates]);

  return selected && createPortal(<TrackPointPopupContent entry={selected} />, popupContent);
}

function TrackPointPopupContent({ entry }: { entry: PositionHistoryEntry }) {
  const { t, i18n } = useTranslation();
  const { lat, lon, height, timestamp, recordedAt, recordedBy } = entry;
  return (
    <>
      <strong>{fmtDate(timestamp, i18n.language)}</strong>
      <div>
        {lat.toFixed(5)}, {lon.toFixed(5)}
        {height !== null && ` · ${t('unitRow.height', { height })}`}
      </div>
      <div className="map-page__popup-time">
        {t('unitHistory.recorded')}: {fmtDate(recordedAt, i18n.language)}{' '}
        {t('unitRow.by', { name: recordedBy?.username ?? t('unitRow.deletedUser') })}
      </div>
    </>
  );
}

/** A MapLibre marker whose element and popup are rendered by React. */
function UnitMarker({
  map,
  unit,
  symbolHeight,
  onContextMenu,
}: {
  map: maplibregl.Map;
  unit: PlacedUnit;
  symbolHeight: number;
  onContextMenu: (e: MouseEvent) => void;
}) {
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
    popup.setOffset((src ? symbolHeight : symbolHeight * DOT_RATIO) / 2);
  }, [popup, src, symbolHeight]);

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
          <img
            className="map-page__symbol"
            src={src}
            alt={unit.name}
            title={unit.name}
            onContextMenu={onContextMenu}
          />
        ) : (
          <span className="map-page__dot" title={unit.name} onContextMenu={onContextMenu} />
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
