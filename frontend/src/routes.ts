// The Go server serves the app at the root. The router is declared from these
// segments (App.tsx) and every link and redirect uses the paths built from
// them, so a path can't drift between the two.
export const ROUTE_SEGMENTS = {
  login: 'login',
  users: 'users',
  groups: 'groups',
  units: 'units',
  map: 'map',
} as const;

export const ROUTES = {
  home: '/',
  login: `/${ROUTE_SEGMENTS.login}`,
  users: `/${ROUTE_SEGMENTS.users}`,
  groups: `/${ROUTE_SEGMENTS.groups}`,
  units: `/${ROUTE_SEGMENTS.units}`,
  map: `/${ROUTE_SEGMENTS.map}`,
} as const;
