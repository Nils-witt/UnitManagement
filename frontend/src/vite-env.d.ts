/// <reference types="vite/client" />

interface ImportMetaEnv {
  // URL of a MapLibre style JSON that replaces the default OSM map style.
  readonly VITE_MAP_STYLE_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
