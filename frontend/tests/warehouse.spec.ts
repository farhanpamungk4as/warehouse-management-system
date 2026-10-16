import { expect, test } from "@playwright/test";
import type { Page } from "@playwright/test";

async function chooseSelect(page: Page, label: string, value: string) {
  await page.getByRole("combobox", { name: label, exact: true }).click();
  await page.getByRole("option", { name: value, exact: true }).click();
}

test.describe.configure({ mode: "serial" });
let id = 0;
let source = 0;
let destination = 0;
let sourceCode = "";
let sourceLabel = "";
let destinationLabel = "";
const sku = `WMS-${Date.now()}`;
async function selectItem(page: Page) {
  await page.getByRole("combobox", { name: "Item", exact: true }).click();
  await page.getByPlaceholder("Search SKU or name…").fill(sku);
  await page.getByRole("option").filter({ hasText: sku }).click();
}
async function balances(request: import("@playwright/test").APIRequestContext) {
  const response = await request.get(`/api/v1/stock/${id}`);
  expect(response.status()).toBe(200);
  return (await response.json()).data.stocks as {
    location_id: number;
    qty: number;
  }[];
}

test("accurate dashboard, actual location discovery and all navigation", async ({
  page,
  request,
}) => {
  const response = await request.post("/api/v1/items", {
    data: {
      sku,
      name: "Precision packing kit",
      category: "Warehouse supplies",
      unit: "pcs",
    },
  });
  expect(response.status()).toBe(201);
  id = (await response.json()).data.id;
  const locations = (await (await request.get("/api/v1/locations")).json())
    .data;
  expect(locations.length).toBeGreaterThanOrEqual(2);
  source = locations[0].id;
  destination = locations[1].id;
  sourceCode = locations[0].code;
  sourceLabel = `${locations[0].code} · Zone ${locations[0].zone}`;
  destinationLabel = `${locations[1].code} · Zone ${locations[1].zone}`;
  await page.goto("/");
  await expect(page).toHaveURL(/\/dashboard$/);
  const summary = (
    await (await request.get("/api/v1/dashboard/summary")).json()
  ).data;
  for (const [label, value] of Object.entries({
    "Active items": summary.active_items,
    "Available quantity": summary.available_quantity,
    "Warehouse locations": summary.locations,
    "Occupied stock pairs": summary.occupied_stock_pairs,
  }))
    await expect(page.getByTestId(label)).toHaveText(
      Number(value).toLocaleString(),
    );
  for (const [label, path] of [
    ["Items", "/items"],
    ["Stock inventory", "/inventory"],
    ["Receive stock", "/receiving"],
    ["Stock transfer", "/transfers"],
    ["Locations", "/locations"],
    ["Dashboard", "/dashboard"],
  ]) {
    await page.getByRole("link", { name: label, exact: true }).first().click();
    await expect(page).toHaveURL(new RegExp(path + "$"));
  }
});
test("receive real stock, prevent duplicate pending submission, refresh detail and inventory", async ({
  page,
  request,
}) => {
  await page.goto("/receiving");
  await selectItem(page);
  await chooseSelect(page, "Location", sourceLabel);
  await expect(page.getByTestId("source-balance")).toHaveText("0");
  await page.getByLabel("Quantity", { exact: true }).fill("12");
  let release!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  let posts = 0;
  await page.route("**/api/v1/stock/receive", async (route) => {
    posts++;
    await gate;
    await route.continue();
  });
  await page
    .getByRole("button", { name: "Receive stock", exact: true })
    .click();
  await expect(page.getByRole("button", { name: "Saving…" })).toBeDisabled();
  await expect(page.getByLabel("Quantity", { exact: true })).toBeDisabled();
  release();
  await expect(
    page.getByText("Committed balance:", { exact: false }),
  ).toContainText("12");
  expect(posts).toBe(1);
  await page.getByRole("link", { name: "View updated inventory" }).click();
  await page.getByLabel("Search inventory").fill(sku);
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await expect(page.locator("tbody tr")).toContainText("12");
  await page
    .getByRole("link", { name: sku + " Precision packing kit" })
    .click();
  await expect(
    page.getByRole("heading", { name: "Precision packing kit", exact: true }),
  ).toBeVisible();
  await expect(page.locator("tbody tr")).toContainText(sourceCode);
  await expect(page.locator("tbody tr")).toContainText("12");
  const stored = await balances(request);
  expect(stored.reduce((s, v) => s + v.qty, 0)).toBe(12);
});
test("transfer confirmation, committed balances, conservation and insufficient source", async ({
  page,
  request,
}) => {
  await page.goto(`/transfers?item_id=${id}&from_location_id=${source}`);
  await chooseSelect(page, "Destination location", destinationLabel);
  await expect(page.getByTestId("source-balance")).toHaveText("12");
  await expect(page.getByTestId("destination-balance")).toHaveText("0");
  await page
    .getByRole("combobox", { name: "Destination location", exact: true })
    .click();
  await expect(
    page.getByRole("option", { name: sourceLabel, exact: true }),
  ).toHaveAttribute("aria-disabled", "true");
  await page.keyboard.press("Escape");
  await page.getByLabel("Quantity", { exact: true }).fill("5");
  await page.getByRole("button", { name: "Review transfer" }).click();
  await expect(page.getByRole("alertdialog")).toBeVisible();
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  expect(
    (await balances(request)).find((v) => v.location_id === source)?.qty,
  ).toBe(12);
  await page.getByRole("button", { name: "Review transfer" }).click();
  await page
    .getByRole("button", { name: "Confirm transfer", exact: true })
    .click();
  await expect(
    page.getByText("Committed balances:", { exact: false }),
  ).toContainText("source 7, destination 5");
  let stored = await balances(request);
  expect(stored.find((v) => v.location_id === source)?.qty).toBe(7);
  expect(stored.find((v) => v.location_id === destination)?.qty).toBe(5);
  expect(stored.reduce((s, v) => s + v.qty, 0)).toBe(12);
  await page.getByLabel("Quantity", { exact: true }).fill("8");
  await page.getByRole("button", { name: "Review transfer" }).click();
  await page
    .getByRole("button", { name: "Confirm transfer", exact: true })
    .click();
  await expect(page.locator(".form-alert")).toContainText(
    "Insufficient source stock",
  );
  await expect(
    page.getByText("Source balance is less than requested quantity", {
      exact: true,
    }),
  ).toBeVisible();
  stored = await balances(request);
  expect(stored.reduce((s, v) => s + v.qty, 0)).toBe(12);
  expect(stored.find((v) => v.location_id === source)?.qty).toBe(7);
});
test("uncertain receipt actually commits once and requires balance reconciliation", async ({
  page,
  request,
}) => {
  await page.goto(`/receiving?item_id=${id}&location_id=${source}`);
  await expect(page.getByTestId("source-balance")).toHaveText("7");
  await page.getByLabel("Quantity", { exact: true }).fill("2");
  let posts = 0;
  await page.route("**/api/v1/stock/receive", async (route) => {
    posts++;
    const res = await route.fetch();
    expect(res.status()).toBe(200);
    await route.abort("failed");
  });
  await page
    .getByRole("button", { name: "Receive stock", exact: true })
    .click();
  await expect(
    page.getByText("The result may already be committed."),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Receive stock", exact: true }),
  ).toBeDisabled();
  expect(posts).toBe(1);
  expect(
    (await balances(request)).find((v) => v.location_id === source)?.qty,
  ).toBe(9);
  await page
    .getByRole("button", { name: "Refresh balances before retry" })
    .click();
  await expect(page.getByTestId("source-balance")).toHaveText("9");
  await expect(page.getByLabel("Quantity", { exact: true })).toHaveValue("");
  expect(posts).toBe(1);
});
test("location search and inventory filter, direct reload on every route", async ({
  page,
}) => {
  await page.goto("/locations");
  await page.getByLabel("Search locations").fill(sourceCode);
  await expect(
    page.getByRole("heading", { name: sourceCode, exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "View inventory", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`location_id=${source}`));
  await expect(
    page.getByRole("combobox", { name: "Location", exact: true }),
  ).toContainText(sourceCode);
  await expect(page.locator("tbody tr")).toHaveCount(1);
  for (const path of [
    "/dashboard",
    "/items",
    `/items/${id}`,
    `/items/${id}/edit`,
    "/inventory",
    "/receiving",
    "/transfers",
    "/locations",
  ]) {
    await page.goto(path);
    await page.reload();
    await expect(page.locator("h1")).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Page not found" }),
    ).toHaveCount(0);
  }
});
for (const width of [1440, 1280, 768, 390]) {
  test(`real rendering, drawer, dialogs, combobox and table scrolling at ${width}px`, async ({
    page,
  }) => {
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await page.setViewportSize({ width, height: 960 });
    await page.goto("/dashboard");
    await expect(page.getByTestId("Active items")).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
    await page.screenshot({
      path: `../docs/screenshots/dashboard-${width}.png`,
      fullPage: true,
    });
    await page.getByRole("button", { name: "Toggle navigation" }).click();
    if (width < 1024) {
      await expect(page.getByRole("dialog")).toBeVisible();
      await page
        .getByRole("link", { name: "Stock inventory", exact: true })
        .click();
      await expect(page.getByRole("dialog")).toHaveCount(0);
    } else {
      await expect(page.locator("[data-collapsible=icon]")).toBeVisible();
      await page.getByRole("button", { name: "Toggle navigation" }).click();
      await page
        .getByRole("link", { name: "Stock inventory", exact: true })
        .click();
    }
    await expect(page.locator("tbody tr")).toHaveCount(2);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
    if (width === 390) {
      const table = page.locator("[data-slot=table-container]");
      expect(
        await table.evaluate((el) => el.scrollWidth > el.clientWidth),
      ).toBeTruthy();
    }
    await page.screenshot({
      path: `../docs/screenshots/inventory-${width}.png`,
      fullPage: true,
    });
    await page.goto(`/transfers?item_id=${id}&from_location_id=${source}`);
    await page.getByRole("combobox", { name: "Item", exact: true }).click();
    await expect(page.getByPlaceholder("Search SKU or name…")).toBeFocused();
    await page.keyboard.press("Escape");
    await chooseSelect(page, "Destination location", destinationLabel);
    await page.getByLabel("Quantity", { exact: true }).fill("1");
    await page.getByRole("button", { name: "Review transfer" }).click();
    const dialog = page.getByRole("alertdialog");
    await expect(dialog).toBeVisible();
    const rect = await dialog.boundingBox();
    expect(rect!.x).toBeGreaterThanOrEqual(0);
    expect(rect!.x + rect!.width).toBeLessThanOrEqual(width);
    await page.screenshot({
      path: `../docs/screenshots/transfer-confirm-${width}.png`,
      fullPage: true,
    });
    await page.getByRole("button", { name: "Cancel", exact: true }).click();
    await page.getByLabel("Quantity", { exact: true }).focus();
    await expect(page.getByLabel("Quantity", { exact: true })).toBeFocused();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
    expect(errors).toEqual([]);
  });
}
test("render failure reaches the actual global error boundary", async ({
  page,
}) => {
  // Instrument only this browser response, leaving production files unchanged.
  let injected = false;
  await page.route("**/assets/*.js", async (route) => {
    const res = await route.fetch();
    let body = await res.text();
    if (body.includes('"Warehouse overview"')) {
      body = body.replace(
        '"Warehouse overview"',
        '(()=>{throw new Error("Deliberate browser render failure")})()',
      );
      injected = true;
    }
    await route.fulfill({ response: res, body });
  });
  await page.goto("/dashboard");
  await expect(
    page.getByRole("heading", { name: "Something went wrong" }),
  ).toBeVisible();
  expect(injected).toBeTruthy();
  await expect(
    page.getByRole("link", { name: "Return to dashboard" }),
  ).toBeVisible();
  await expect(page.getByText("Deliberate browser render failure")).toHaveCount(
    0,
  );
});
test("delete confirmation preserves records and rejects later receive and transfer", async ({
  page,
  request,
}) => {
  await page.goto(`/items/${id}`);
  await page
    .getByRole("button", { name: "Delete Precision packing kit" })
    .click();
  await expect(page.getByRole("alertdialog")).toContainText(
    "stock balances are preserved",
  );
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  expect((await request.get(`/api/v1/items/${id}`)).status()).toBe(200);
  await page
    .getByRole("button", { name: "Delete Precision packing kit" })
    .click();
  await page.getByRole("button", { name: "Delete item", exact: true }).click();
  await expect(page).toHaveURL(/\/items$/);
  expect((await request.get(`/api/v1/items/${id}`)).status()).toBe(404);
  expect(
    (
      await request.post("/api/v1/stock/receive", {
        data: { item_id: id, location_id: source, qty: 1 },
      })
    ).status(),
  ).toBe(404);
  expect(
    (
      await request.post("/api/v1/stock/transfer", {
        data: {
          item_id: id,
          from_location_id: source,
          to_location_id: destination,
          qty: 1,
        },
      })
    ).status(),
  ).toBe(404);
  await page.goto(`/receiving?item_id=${id}&location_id=${source}`);
  await expect(page.getByRole("alert")).toContainText("Item not found");
  await expect(
    page.getByRole("button", { name: "Receive stock", exact: true }),
  ).toBeDisabled();
});
