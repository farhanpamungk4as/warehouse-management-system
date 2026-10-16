import { expect, test } from "@playwright/test";

test("shadcn selects, density, action menus and keyboard navigation search", async ({
  page,
  request,
}) => {
  const item = (await (await request.get("/api/v1/items?limit=10")).json())
    .data[0];
  expect(item).toBeTruthy();
  await page.goto("/items");
  await expect(
    page
      .locator("header")
      .getByRole("link", { name: "Create item", exact: true }),
  ).toHaveCount(0);
  await expect(
    page
      .locator('[aria-label="Item directory"]')
      .getByRole("link", { name: "Create item", exact: true }),
  ).toBeVisible();
  await expect(page.locator("tbody tr")).toHaveCount(10);
  const comfortable = (await page.locator("tbody tr").first().boundingBox())!
    .height;
  await page.getByRole("button", { name: "Display", exact: false }).click();
  await page
    .getByRole("menuitemradio", { name: "Compact", exact: true })
    .click();
  await expect(page.locator("[data-density=compact]")).toBeVisible();
  expect(
    (await page.locator("tbody tr").first().boundingBox())!.height,
  ).toBeLessThan(comfortable);
  await page
    .getByRole("combobox", { name: "Rows per page", exact: true })
    .focus();
  await page.keyboard.press("Space");
  await expect(page.getByRole("listbox")).toBeVisible();
  await page.getByRole("option", { name: "25", exact: true }).click();
  await expect(page.locator("tbody tr")).toHaveCount(25);
  await page
    .getByRole("button", { name: `Actions for ${item.name}`, exact: true })
    .click();
  await page
    .getByRole("menuitem", { name: "View details", exact: true })
    .click();
  await expect(page).toHaveURL(new RegExp(`/items/${item.id}$`));
  await page.keyboard.press("Control+k");
  await expect(page.getByRole("dialog", { name: "Find a page" })).toBeVisible();
  await page.getByPlaceholder("Search menus or modules…").fill("transfer");
  await page.getByRole("option", { name: /Stock transfer/ }).click();
  await expect(page).toHaveURL(/\/transfers$/);
  await expect(page.getByRole("dialog")).toHaveCount(0);
});
for (const width of [1440, 1280, 768, 390]) {
  test(`reference design and select popover within viewport at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 960 });
    await page.goto("/items");
    await expect(page.locator("tbody tr")).toHaveCount(10);
    expect(
      (await page.locator("tbody tr").first().boundingBox())!.height,
    ).toBeLessThan(85);
    await page.screenshot({
      path: `../docs/screenshots/items-${width}.png`,
      fullPage: true,
    });
    const category = page.getByRole("combobox", {
      name: "Filter by category",
      exact: true,
    });
    await category.click();
    const menu = page.getByRole("listbox");
    await expect(menu).toBeVisible();
    const box = await menu.boundingBox();
    expect(box!.height).toBeGreaterThan(100);
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
    await page.screenshot({
      path: `../docs/screenshots/items-dropdown-${width}.png`,
      fullPage: false,
    });
    await page.keyboard.press("Escape");
    await expect(category).toBeFocused();
    expect(await page.locator("select:visible").count()).toBe(0);
  });
}
