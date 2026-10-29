import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The Go binary embeds internal/server/static (specification §7),
// so the production build lands there. Assets are content-hashed by
// Vite; script-src 'self' (specification §6) covers them.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../internal/server/static",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
});
