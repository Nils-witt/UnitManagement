import { useEffect, useMemo, useRef, useState, type MouseEvent } from 'react';
import { createPortal } from 'react-dom';
import {
  Button,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Paper,
  Typography,
} from '@mui/material';
import PlaceIcon from '@mui/icons-material/Place';
import { useTranslation } from 'react-i18next';
import * as maplibregl from 'maplibre-gl';
import type { StyleSpecification } from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import type { Position, Unit } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';
import { useApi } from '../hooks/useApi';
import { useUnits } from '../hooks/useUnits';
import { errorMessage } from '../lib/errors';
import { fmtDate } from '../lib/format';
import { formatTacticalName } from '../lib/tacticalName';
import { symbolDataUrl } from '../lib/unitSymbol';
import './MapPage.scss';

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

// Symbols are 3:2, like in the unit list.
const SYMBOL_HEIGHT = 40;
const DOT_SIZE = 16;

export default function MapPage() {
  const { t } = useTranslation();
  const api = useApi();
  const { units, loading, error: loadError, reloadUnits } = useUnits();
  const containerRef = useRef<HTMLDivElement>(null);
  const [map, setMap] = useState<maplibregl.Map | null>(null);
  const [menu, setMenu] = useState<MenuState | null>(null);
  // The unit being moved: while set, the next click on the map sets its position.
  const [movingId, setMovingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

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

  return (
    <Paper className="map-page">
      <ErrorBanner message={error ?? loadError} />
      <div ref={containerRef} className="map-page__map" />
      {map &&
        placed.map((u) => (
          <UnitMarker key={u.id} map={map} unit={u} onContextMenu={(e) => openMenu(u, e)} />
        ))}
      <Menu
        open={menu !== null}
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
      </Menu>
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

/** A MapLibre marker whose element and popup are rendered by React. */
function UnitMarker({
  map,
  unit,
  onContextMenu,
}: {
  map: maplibregl.Map;
  unit: PlacedUnit;
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
