import { execFileSync } from "node:child_process";
import path from "node:path";
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
  const dark = await page.evaluate(() => matchMedia("(prefers-color-scheme: dark)").matches);
  await page.screenshot({ path: `test-results/visual/${dark ? "dark-" : ""}${width}${path.replaceAll("/", "_") || "_root"}.png`, fullPage: true, animations: "disabled" });
});

Given("Dhira's system is set to dark", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
});

// background is the page's background colour as the browser computed it.
const background = (page: import("@playwright/test").Page) => page.evaluate(() => getComputedStyle(document.body).backgroundColor);

Then("the page is dark", async ({ page }) => {
  await expect.poll(() => background(page)).toBe("rgb(8, 11, 16)");
});

// light is the light theme's background (oklch(1 0 0), white).
const light = /^(rgb\(255, 255, 255\)|oklch\(1 0 0\))$/;

Then("the page is light", async ({ page }) => {
  await expect.poll(() => background(page)).toMatch(light);
});

// themeSwitch is the header's light/dark switch.
const themeSwitch = (page: import("@playwright/test").Page) => page.getByRole("switch", { name: "Dark theme" });

Then("the theme switch shows {string}", async ({ page }, name: string) => {
  await expect(themeSwitch(page)).toHaveAttribute("aria-checked", String(name === "Dark"));
});

When("he flips the theme switch", async ({ page }) => {
  await themeSwitch(page).click();
});

When("he flips the theme switch with the keyboard", async ({ page }) => {
  await themeSwitch(page).focus();
  await page.keyboard.press("Space");
});

Then("after a reload the page is still light", async ({ page }) => {
  await page.reload();
  await expect.poll(() => background(page)).toMatch(light);
  await expect(themeSwitch(page)).toHaveAttribute("aria-checked", "false");
});

When("he comments {string}", async ({ page }, text: string) => {
  await page.getByLabel("Add a comment").fill(text);
  await page.getByRole("button", { name: "Comment", exact: true }).click();
  await expect(page.getByRole("listitem").filter({ hasText: text })).toBeVisible();
});

When("he comments {string} about {string}", async ({ page }, text: string, section: string) => {
  await page.getByLabel("Add a comment").fill(text);
  await page.getByLabel("About", { exact: true }).selectOption({ label: section });
  await page.getByRole("button", { name: "Comment", exact: true }).click();
});

Then("he sees the comment {string} about {string}", async ({ page }, text: string, section: string) => {
  const item = page.getByRole("listitem").filter({ hasText: text });
  await expect(item.getByRole("link", { name: section })).toBeVisible();
});

When("the agent {string} comments {string} on {string}", async ({}, actor: string, text: string, id: string) => {
  const root = path.resolve(__dirname, "..");
  execFileSync(path.resolve(root, "../../bin/camelot"), ["--log-level", "warn", "comment", "add", "--kind", "card", "--id", id, "--body", text], {
    env: { ...process.env, CAMELOT_VAULT: path.join(root, ".vault"), CAMELOT_ACTOR: actor },
  });
});

Then("{string} appears without a reload", async ({ page }, text: string) => {
  await page.evaluate(() => ((window as any).__noReload = true));
  await expect(page.getByText(text, { exact: true })).toBeVisible({ timeout: 20_000 });
  expect(await page.evaluate(() => (window as any).__noReload)).toBe(true);
});

Then("on the board, card {string} shows {string} and {string}", async ({ page }, id: string, a: string, b: string) => {
  await page.goto("/board");
  const tile = page.locator(`[data-card="${id}"]`);
  await expect(tile).toContainText(a);
  await expect(tile).toContainText(b);
});

Then("card {string} shows {string} as {string}", async ({ page }, id: string, text: string, tone: string) => {
  await expect(page.locator(`[data-card="${id}"] [data-tone="${tone}"]`, { hasText: text })).toBeVisible();
});

Then(
  "the column {string} is marked {string}, {string} is marked {string} and {string} is marked {string}",
  async ({ page }, a: string, ta: string, b: string, tb: string, c: string, tc: string) => {
    for (const [column, tone] of [[a, ta], [b, tb], [c, tc]]) {
      await expect(page.locator(`[data-column="${column}"] h2 [data-light]`)).toHaveClass(new RegExp(`bg-tone-${tone}`));
    }
  },
);

When("he opens {string} from the footer", async ({ page }, name: string) => {
  await page.getByRole("contentinfo").getByRole("link", { name }).click();
});

When("he resolves the comment {string}", async ({ page }, text: string) => {
  await page.getByRole("listitem").filter({ hasText: text }).getByRole("button", { name: /^Resolve/ }).click();
});

Then("he sees the heading {string}", async ({ page }, name: string) => {
  await expect(page.getByRole("heading", { name })).toBeVisible();
});

Then("he sees {string}", async ({ page }, text: string) => {
  await expect(page.getByText(text).first()).toBeVisible();
});

