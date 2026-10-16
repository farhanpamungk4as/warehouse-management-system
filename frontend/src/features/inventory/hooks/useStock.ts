import { useQuery } from "@tanstack/react-query";
import { getStock } from "../../../services/warehouse";
export const stockKey = (id: number) => ["stock", id] as const;
export function useStock(id: number) {
  return useQuery({
    queryKey: stockKey(id),
    queryFn: ({ signal }) => getStock(id, signal),
    enabled: id > 0,
  });
}
