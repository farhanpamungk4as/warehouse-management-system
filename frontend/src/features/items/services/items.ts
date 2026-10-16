import { ApiError, apiRequest } from "../../../services/apiClient";
import type {
  Item,
  ItemListParams,
  ItemListResponse,
  ItemWrite,
} from "../types/item";

function assertItem(value: unknown): asserts value is Item {
  const item = value as Item | null;
  if (
    !item ||
    !Number.isInteger(item.id) ||
    item.id < 1 ||
    item.id > 2147483647 ||
    !["sku", "name", "category", "unit", "created_at"].every(
      (key) => typeof item[key as keyof Item] === "string",
    )
  ) {
    throw new ApiError(
      "The server returned invalid item data. Please try again.",
    );
  }
}

export async function getItems(
  params: ItemListParams,
  signal?: AbortSignal,
): Promise<ItemListResponse> {
  const query = new URLSearchParams({
    page: String(params.page),
    limit: String(params.limit),
  });
  if (params.category) query.set("category", params.category);
  if (params.q) query.set("q", params.q);
  const response = await apiRequest<Item[], ItemListResponse>(
    `/items?${query}`,
    { signal },
  );
  const { meta } = response;
  if (
    !Array.isArray(response.data) ||
    !meta ||
    !Number.isSafeInteger(meta.total) ||
    meta.total < 0 ||
    meta.page !== params.page ||
    meta.limit !== params.limit ||
    !Array.isArray(meta.categories) ||
    !meta.categories.every((category) => typeof category === "string")
  ) {
    throw new ApiError(
      "The server returned invalid list data. Please try again.",
    );
  }
  response.data.forEach(assertItem);
  return response;
}
export async function getItem(id: number, signal?: AbortSignal) {
  const response = await apiRequest<Item>(`/items/${id}`, { signal });
  assertItem(response.data);
  return response;
}
export async function createItem(input: ItemWrite) {
  const response = await apiRequest<Item>("/items", {
    method: "POST",
    body: JSON.stringify(input),
  });
  assertItem(response.data);
  return response;
}
export async function updateItem(id: number, input: ItemWrite) {
  const response = await apiRequest<Item>(`/items/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
  assertItem(response.data);
  return response;
}
