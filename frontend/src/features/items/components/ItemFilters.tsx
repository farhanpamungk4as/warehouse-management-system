import { ListFilter, RotateCw, Search, X } from "lucide-react";
import { Input } from "../../../components/ui/input";
import { Button } from "../../../components/ui/button";
import { SelectField } from "../../../components/ui/SelectField";
import { TableDisplayMenu } from "../../../components/ui/TableDisplayMenu";
import type { TableDensity } from "../../../components/ui/TableDisplayMenu";
export function ItemFilters({
  search,
  category,
  categories,
  filtered,
  onSearch,
  onCategory,
  onClear,
  refresh,
  fetching,
  density,
  onDensity,
}: {
  search: string;
  category: string;
  categories: string[];
  filtered: boolean;
  onSearch: (value: string) => void;
  onCategory: (value: string) => void;
  onClear: () => void;
  refresh: () => void;
  fetching: boolean;
  density: TableDensity;
  onDensity: (value: TableDensity) => void;
}) {
  const options = [
    ...new Set([...categories, ...(category ? [category] : [])]),
  ].sort();
  return (
    <div className="list-toolbar">
      <div className="search-field">
        <Search size={16} />
        <label className="sr-only" htmlFor="item-search">
          Search items
        </label>
        <Input
          id="item-search"
          type="search"
          placeholder="Search by SKU or item name…"
          value={search}
          onChange={(event) => onSearch(event.target.value)}
          maxLength={400}
          autoComplete="off"
        />
      </div>
      <div className="toolbar-controls">
        <SelectField
          id="item-category"
          label="Filter by category"
          value={category}
          onChange={onCategory}
          icon={<ListFilter size={15} />}
          groupLabel="Filter by category"
          options={[
            { value: "", label: "All categories" },
            ...options.map((value) => ({ value, label: value })),
          ]}
        />
        <TableDisplayMenu value={density} onChange={onDensity} />
        <Button
          variant="outline"
          className="toolbar-button"
          onClick={refresh}
          disabled={fetching}
          aria-label="Refresh items"
        >
          <RotateCw className={fetching ? "animate-spin" : ""} />
          <span>Refresh</span>
        </Button>
        {filtered && (
          <Button
            variant="ghost"
            size="icon"
            aria-label="Clear filters"
            onClick={onClear}
          >
            <X size={16} />
          </Button>
        )}
      </div>
    </div>
  );
}
