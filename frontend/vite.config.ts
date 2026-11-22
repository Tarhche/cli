import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";
import {defineConfig} from "vitest/config";

// The app's bundle is read from the binary, never over a network, so its
// size is no reason to split it.
export default defineConfig({
  plugins: [react(), wails("./bindings")],
  build: {
    chunkSizeWarningLimit: 4096,
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    css: false,
    restoreMocks: true,
  },
});
