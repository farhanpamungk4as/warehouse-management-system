import { ApiError, apiRequest } from "./apiClient";
import type { ApiSuccess } from "../types/api";
import type {
  Item,
  ItemListMeta,
  ItemListParams,
} from "../features/items/types/item";
export interface Location {
  id: number;
  code: string;
  zone: string;
  type: string;
}
export type LocationWrite = Omit<Location, "id">;
export interface Stock {
  id: number;
  item_id: number;
  location_id: number;
  qty: number;
  updated_at: string;
}
export interface Balance extends Stock {
  location: Location;
}
export interface StockLookup {
  item_id: number;
  stocks: Balance[];
}
export interface Summary {
  active_items: number;
  available_quantity: number;
  locations: number;
  occupied_stock_pairs: number;
  distribution: { location_id: number; code: string; qty: number }[];
}
export interface InventoryRow {
  id: number;
  item: Item;
  location: Location;
  qty: number;
  updated_at: string;
}
export interface InventoryParams extends ItemListParams {
  location_id: string;
}
export interface InventoryResponse extends ApiSuccess<InventoryRow[]> {
  meta: ItemListMeta;
}
export interface ReceiveInput {
  item_id: number;
  location_id: number;
  qty: number;
}
export interface TransferInput {
  item_id: number;
  from_location_id: number;
  to_location_id: number;
  qty: number;
}
export interface Transfer {
  item_id: number;
  from_location_id: number;
  to_location_id: number;
  qty: number;
  source: Stock;
  destination: Stock;
}
const object = (v: unknown): v is Record<string, unknown> =>
  !!v && typeof v === "object";
const nonnegative = (v: unknown): v is number =>
  Number.isSafeInteger(v) && Number(v) >= 0;
const id = (v: unknown): v is number =>
  nonnegative(v) && v > 0 && v <= 2147483647;
const string = (v: unknown) => typeof v === "string";
const location = (v: unknown): v is Location =>
  object(v) && id(v.id) && ["code", "zone", "type"].every((k) => string(v[k]));
const stock = (v: unknown): v is Stock =>
  object(v) &&
  id(v.id) &&
  id(v.item_id) &&
  id(v.location_id) &&
  nonnegative(v.qty) &&
  Number(v.qty) <= 2147483647 &&
  string(v.updated_at);
const item = (v: unknown): v is Item =>
  object(v) &&
  id(v.id) &&
  ["sku", "name", "category", "unit", "created_at"].every((k) => string(v[k]));
function ensure(value: unknown, valid: boolean): asserts value {
  void value;
  if (!valid) throw new ApiError("The server returned invalid warehouse data.");
}
export async function getLocations(signal?: AbortSignal) {
  const r = await apiRequest<Location[]>("/locations", { signal });
  ensure(r.data, Array.isArray(r.data) && r.data.every(location));
  return r;
}
export async function saveLocation(input: LocationWrite, locationID?: number) {
  const r = await apiRequest<Location>(
    locationID ? `/locations/${locationID}` : "/locations",
    {
      method: locationID ? "PUT" : "POST",
      body: JSON.stringify(input),
    },
  );
  ensure(r.data, location(r.data));
  return r;
}
export async function deleteLocation(locationID: number) {
  const r = await apiRequest<Location>(`/locations/${locationID}`, {
    method: "DELETE",
  });
  ensure(r.data, location(r.data) && r.data.id === locationID);
  return r;
}
export async function getSummary(signal?: AbortSignal) {
  const r = await apiRequest<Summary>("/dashboard/summary", { signal });
  const v = r.data;
  ensure(
    v,
    object(v) &&
      [
        "active_items",
        "available_quantity",
        "locations",
        "occupied_stock_pairs",
      ].every((k) => nonnegative(v[k as keyof Summary])) &&
      Array.isArray(v.distribution) &&
      v.distribution.every(
        (d) =>
          object(d) &&
          id(d.location_id) &&
          string(d.code) &&
          nonnegative(d.qty),
      ),
  );
  return r;
}
export async function getStock(itemID: number, signal?: AbortSignal) {
  const r = await apiRequest<StockLookup>(`/stock/${itemID}`, { signal });
  ensure(
    r.data,
    object(r.data) &&
      r.data.item_id === itemID &&
      Array.isArray(r.data.stocks) &&
      r.data.stocks.every((s) => stock(s) && location(s.location)),
  );
  return r;
}
export async function getInventory(
  params: InventoryParams,
  signal?: AbortSignal,
) {
  const qs = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== "") qs.set(k, String(v));
  });
  const r = await apiRequest<InventoryRow[], InventoryResponse>(
    `/inventory/overview?${qs}`,
    { signal },
  );
  ensure(
    r.data,
    Array.isArray(r.data) &&
      r.data.every(
        (v) =>
          object(v) &&
          id(v.id) &&
          item(v.item) &&
          location(v.location) &&
          nonnegative(v.qty) &&
          string(v.updated_at),
      ),
  );
  ensure(
    r.meta,
    object(r.meta) &&
      nonnegative(r.meta.total) &&
      r.meta.page === params.page &&
      r.meta.limit === params.limit &&
      Array.isArray(r.meta.categories) &&
      r.meta.categories.every(string),
  );
  return r;
}
export async function receiveStock(input: ReceiveInput) {
  const r = await apiRequest<Stock>("/stock/receive", {
    method: "POST",
    body: JSON.stringify(input),
  });
  ensure(r.data, stock(r.data));
  return r;
}
export async function transferStock(input: TransferInput) {
  const r = await apiRequest<Transfer>("/stock/transfer", {
    method: "POST",
    body: JSON.stringify(input),
  });
  ensure(
    r.data,
    object(r.data) &&
      id(r.data.item_id) &&
      id(r.data.from_location_id) &&
      id(r.data.to_location_id) &&
      id(r.data.qty) &&
      stock(r.data.source) &&
      stock(r.data.destination),
  );
  return r;
}