Then("he no longer sees {string}", async ({ page }, text: string) => {
  await expect(page.locator("main").getByText(text, { exact: true })).toHaveCount(0);
});

Then("the first heading is {string}", async ({ page }, name: string) => {
  await expect(page.locator("main h1").first()).toHaveText(name);
});

Then("the updates list {string} first", async ({ page }, id: string) => {
  await expect(page.getByRole("list", { name: "Updates, newest first" }).getByRole("listitem").first()).toContainText(id);
});

When("he answers {string} for {string}", async ({ page }, answer: string, id: string) => {
  await page.getByRole("group", { name: `Answer for ${id}` }).getByRole("button", { name: answer, exact: true }).click();
  await page.waitForLoadState("load");
});

When("he writes the answer {string} for {string}", async ({ page }, answer: string, id: string) => {
  const group = page.getByRole("group", { name: `Answer for ${id}` });
  await group.getByLabel(`Your answer to ${id}`).fill(answer);
  await group.getByLabel(`Your answer to ${id}`).press("Enter");
  await page.waitForLoadState("load");
});

When("he approves the document {string}", async ({ page }, id: string) => {
  const card = page.locator("#decisions [data-slot=card]").filter({ hasText: id });
  await card.getByRole("button", { name: "Approve", exact: true }).click();
  await page.waitForLoadState("load");
});

When("he opens the stat {string}", async ({ page }, label: string) => {
  await page.getByRole("link", { name: label, exact: true }).click();
});

When("he opens card {string} in the panel", async ({ page }, id: string) => {
  await page.locator(`[data-card="${id}"] a[data-panel]`).click();
});

Then("the panel shows {string}", async ({ page }, text: string) => {
  await expect(page.locator("[data-panel-body]")).toContainText(text);
});

When("the agent {string} moves {string} to {string}", async ({}, actor: string, id: string, status: string) => {
  const root = path.resolve(__dirname, "..");
  execFileSync(path.resolve(root, "../../bin/camelot"), ["--log-level", "warn", "task", "move", "--id", id, "--status", status], {
    env: { ...process.env, CAMELOT_VAULT: path.join(root, ".vault"), CAMELOT_ACTOR: actor },
  });
});

Then("card {string} appears in the column {string} without a reload", async ({ page }, id: string, column: string) => {
  await page.evaluate(() => ((window as any).__noReload = true));
  await expect(page.locator(`[data-column="${column}"] [data-card="${id}"]`)).toBeVisible({ timeout: 20_000 });
  expect(await page.evaluate(() => (window as any).__noReload)).toBe(true);
  await expect(page.locator(`[data-card="${id}"]`)).toHaveClass(/is-fresh/);
});

When("he presses Command-K and types {string}", async ({ page }, text: string) => {
  await page.keyboard.press("ControlOrMeta+k");
  await expect(page.getByRole("combobox", { name: "Search cards, documents and pages" })).toBeFocused();
  await page.keyboard.type(text);
});

When("he types {string} into {string} and clicks away", async ({ page }, text: string, label: string) => {
  await page.getByLabel(label, { exact: true }).fill(text);
  await page.locator("main h1").click();
});

Then("after the next refresh {string} still holds {string}", async ({ page }, label: string, text: string) => {
  await page.waitForTimeout(12_000);
  await expect(page.getByLabel(label, { exact: true })).toHaveValue(text);
});

When("he presses Enter", async ({ page }) => {
  await page.keyboard.press("Enter");
});

Then("the contents list {string} and {string}", async ({ page }, a: string, b: string) => {
  const toc = page.getByRole("navigation", { name: "On this page" });
  await expect(toc.getByRole("link", { name: a })).toBeVisible();
  await expect(toc.getByRole("link", { name: b })).toBeVisible();
});

When("he chooses {string} as another stage", async ({ page }, status: string) => {
  await page.getByText("Another stage").click();
  await page.getByLabel("New status").selectOption({ label: status });
  await page.getByRole("button", { name: "Move", exact: true }).click();
});

Then("he sees the document {string} waiting for review", async ({ page }, id: string) => {
  await expect(page.locator("#decisions").getByRole("link", { name: new RegExp(id) })).toBeVisible();
});

Then("the stat {string} shows {string}", async ({ page }, label: string, value: string) => {
  const tile = page.getByRole("region", { name: "Numbers" }).locator("div.relative").filter({ has: page.getByRole("link", { name: label, exact: true }) });
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
  await page.waitForLoadState("load");
  await page.waitForURL("**/board**");
});

Then("card {string} is in the column {string}", async ({ page }, id: string, column: string) => {
  await expect(page.locator(`[data-column="${column}"] [data-card="${id}"]`)).toBeVisible();
});

Then("the card shows the status {string}", async ({ page }, status: string) => {
  await expect(page.locator("main").getByText(status, { exact: true }).first()).toBeVisible();
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
