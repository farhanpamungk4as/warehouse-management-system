// Run against the disposable Compose project after stopping its Redis service.
import assert from "node:assert/strict";
import { chromium, request } from "@playwright/test";
const baseURL = process.env.E2E_BASE_URL || "http://127.0.0.1:3001";
const api = await request.newContext({ baseURL });
const browser = await chromium.launch({ channel: "chrome" });
let id;
try {
  const locations = await api.get("/api/v1/locations");
  assert.equal(locations.status(), 200);
  const location = (await locations.json()).data[0];
  assert(location);
  const page = await browser.newPage({
    baseURL,
    viewport: { width: 390, height: 900 },
  });
  await page.goto("/locations");
  await page
    .getByRole("heading", { name: location.code, exact: true })
    .waitFor();
  const created = await api.post("/api/v1/items", {
    data: {
      sku: `REDIS-DOWN-${Date.now()}`,
      name: "Redis outage verification",
      category: "Acceptance",
      unit: "pcs",
    },
  });
  assert.equal(created.status(), 201);
  id = (await created.json()).data.id;
  await page.goto(`/receiving?item_id=${id}&location_id=${location.id}`);
  await page
    .getByTestId("source-balance")
    .getByText("0", { exact: true })
    .waitFor();
  await page.getByLabel("Quantity", { exact: true }).fill("3");
  await page
    .getByRole("button", { name: "Receive stock", exact: true })
    .click();
  await page.getByText("Committed balance:", { exact: false }).waitFor();
  const stock = await api.get(`/api/v1/stock/${id}`);
  assert.equal((await stock.json()).data.stocks[0].qty, 3);
  console.log(
    "PASS: Redis stopped; location discovery + real Chrome receipt + PostgreSQL balance = 3",
  );
} finally {
  if (id) assert.equal((await api.delete(`/api/v1/items/${id}`)).status(), 200);
  await browser.close();
  await api.dispose();
}
