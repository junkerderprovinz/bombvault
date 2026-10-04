import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The Android app's start screen. It ships inside the APK, which serves it
// through WebViewAssetLoader with this directory as the site root.
export default defineConfig({
  plugins: [react()],
  base: "/",
  build: {
    outDir: "../android/app/src/main/assets/launcher",
    emptyOutDir: true,
    rollupOptions: { input: "launcher.html" },
  },
});
