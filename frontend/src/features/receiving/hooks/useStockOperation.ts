import { useRef, useState } from "react";
import type { FormEvent } from "react";
import { useSearchParams } from "react-router-dom";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ApiError } from "../../../services/apiClient";
import { receiveStock, transferStock } from "../../../services/warehouse";
import type { Stock, Transfer } from "../../../services/warehouse";
import type { ApiSuccess } from "../../../types/api";
import { useLocations } from "../../locations/hooks/useLocations";
import { useStock } from "../../inventory/hooks/useStock";
import { useToast } from "../../../components/ui/ToastProvider";
const positive = (s: string) =>
  /^\d+$/.test(s) &&
  Number.isInteger(Number(s)) &&
  Number(s) > 0 &&
  Number(s) <= 2147483647;
export function useStockOperation(transfer: boolean) {
  const [search] = useSearchParams();
  const initial = search.get("item_id") ?? "";
  const [item, setItem] = useState(positive(initial) ? Number(initial) : 0);
  const [from, setFrom] = useState(
    search.get(transfer ? "from_location_id" : "location_id") ?? "",
  );
  const [to, setTo] = useState("");
  const [qty, setQty] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [failure, setFailure] = useState("");
  const [uncertain, setUncertain] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [result, setResult] = useState<Stock | Transfer | null>(null);
  const locked = useRef(false);
  const form = useRef<HTMLFormElement>(null);
  const client = useQueryClient();
  const toast = useToast();
  const locations = useLocations();
  const stock = useStock(item);
  const balance = (id: string) =>
    stock.data?.data.stocks.find((s) => s.location_id === Number(id))?.qty ?? 0;
  const mutation = useMutation({
    mutationFn: async (): Promise<ApiSuccess<Stock | Transfer>> =>
      transfer
        ? transferStock({
            item_id: item,
            from_location_id: Number(from),
            to_location_id: Number(to),
            qty: Number(qty),
          })
        : receiveStock({
            item_id: item,
            location_id: Number(from),
            qty: Number(qty),
          }),
    retry: false,
    onSuccess: (r) => {
      setResult(r.data);
      setQty("");
      setFailure("");
      setFields({});
      toast(r.message, "success");
      void client.invalidateQueries({ queryKey: ["stock", item] });
      void client.invalidateQueries({ queryKey: ["inventory"] });
      void client.invalidateQueries({ queryKey: ["dashboard"] });
    },
    onError: (e: Error) => {
      const errors: Record<string, string> = {};
      if (e instanceof ApiError)
        e.fields.forEach((f) => {
          errors[f.field] = f.reason;
        });
      setFields(errors);
      setFailure(e.message);
      setUncertain(
        !(e instanceof ApiError) || e.status === 0 || e.status >= 500,
      );
      toast(e.message, "error");
    },
  });
  const pending = mutation.isPending;
  function validate() {
    const errors: Record<string, string> = {};
    if (!item) errors.item_id = "Choose an active item.";
    if (!positive(from))
      errors[transfer ? "from_location_id" : "location_id"] =
        "Choose a location.";
    if (transfer && (!positive(to) || from === to))
      errors.to_location_id = "Choose a different destination.";
    if (!positive(qty))
      errors.qty = "Enter a positive integer no greater than 2,147,483,647.";
    setFields(errors);
    setFailure("");
    setResult(null);
    const first = Object.keys(errors)[0];
    if (first)
      form.current?.querySelector<HTMLElement>(`[id="${first}"]`)?.focus();
    return !first;
  }
  async function execute() {
    if (locked.current || pending || uncertain) return;
    locked.current = true;
    setConfirm(false);
    try {
      await mutation.mutateAsync();
    } catch {
      /* mutation owns feedback */
    } finally {
      locked.current = false;
    }
  }
  function submit(e: FormEvent) {
    e.preventDefault();
    if (pending || uncertain || !validate()) return;
    if (transfer) setConfirm(true);
    else void execute();
  }
  return {
    item,
    setItem,
    from,
    setFrom,
    to,
    setTo,
    qty,
    setQty,
    fields,
    setFields,
    failure,
    setFailure,
    uncertain,
    setUncertain,
    confirm,
    setConfirm,
    result,
    setResult,
    form,
    locations,
    stock,
    balance,
    pending,
    execute,
    submit,
  };
}
