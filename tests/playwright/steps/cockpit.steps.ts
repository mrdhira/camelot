import AxeBuilder from "@axe-core/playwright";
import { expect } from "@playwright/test";
import { createBdd } from "playwright-bdd";

const { Given, When, Then } = createBdd();

Given("Dhira opens {string}", async ({ page }, path: string) => {
  await page.goto(path);
});

Given("Dhira opens {string} on a {int} px wide screen", async ({ page }, path: string, width: number) => {
  await page.setViewportSize({ width, height: 900 });
  await page.goto(path);
  await page.screenshot({ path: `test-results/visual/${width}${path.replaceAll("/", "_") || "_root"}.png`, fullPage: true, animations: "disabled" });
});

Then("he sees the heading {string}", async ({ page }, name: string) => {
  await expect(page.getByRole("heading", { name })).toBeVisible();
});

Then("he sees {string}", async ({ page }, text: string) => {
  await expect(page.getByText(text).first()).toBeVisible();
});

Then("the card no longer waits for his decision", async ({ page }) => {
  await expect(page.getByText("Waiting for your decision", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Mark as decided" })).toHaveCount(0);
});

Then("he sees the document {string} waiting for review", async ({ page }, id: string) => {
  const waiting = page.getByRole("region", { name: "Waiting for your decision" });
  await expect(waiting.getByRole("link", { name: new RegExp(id) })).toBeVisible();
});

Then("the stat {string} shows {string}", async ({ page }, label: string, value: string) => {
  const tile = page.locator("div[title]").filter({ has: page.getByText(label, { exact: true }) });
  await expect(tile.getByText(value, { exact: true })).toBeVisible();
});

When("he presses {string}", async ({ page }, name: string) => {
  await page.getByRole("button", { name }).click();
});

Then("the history of {string} shows {string} by {string}", async ({ page }, id: string, change: string, actor: string) => {
  await page.goto(`/tasks/${id}`);
  const row = page.getByRole("region", { name: "History" }).getByRole("row").filter({ hasText: change });
  await expect(row).toHaveCount(1);
  await expect(row.getByRole("cell").nth(1)).toHaveText(actor);
});

When("he drags card {string} to the column {string}", async ({ page }, id: string, column: string) => {
  await page.locator(`[data-card="${id}"]`).dragTo(page.locator(`[data-column="${column}"]`));
  await page.waitForURL("**/board**");
});

Then("card {string} is in the column {string}", async ({ page }, id: string, column: string) => {
  await expect(page.locator(`[data-column="${column}"] [data-card="${id}"]`)).toBeVisible();
});

When("he chooses {string} as the new status and presses {string}", async ({ page }, status: string, button: string) => {
  await page.getByLabel("New status").selectOption({ label: status });
  await page.getByRole("button", { name: button, exact: true }).click();
});

Then("the card shows the status {string}", async ({ page }, status: string) => {
  await expect(page.locator("main").getByText(status, { exact: true }).first()).toBeVisible();
  await expect(page.getByLabel("New status")).toHaveValue(/.+/);
  await expect(page.getByLabel("New status").locator("option:checked")).toHaveText(status);
});

When(
  "he fills {string} with {string} and {string} with {string} and presses {string}",
  async ({ page }, f1: string, v1: string, f2: string, v2: string, button: string) => {
    await page.getByLabel(f1).fill(v1);
    await page.getByLabel(f2).fill(v2);
    await page.getByRole("button", { name: button }).click();
  },
);

When(
  "he fills {string} with {string} and {string} with {string} and {string} with {string} and presses {string}",
  async ({ page }, f1: string, v1: string, f2: string, v2: string, f3: string, v3: string, button: string) => {
    await page.getByLabel(f1).fill(v1);
    await page.getByLabel(f2).fill(v2);
    await page.getByLabel(f3).fill(v3);
    await page.getByRole("button", { name: button }).click();
  },
);

Then("he is on the page of card {string}", async ({ page }, id: string) => {
  await expect(page).toHaveURL(new RegExp(`/tasks/${id}$`));
});

Then("he sees the chain {string} with {string} and {string}", async ({ page }, title: string, a: string, b: string) => {
  const list = page.getByRole("region", { name: title, exact: true });
  await expect(list.getByRole("link", { name: new RegExp(a) })).toBeVisible();
  await expect(list.getByRole("link", { name: new RegExp(b) })).toBeVisible();
});

Then("he sees the chain {string} with {string}", async ({ page }, title: string, a: string) => {
  await expect(page.getByRole("region", { name: title, exact: true }).getByRole("link", { name: new RegExp(a) })).toBeVisible();
});

When("he sets the review status to {string}", async ({ page }, status: string) => {
  await page.getByLabel("Status").selectOption({ label: status });
  await page.getByRole("button", { name: "Set" }).click();
});

Then("the document shows the status {string}", async ({ page }, status: string) => {
  await expect(page.getByLabel("Status").locator("option:checked")).toHaveText(status);
});

Then("he sees a table with {string}", async ({ page }, text: string) => {
  await expect(page.getByRole("cell", { name: text })).toBeVisible();
});

Then("the page has no horizontal scroll", async ({ page }) => {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});

Then("the page has no detectable accessibility violations", async ({ page }) => {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  expect(results.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`)).toEqual([]);
});
