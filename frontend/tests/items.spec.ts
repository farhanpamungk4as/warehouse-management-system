import { expect, test } from "@playwright/test";
import type { Page } from "@playwright/test";

async function chooseSelect(page: Page, label: string, value: string) {
  await page.getByRole("combobox", { name: label, exact: true }).click();
  await page.getByRole("option", { name: value, exact: true }).click();
}

// Run only against a dedicated disposable Compose project, never a retained DB.
test.describe.configure({ mode: "serial" });
const prefix = `BROWSER-${Date.now()}`;
let createdID = 0;
let createdSKU = `${prefix}-NEW`;
const createdCategory = `${prefix} Supplies`;
const seeds: {
  id: number;
  sku: string;
  name: string;
  category: string;
  unit: string;
}[] = [];

async function fillForm(
  page: Page,
  values: { sku: string; name: string; category: string; unit: string },
) {
  await page.getByLabel("SKU", { exact: false }).fill(values.sku);
  await page.getByLabel("Item name", { exact: false }).fill(values.name);
  await page.getByLabel("Category", { exact: false }).fill(values.category);
  await page.getByLabel("Unit", { exact: false }).fill(values.unit);
}

test("fresh empty dashboard, required fields and actual create with pending/success", async ({
  page,
  request,
}) => {
  const initial = await request.get("/api/v1/items");
  expect(initial.ok()).toBeTruthy();
  expect(
    (await initial.json()).meta.total,
    "E2E must target a disposable empty database",
  ).toBe(0);
  await page.goto("/items");
  await expect(
    page.getByRole("heading", { name: "Your item directory starts here" }),
  ).toBeVisible();
  await page.screenshot({
    path: "test-results/dashboard-empty.png",
    fullPage: true,
  });
  await page.getByRole("link", { name: "Create item", exact: true }).click();
  await page.getByRole("button", { name: "Create item", exact: true }).click();
  await expect(page.getByText("SKU is required.")).toBeVisible();
  await expect(page.getByLabel("SKU")).toBeFocused();
  await fillForm(page, {
    sku: createdSKU,
    name: "Shipping carton",
    category: createdCategory,
    unit: "pcs",
  });
  let release!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/v1/items", async (route) => {
    if (route.request().method() === "POST") await gate;
    await route.continue();
  });
  const saved = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().endsWith("/api/v1/items"),
  );
  await page.getByRole("button", { name: "Create item", exact: true }).click();
  await expect(page.getByRole("button", { name: "Saving…" })).toBeDisabled();
  await expect(page.getByLabel("SKU")).toBeDisabled();
  release();
  const response = await saved;
  expect(response.status()).toBe(201);
  createdID = (await response.json()).data.id;
  await expect(page).toHaveURL(/\/items$/);
  await expect(
    page.getByText("Item created successfully", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("row").filter({ hasText: createdSKU }),
  ).toBeVisible();
});

