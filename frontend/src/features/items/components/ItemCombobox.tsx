import { useState } from "react";
import { Check, ChevronsUpDown } from "lucide-react";
import { useItems } from "../hooks/useItems";
import { useItem } from "../hooks/useItem";
import { useDebouncedValue } from "../../../hooks/useDebouncedValue";
import { Button } from "../../../components/ui/button";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "../../../components/ui/popover";
import {
  Command,
  CommandInput,
  CommandList,
  CommandItem,
} from "../../../components/ui/command";
export function ItemCombobox({
  value,
  onChange,
  disabled,
}: {
  value: number;
  onChange: (id: number) => void;
  disabled: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const q = useDebouncedValue(search, 300);
  const query = useItems({ q, category: "", page: 1, limit: 20 });
  const selected = useItem(value);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          id="item_id"
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          aria-label="Item"
          disabled={disabled}
          className="item-combobox w-full justify-between overflow-hidden"
        >
          <span className="truncate">
            {selected.data
              ? `${selected.data.data.sku} · ${selected.data.data.name}`
              : value
                ? selected.isPending
                  ? "Loading selected item…"
                  : "Selected item unavailable"
                : "Search and select an item"}
          </span>
          <ChevronsUpDown className="shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className="w-[var(--radix-popover-trigger-width)] p-0"
        align="start"
      >
        <Command shouldFilter={false}>
          <CommandInput
            value={search}
            onValueChange={setSearch}
            placeholder="Search SKU or name…"
          />
          <CommandList>
            {query.isPending ? (
              <p className="p-4 muted">Loading items…</p>
            ) : query.isError ? (
              <div className="p-4">
                <p className="field-error">{query.error.message}</p>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => void query.refetch()}
                >
                  Retry
                </Button>
              </div>
            ) : query.data.data.length === 0 ? (
              <p className="p-4 muted">No matching active items.</p>
            ) : (
              query.data.data.map((item) => (
                <CommandItem
                  key={item.id}
                  value={String(item.id)}
                  onSelect={() => {
                    onChange(item.id);
                    setOpen(false);
                  }}
                >
                  <Check
                    size={15}
                    className={value === item.id ? "opacity-100" : "opacity-0"}
                  />
                  <span>
                    <strong>{item.sku}</strong>
                    <span className="block muted">{item.name}</span>
                  </span>
                </CommandItem>
              ))
            )}
          </CommandList>
          <p className="p-3 text-xs text-muted-foreground border-t">
            Up to 20 matches. Refine your search to find an item.
          </p>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
