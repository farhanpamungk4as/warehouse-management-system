import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Trash2 } from "lucide-react";
import { apiRequest } from "../../../services/apiClient";
import type { Item } from "../types/item";
import { useToast } from "../../../components/ui/ToastProvider";
import { Button } from "../../../components/ui/button";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
} from "../../../components/ui/alert-dialog";
export function DeleteItem({
  item,
  onDeleted,
}: {
  item: Item;
  onDeleted?: () => void;
}) {
  const [open, setOpen] = useState(false);
  const client = useQueryClient();
  const toast = useToast();
  const mutation = useMutation({
    mutationFn: () => apiRequest(`/items/${item.id}`, { method: "DELETE" }),
    retry: false,
    onSuccess: (r) => {
      client.removeQueries({ queryKey: ["items", "detail", item.id] });
      void client.invalidateQueries({ queryKey: ["items", "list"] });
      void client.invalidateQueries({ queryKey: ["inventory"] });
      void client.invalidateQueries({ queryKey: ["dashboard"] });
      void client.invalidateQueries({ queryKey: ["stock", item.id] });
      toast(r.message, "success");
      setOpen(false);
      onDeleted?.();
    },
    onError: (e: Error) => toast(e.message, "error"),
  });
  return (
    <>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={`Delete ${item.name}`}
        onClick={() => setOpen(true)}
      >
        <Trash2 size={15} />
      </Button>
      <AlertDialog
        open={open}
        onOpenChange={(v) => {
          if (!mutation.isPending) setOpen(v);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {item.name}?</AlertDialogTitle>
            <AlertDialogDescription>
              This item will leave the active directory. Its stored record and
              stock balances are preserved. Its SKU remains reserved.
            </AlertDialogDescription>
          </AlertDialogHeader>
          {mutation.isError && (
            <p role="alert" className="field-error">
              {mutation.error.message}
            </p>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={mutation.isPending}>
              Cancel
            </AlertDialogCancel>
            <Button
              variant="destructive"
              disabled={mutation.isPending}
              onClick={() => mutation.mutate()}
            >
              {mutation.isPending ? "Deleting…" : "Delete item"}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
