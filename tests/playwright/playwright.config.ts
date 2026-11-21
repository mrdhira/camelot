// Needs bin/camelot (make build). The server runs on a fresh copy of testdata/vault (support/serve.sh).
import { defineConfig, devices } from "@playwright/test";
import { defineBddProject } from "playwright-bdd";

export const baseURL = "http://127.0.0.1:7711";

export default defineConfig({
  retries: 1,
  failOnFlakyTests: true,
  workers: 1,
  reporter: [["list"], ["html", { open: "never" }]],
  use: { baseURL, trace: "on-first-retry", screenshot: "only-on-failure" },
  webServer: {
    command: "bash support/serve.sh",
    url: baseURL + "/",
    reuseExistingServer: false,
    timeout: 30_000,
  },
  projects: [
    {
      ...defineBddProject({ name: "features", features: "features/**/*.feature", steps: "steps/**/*.ts", outputDir: ".features-gen" }),
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
