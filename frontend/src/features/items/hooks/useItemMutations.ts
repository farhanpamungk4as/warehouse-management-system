import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useToast } from "../../../components/ui/ToastProvider";
import { ApiError } from "../../../services/apiClient";
import { createItem, updateItem } from "../services/items";
import type { ItemWrite } from "../types/item";
import { itemKeys } from "./useItems";

export function useItemMutations() {
  const client = useQueryClient();
  const toast = useToast();
  const onError = (error: Error) =>
    toast(
      error instanceof ApiError
        ? error.message
        : "The item could not be saved. Please try again.",
      "error",
    );
  const onSuccess = (response: Awaited<ReturnType<typeof createItem>>) => {
    client.setQueryData(itemKeys.detail(response.data.id), response);
    // A refresh failure must not turn a committed write into a reported save failure.
    void client.invalidateQueries({ queryKey: itemKeys.lists });
    void client.invalidateQueries({ queryKey: ["dashboard"] });
    void client.invalidateQueries({ queryKey: ["inventory"] });
    toast(response.message, "success");
  };
  return {
    create: useMutation({
      mutationFn: createItem,
      retry: false,
      onSuccess,
      onError,
    }),
    update: useMutation({
      mutationFn: ({ id, input }: { id: number; input: ItemWrite }) =>
        updateItem(id, input),
      retry: false,
      onSuccess,
      onError,
    }),
  };
}
