import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  base: "/",
  build: {
    outDir: "dist",
    emptyOutDir: false,
  },
  server: {
    proxy: {
      "/api": {
        target: "https://localhost:3443",
        secure: false,
      },
    },
  },
});
