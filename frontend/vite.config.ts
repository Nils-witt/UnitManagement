import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    rolldownOptions: {
      output: {
        // Stable, named vendor chunks: they change only when the dependency
        // does, so a redeploy of the app code leaves them cached in browsers.
        codeSplitting: {
          groups: [
            {
              name: 'react-vendor',
              test: /node_modules[\\/](react|react-dom|react-router|react-router-dom|scheduler)[\\/]/,
            },
            {
              name: 'mui-vendor',
              test: /node_modules[\\/](@mui[\\/](material|system|utils|styled-engine|private-theming|types)|@emotion)[\\/]/,
            },
            { name: 'query-vendor', test: /node_modules[\\/]@tanstack[\\/]/ },
          ],
        },
      },
    },
  },
  server: {
    // During `npm run dev`, forward API calls to the Go server.
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
});