test("25+ real records, off-page search, complete categories, pagination and history", async ({
  page,
  request,
}) => {
  for (let i = 1; i <= 25; i++) {
    const values = {
      sku: `${prefix}-${String(i).padStart(2, "0")}`,
      name:
        i === 25
          ? "Off-page needle"
          : `Warehouse item ${String(i).padStart(2, "0")}`,
      category:
        i === 25
          ? `${prefix} Z-last`
          : i % 2 === 0
            ? `${prefix} Packaging`
            : `${prefix} Tools`,
      unit: "pcs",
    };
    const response = await request.post("/api/v1/items", { data: values });
    expect(response.status()).toBe(201);
    seeds.push((await response.json()).data);
  }
  await page.goto("/items");
  await expect(page.getByRole("row")).toHaveCount(11);
  await page.getByRole("combobox", { name: "Filter by category" }).click();
  await expect(
    page.getByRole("option", { name: `${prefix} Z-last`, exact: true }),
  ).toHaveCount(1);
  await page.keyboard.press("Escape");
  const first = await page.locator("tbody tr").first().innerText();
  await page.getByRole("button", { name: "Next page" }).click();
  await expect(page).toHaveURL(/page=2/);
  await expect(page.locator("tbody tr").first()).not.toHaveText(first);
  await expect(page.locator(".page-buttons")).toContainText("Page 2 of 3");
  await page.getByRole("searchbox", { name: "Search items" }).fill("needle");
  await expect(page).toHaveURL(/q=needle/);
  await expect(page).not.toHaveURL(/page=2/);
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await expect(
    page.getByText("Off-page needle", { exact: true }),
  ).toBeVisible();
  await chooseSelect(page, "Filter by category", `${prefix} Tools`);
  await expect(
    page.getByRole("heading", { name: "No matching items" }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Clear filters", exact: true })
    .first()
    .click();
  await chooseSelect(page, "Filter by category", `${prefix} Packaging`);
  await expect(page.locator("tbody tr")).toHaveCount(10);
  await expect(page.locator(".results-range")).toContainText("of 12 items");
  await page.getByRole("button", { name: "Next page" }).click();
  await expect(page.locator("tbody tr")).toHaveCount(2);
  await page.goBack();
  await expect(page.locator(".page-buttons")).toContainText("Page 1 of 2");
  await page.getByRole("searchbox").fill("pending search must be canceled");
  await page.goBack();
  await expect(page.getByRole("searchbox")).toHaveValue("");
  await page.waitForTimeout(350); // Deliberately exceed debounce to detect stale publication after history navigation.
  await expect(page).not.toHaveURL(/pending/);
  await page.goto("/items?page=999");
  await expect(page).toHaveURL(/page=3/);
  await expect(page.locator("tbody tr")).toHaveCount(6);
});

test("actual edit, direct SPA reload, unchanged draft on refetch and field persistence", async ({
  page,
  request,
}) => {
  const original = (
    await (await request.get(`/api/v1/items/${createdID}`)).json()
  ).data;
  await page.goto(`/items/${createdID}/edit`);
  await expect(page.getByLabel("SKU")).toHaveValue(createdSKU);
  await page.reload();
  await expect(page.getByLabel("Item name")).toHaveValue("Shipping carton");
  await page.getByLabel("Item name").fill("Unsaved carton draft");
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  await expect(page.getByLabel("Item name")).toHaveValue(
    "Unsaved carton draft",
  );
  createdSKU = `${prefix}-UPDATED`;
  await fillForm(page, {
    sku: createdSKU,
    name: "Large shipping carton",
    category: `${prefix} Updated`,
    unit: "box",
  });
  const saved = page.waitForResponse(
    (response) => response.request().method() === "PUT",
  );
  await page.getByRole("button", { name: "Save changes" }).click();
  expect((await saved).status()).toBe(200);
  await expect(
    page.getByText("Item updated successfully", { exact: true }),
  ).toBeVisible();
  await page.getByRole("searchbox").fill(createdSKU);
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await expect(
    page.getByText("Large shipping carton", { exact: true }),
  ).toBeVisible();
  const updated = (
    await (await request.get(`/api/v1/items/${createdID}`)).json()
  ).data;
  expect(updated).toMatchObject({
    id: createdID,
    sku: createdSKU,
    name: "Large shipping carton",
    category: `${prefix} Updated`,
    unit: "box",
    created_at: original.created_at,
  });
});

test("duplicate SKU shows API toast and field error while preserving user input", async ({
  page,
}) => {
  await page.goto("/items/new");
  await fillForm(page, {
    sku: createdSKU,
    name: "Keep this value",
    category: "My category",
    unit: "pcs",
  });
  await page.getByRole("button", { name: "Create item", exact: true }).click();
  await expect(
    page.locator("[data-sonner-toast][data-type=error]"),
  ).toHaveCount(1);
  await expect(
    page.locator("[data-sonner-toast][data-type=error]"),
  ).toContainText("SKU already exists");
  await expect(
    page.getByText("Duplicate entry", { exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel("Item name")).toHaveValue("Keep this value");
  await expect(page.getByLabel("SKU")).toBeFocused();
  await expect(
    page.getByRole("button", { name: "Create item", exact: true }),
  ).toBeEnabled();
});

test("loading, safe error with retry, and filtered empty states", async ({
  page,
}) => {
  let release!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/v1/items?**", async (route) => {
    await gate;
    await route.continue();
  });
  await page.goto("/items");
  await expect(
    page.getByRole("status", { name: "Loading items" }),
  ).toBeVisible();
  release();
  await expect(page.locator("tbody tr")).toHaveCount(10);
  await page.unrouteAll({ behavior: "wait" });
  await page.route("**/api/v1/items?**", (route) =>
    route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({
        success: false,
        message: "Service temporarily unavailable",
        errors: [
          { field: "request", reason: "Operation could not be completed" },
        ],
      }),
    }),
  );
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "We couldn’t load your items" }),
  ).toBeVisible();
  await expect(page.getByRole("alert")).toContainText(
    "Service temporarily unavailable",
  );
  await page.unrouteAll({ behavior: "wait" });
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.locator("tbody tr")).toHaveCount(10);
  await page.getByRole("searchbox").fill("there-is-no-such-item-928521");
  await expect(
    page.getByRole("heading", { name: "No matching items" }),
  ).toBeVisible();
});

test("invalid and deleted edit routes are handled safely", async ({
  page,
  request,
}) => {
  await page.goto("/items/not-a-number/edit");
  await expect(
    page.getByRole("heading", { name: "Invalid item link" }),
  ).toBeVisible();
  await page.goto("/items/2147483647/edit");
  await expect(
    page.getByRole("heading", { name: "This item is unavailable" }),
  ).toBeVisible();
  const deleted = seeds[0];
  expect((await request.delete(`/api/v1/items/${deleted.id}`)).status()).toBe(
    200,
  );
  await page.goto(`/items/${deleted.id}/edit`);
  await expect(
    page.getByRole("heading", { name: "This item is unavailable" }),
  ).toBeVisible();
});

for (const width of [1280, 768]) {
  test(`desktop/tablet layout and keyboard access at ${width}px`, async ({
    page,
  }) => {
    const exceptions: string[] = [];
    page.on("pageerror", (error) => exceptions.push(error.message));
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/items");
    await expect(page.locator("tbody tr")).toHaveCount(10);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBeTruthy();
    await page.keyboard.press("Tab");
    await expect(
      page.getByRole("link", { name: "Skip to content" }),
    ).toBeFocused();
    await page.keyboard.press("Enter");
    await page.getByRole("searchbox").focus();
    await expect(page.getByRole("searchbox")).toBeFocused();
    await page.screenshot({
      path: `test-results/dashboard-${width}.png`,
      fullPage: true,
    });
    await page.goto(`/items/${createdID}/edit`);
    await expect(page.getByLabel("Item name")).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBeTruthy();
    await page.screenshot({
      path: `test-results/form-${width}.png`,
      fullPage: true,
    });
    expect(exceptions).toEqual([]);
  });
}
