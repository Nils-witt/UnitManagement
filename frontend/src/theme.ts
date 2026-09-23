import { createTheme, type Theme } from '@mui/material/styles';

// `cssVariables` exposes the palette as `--mui-palette-*` custom properties,
// which is how the component .scss files read theme colors.
export function createAppTheme(mode: 'light' | 'dark'): Theme {
  return createTheme({
    cssVariables: true,
    palette: { mode },
    shape: { borderRadius: 8 },
  });
}
