import { expect, test } from "@playwright/test";
test.describe.configure({ mode: "serial" });
const prefix = `ZZ-LOCATION-${Date.now()}`;
let locationID = 0;

test("create/edit location, validation, pending guard and fresh Redis-backed reads", async ({
  page,
  request,
}) => {
  await page.goto("/locations");
  await page
    .getByRole("button", { name: "Create location", exact: true })
    .click();
  const dialog = page.getByRole("dialog", {
    name: "Create location",
    exact: true,
  });
  await dialog
    .getByRole("button", { name: "Create location", exact: true })
    .click();
  await expect(dialog.getByLabel("Location code")).toBeFocused();
  await dialog.getByLabel("Location code").fill(prefix);
  await dialog.getByLabel("Zone", { exact: true }).fill("Demo");
  await dialog.getByLabel("Storage type").fill("bin");
  let release!: () => void;
  const gate = new Promise<void>((resolve) => (release = resolve));
  await page.route("**/api/v1/locations", async (route) => {
    if (route.request().method() === "POST") await gate;
    await route.continue();
  });
  const saved = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/v1/locations") && r.request().method() === "POST",
  );
  await dialog
    .getByRole("button", { name: "Create location", exact: true })
    .click();
  await expect(dialog.getByRole("button", { name: "Saving…" })).toBeDisabled();
  await expect(dialog.getByLabel("Location code")).toBeDisabled();
  release();
  const result = await saved;
  expect(result.status()).toBe(201);
  locationID = (await result.json()).data.id;
  await expect(dialog).toHaveCount(0);
  await expect(
    page.getByRole("heading", { name: prefix, exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Create location", exact: true })
    .click();
  await dialog.getByLabel("Location code").fill(prefix);
  await dialog.getByLabel("Zone", { exact: true }).fill("Demo");
  await dialog
    .getByRole("button", { name: "Create location", exact: true })
    .click();
  await expect(dialog.getByRole("alert")).toHaveText(
    "Location code already exists",
  );
  await expect(dialog.getByLabel("Location code")).toBeFocused();
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await page
    .getByRole("button", { name: `Edit ${prefix}`, exact: true })
    .click();
  const edit = page.getByRole("dialog", { name: "Edit location", exact: true });
  await edit.getByLabel("Location code").fill(`${prefix}-EDIT`);
  await edit.getByLabel("Zone", { exact: true }).fill("Updated");
  await edit.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(edit).toHaveCount(0);
  const list = (await (await request.get("/api/v1/locations")).json()).data;
  expect(list.find((v: { id: number }) => v.id === locationID)).toMatchObject({
    code: `${prefix}-EDIT`,
    zone: "Updated",
  });
});

test("location menus use CRUD data, delete removes unused option", async ({
  page,
  request,
}) => {
  await page.goto("/receiving");
  await page.getByRole("combobox", { name: "Location", exact: true }).click();
  await expect(
    page.getByRole("option", {
      name: `${prefix}-EDIT · Zone Updated`,
      exact: true,
    }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await page.goto("/inventory");
  await expect(
    page
      .locator("header")
      .getByRole("link", { name: "Receive stock", exact: true }),
  ).toHaveCount(0);
  await expect(
    page
      .locator('[data-slot="card"]')
      .getByRole("link", { name: "Receive stock", exact: true }),
  ).toBeVisible();
  await page.getByRole("combobox", { name: "Location", exact: true }).click();
  await expect(
    page.getByRole("option", { name: `${prefix}-EDIT`, exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await page.goto("/locations");
  await page
    .getByRole("button", { name: `Delete ${prefix}-EDIT`, exact: true })
    .click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "Cancel", exact: true })
    .click();
  expect((await request.get(`/api/v1/locations/${locationID}`)).status()).toBe(
    200,
  );
  await page
    .getByRole("button", { name: `Delete ${prefix}-EDIT`, exact: true })
    .click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "Delete location", exact: true })
    .click();
  await expect(page.getByRole("alertdialog")).toHaveCount(0);
  await expect(
    page.getByRole("heading", { name: `${prefix}-EDIT`, exact: true }),
  ).toHaveCount(0);
  expect((await request.get(`/api/v1/locations/${locationID}`)).status()).toBe(
    404,
  );
  await page.goto("/transfers");
  await page
    .getByRole("combobox", { name: "Source location", exact: true })
    .click();
  await expect(
    page.getByRole("option", {
      name: `${prefix}-EDIT · Zone Updated`,
      exact: true,
    }),
  ).toHaveCount(0);
});

test("deletion preserves a location with received stock", async ({
  page,
  request,
}) => {
  const location = (
    await (
      await request.post("/api/v1/locations", {
        data: { code: `${prefix}-USED`, zone: "Demo", type: "rack" },
      })
    ).json()
  ).data;
  const item = (
    await (
      await request.post("/api/v1/items", {
        data: {
          sku: `${prefix}-ITEM`,
          name: "Location stock guard",
          category: "Demo",
          unit: "pcs",
        },
      })
    ).json()
  ).data;
  expect(
    (
      await request.post("/api/v1/stock/receive", {
        data: { item_id: item.id, location_id: location.id, qty: 3 },
      })
    ).status(),
  ).toBe(200);
  await page.goto("/locations");
  await page
    .getByRole("button", { name: `Delete ${prefix}-USED`, exact: true })
    .click();
  const dialog = page.getByRole("alertdialog");
  await dialog
    .getByRole("button", { name: "Delete location", exact: true })
    .click();
  await expect(dialog.getByRole("alert")).toHaveText("Location is in use");
  expect((await request.get(`/api/v1/locations/${location.id}`)).status()).toBe(
    200,
  );
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
});

test("location cards and create dialog fit mobile and desktop", async ({
  page,
}) => {
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 960 });
    await page.goto("/locations");
    await page
      .getByRole("button", { name: "Create location", exact: true })
      .click();
    const dialog = page.getByRole("dialog", {
      name: "Create location",
      exact: true,
    });
    await expect(dialog).toBeVisible();
    const box = (await dialog.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(width);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
    await page.screenshot({
      path: `../docs/screenshots/location-create-${width}.png`,
    });
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await page.screenshot({
      path: `../docs/screenshots/locations-${width}.png`,
      fullPage: true,
    });
  }
});
