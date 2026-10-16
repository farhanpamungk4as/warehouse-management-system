import { useQuery } from "@tanstack/react-query";
import { getLocations } from "../../../services/warehouse";
export function useLocations() {
  return useQuery({
    queryKey: ["locations"],
    queryFn: ({ signal }) => getLocations(signal),
    // Recheck on navigation/focus so changes made through the API appear in menus.
    staleTime: 0,
  });
}
