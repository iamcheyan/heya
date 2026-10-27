import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

export default defineConfig({
  plugins: [solid({ hot: false })],
  resolve: {
    conditions: ["development", "browser"],
  },
  optimizeDeps: {
    include: ["@pocketjs/framework/solid", "@pocketjs/framework/solid/components"],
  },
});
