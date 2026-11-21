import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  base: "./",
  build: {
    outDir: "dist",
    emptyOutDir: false,
  },
  server: {
    // The compose file of a node lives in deploy/ and the page reads it from there.
    fs: { allow: [".."] },
    proxy: {
      "/api": {
        target: "https://localhost:3443",
        secure: false,
      },
    },
  },
});
