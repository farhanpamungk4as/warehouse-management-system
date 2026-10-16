import { afterEach, expect, it, vi } from "vitest";
import {
  getLocations,
  getSummary,
  getStock,
  getInventory,
  transferStock,
} from "./warehouse";
afterEach(() => vi.unstubAllGlobals());
const success = (data: unknown, meta?: unknown) =>
  new Response(JSON.stringify({ success: true, message: "ok", data, meta }), {
    status: 200,
  });
it("rejects malformed data inside successful warehouse envelopes", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockImplementation(async () => success({})),
  );
  await expect(getLocations()).rejects.toThrow("invalid warehouse data");
  await expect(getSummary()).rejects.toThrow("invalid warehouse data");
  await expect(getStock(1)).rejects.toThrow("invalid warehouse data");
  await expect(
    getInventory({ page: 1, limit: 10, q: "", category: "", location_id: "" }),
  ).rejects.toThrow("invalid warehouse data");
});
it("accepts empty summary and empty stock arrays", async () => {
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(
      success({
        active_items: 0,
        available_quantity: 0,
        locations: 0,
        occupied_stock_pairs: 0,
        distribution: [],
      }),
    )
    .mockResolvedValueOnce(success({ item_id: 1, stocks: [] }));
  vi.stubGlobal("fetch", fetch);
  expect((await getSummary()).data.available_quantity).toBe(0);
  expect((await getStock(1)).data.stocks).toEqual([]);
});
it("sends transfer only once on an uncertain network failure", async () => {
  const fetch = vi.fn().mockRejectedValue(new TypeError("network"));
  vi.stubGlobal("fetch", fetch);
  await expect(
    transferStock({
      item_id: 1,
      from_location_id: 1,
      to_location_id: 2,
      qty: 1,
    }),
  ).rejects.toThrow("Unable to reach");
  expect(fetch).toHaveBeenCalledTimes(1);
});
