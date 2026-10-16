import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { deleteLocation } from "../../services/warehouse";
import type { Location } from "../../services/warehouse";
import { useToast } from "../../components/ui/ToastProvider";
import { Button } from "../../components/ui/button";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
} from "../../components/ui/alert-dialog";

export function DeleteLocationDialog({
  location,
  onClose,
}: {
  location: Location;
  onClose: () => void;
}) {
  const [failure, setFailure] = useState("");
  const client = useQueryClient();
  const toast = useToast();
  const mutation = useMutation({
    mutationFn: () => deleteLocation(location.id),
    retry: false,
    onSuccess: (r) => {
      client.setQueryData<{ data: Location[] }>(["locations"], (old) =>
        old
          ? { ...old, data: old.data.filter((v) => v.id !== location.id) }
          : undefined,
      );
      for (const key of ["locations", "dashboard", "inventory", "stock"])
        void client.invalidateQueries({ queryKey: [key] });
      toast(r.message, "success");
      onClose();
    },
    onError: (error: Error) => {
      setFailure(error.message);
      toast(error.message, "error");
    },
  });
  return (
    <AlertDialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) onClose();
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {location.code}?</AlertDialogTitle>
          <AlertDialogDescription>
            Only unused locations can be deleted. Locations with stock records,
            including zero balances, are kept to preserve inventory references.
          </AlertDialogDescription>
        </AlertDialogHeader>
        {failure && (
          <p className="field-error" role="alert">
            {failure}
          </p>
        )}
        <AlertDialogFooter>
          <Button
            variant="outline"
            onClick={onClose}
            disabled={mutation.isPending}
          >
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            {mutation.isPending ? "Deleting…" : "Delete location"}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
