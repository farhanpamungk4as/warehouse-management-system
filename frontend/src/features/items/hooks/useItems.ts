import { useQuery } from "@tanstack/react-query";
import type { ItemListParams } from "../types/item";
import { getItems } from "../services/items";

export const itemKeys = {
  lists: ["items", "list"] as const,
  list: (params: ItemListParams) => ["items", "list", params] as const,
  detail: (id: number) => ["items", "detail", id] as const,
};
export function useItems(params: ItemListParams) {
  return useQuery({
    queryKey: itemKeys.list(params),
    queryFn: ({ signal }) => getItems(params, signal),
  });
}
