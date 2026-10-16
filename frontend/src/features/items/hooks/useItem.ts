import { useQuery } from "@tanstack/react-query";
import { getItem } from "../services/items";
import { itemKeys } from "./useItems";

export function useItem(id: number) {
  return useQuery({
    queryKey: itemKeys.detail(id),
    queryFn: ({ signal }) => getItem(id, signal),
    enabled: Number.isInteger(id) && id >= 1 && id <= 2147483647,
  });
}
