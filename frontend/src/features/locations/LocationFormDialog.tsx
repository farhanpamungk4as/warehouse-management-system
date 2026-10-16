import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { saveLocation } from "../../services/warehouse";
import type { Location, LocationWrite } from "../../services/warehouse";
import { ApiError } from "../../services/apiClient";
import { useToast } from "../../components/ui/ToastProvider";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "../../components/ui/dialog";

export function LocationFormDialog({
  location,
  onClose,
}: {
  location: Location | null;
  onClose: () => void;
}) {
  const [values, setValues] = useState<LocationWrite>(
    location
      ? { code: location.code, zone: location.zone, type: location.type }
      : { code: "", zone: "", type: "rack" },
  );
  const [fields, setFields] = useState<Record<string, string>>({});
  const [failure, setFailure] = useState("");
  const form = useRef<HTMLFormElement>(null);
  const client = useQueryClient();
  const toast = useToast();
  const mutation = useMutation({
    mutationFn: (input: LocationWrite) => saveLocation(input, location?.id),
    retry: false,
    onSuccess: (r) => {
      client.setQueryData<{ data: Location[] }>(["locations"], (old) =>
        old
          ? {
              ...old,
              data: [
                ...old.data.filter((v) => v.id !== r.data.id),
                r.data,
              ].sort((a, b) => a.code.localeCompare(b.code)),
            }
          : undefined,
      );
      for (const key of ["locations", "dashboard", "inventory", "stock"])
        void client.invalidateQueries({ queryKey: [key] });
      toast(r.message, "success");
      onClose();
    },
  });
  useEffect(() => {
    if (!mutation.isPending && Object.keys(fields).length) {
      form.current
        ?.querySelector<HTMLInputElement>("[aria-invalid=true]")
        ?.focus();
    }
  }, [fields, mutation.isPending]);
  async function submit(e: FormEvent) {
    e.preventDefault();
    if (mutation.isPending) return;
    const input = Object.fromEntries(
      Object.entries(values)
        .filter(([key]) => ["code", "zone", "type"].includes(key))
        .map(([key, value]) => [key, value.replace(/^ +| +$/g, "")]),
    ) as LocationWrite;
    const errors: Record<string, string> = {};
    for (const [key, max] of [
      ["code", 64],
      ["zone", 64],
      ["type", 32],
    ] as const) {
      const v = input[key];
      if (
        /^[\u0009-\u000d\u0020\u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]*$/.test(
          v,
        )
      )
        errors[key] = "This field is required.";
      else if ([...v].length > max) errors[key] = `Maximum ${max} characters.`;
      else if (/[\u0000-\u001f\u007f]/u.test(v))
        errors[key] = "Control characters are not allowed.";
      else if (
        /[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/.test(
          v,
        )
      )
        errors[key] = "Enter valid Unicode text.";
    }
    setFields(errors);
    setFailure("");
    if (Object.keys(errors).length) return;
    try {
      await mutation.mutateAsync(input);
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Location could not be saved.";
      if (error instanceof ApiError)
        setFields(
          Object.fromEntries(error.fields.map((f) => [f.field, f.reason])),
        );
      setFailure(message);
      toast(message, "error");
    }
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) onClose();
      }}
    >
      <DialogContent showCloseButton={!mutation.isPending}>
        <DialogHeader>
          <DialogTitle>
            {location ? "Edit location" : "Create location"}
          </DialogTitle>
          <DialogDescription>
            Set the storage code, zone and type used in stock location menus.
          </DialogDescription>
        </DialogHeader>
        <form
          ref={form}
          noValidate
          aria-busy={mutation.isPending}
          onSubmit={(e) => void submit(e)}
          className="space-y-5"
        >
          {(
            [
              ["code", "Location code", "RACK-C1"],
              ["zone", "Zone", "C"],
              ["type", "Storage type", "rack"],
            ] as const
          ).map(([key, label, placeholder]) => (
            <div key={key} className="space-y-2">
              <label
                htmlFor={`location-${key}`}
                className="text-sm font-medium"
              >
                {label}
              </label>
              <Input
                id={`location-${key}`}
                value={values[key]}
                placeholder={placeholder}
                disabled={mutation.isPending}
                required
                aria-invalid={!!fields[key]}
                aria-describedby={
                  fields[key] ? `location-${key}-error` : undefined
                }
                onChange={(e) => {
                  setValues((v) => ({ ...v, [key]: e.target.value }));
                  setFields((v) => ({ ...v, [key]: "" }));
                  setFailure("");
                }}
              />
              {fields[key] && (
                <p id={`location-${key}-error`} className="field-error">
                  {fields[key]}
                </p>
              )}
            </div>
          ))}
          {failure && (
            <p role="alert" className="field-error">
              {failure}
            </p>
          )}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={mutation.isPending}
              onClick={onClose}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending
                ? "Saving…"
                : location
                  ? "Save changes"
                  : "Create location"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
