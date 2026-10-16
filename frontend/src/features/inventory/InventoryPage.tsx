import { useState } from "react";
import { Link, Navigate, useSearchParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowDownToLine,
  ArrowRightLeft,
  Search,
  ListFilter,
  MapPin,
  RotateCw,
  X,
} from "lucide-react";
import { getInventory } from "../../services/warehouse";
import { useLocations } from "../locations/hooks/useLocations";
import { useItemFilters } from "../items/hooks/useItemFilters";
import { SelectField } from "../../components/ui/SelectField";
import { TableDisplayMenu } from "../../components/ui/TableDisplayMenu";
import type { TableDensity } from "../../components/ui/TableDisplayMenu";
import { Card } from "../../components/ui/card";
import { Input } from "../../components/ui/input";
import { Button } from "../../components/ui/button";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "../../components/ui/table";
import { Pagination } from "../../components/ui/Pagination";
import { LoadingState } from "../../components/ui/LoadingState";
import { ErrorState } from "../../components/ui/ErrorState";
export function InventoryPage() {
  const [density, setDensity] = useState<TableDensity>("comfortable");
  const filters = useItemFilters(["location_id"]);
  const [search, setSearchParams] = useSearchParams();
  const rawLocation = search.get("location_id") ?? "";
  const locationID =
    /^\d+$/.test(rawLocation) &&
    Number(rawLocation) > 0 &&
    Number(rawLocation) <= 2147483647
      ? rawLocation
      : "";
  const params = { ...filters.params, location_id: locationID };
  const { page, limit } = params;
  const query = useQuery({
    queryKey: ["inventory", params],
    queryFn: ({ signal }) => getInventory(params, signal),
  });
  const locations = useLocations();
  const data = query.data;
  if (data && page > Math.max(1, Math.ceil(data.meta.total / limit))) {
    const next = new URLSearchParams(search);
    next.set("page", String(Math.max(1, Math.ceil(data.meta.total / limit))));
    return <Navigate replace to={`/inventory?${next}`} />;
  }

  return (
    <>
      <Card className="directory-card list-panel" data-density={density}>
        <div className="directory-heading">
          <div className="flex items-center gap-3">
            <h1>Stock inventory</h1>
            {data && (
              <span className="count-badge">{data.meta.total} records</span>
            )}
          </div>
          <div className="directory-actions">
            <span className="directory-status">
              <span className="live-dot" />
              Current balances
            </span>
            <Button asChild>
              <Link to="/receiving">
                <ArrowDownToLine />
                Receive stock
              </Link>
            </Button>
          </div>
        </div>
        <div className="panel-intro">
          <span>Monitor stock across every warehouse location.</span>
          <span>Active items only</span>
        </div>
        <div className="list-toolbar">
          <div className="search-field">
            <Search size={16} />
            <Input
              aria-label="Search inventory"
              placeholder="Search SKU or item name…"
              value={filters.search}
              onChange={(event) => filters.setSearch(event.target.value)}
            />
          </div>
          <div className="toolbar-controls">
            <SelectField
              label="Category"
              value={params.category}
              onChange={filters.setCategory}
              groupLabel="Item categories"
              icon={<ListFilter size={15} />}
              options={[
                { value: "", label: "All categories" },
                ...[
                  ...new Set([
                    ...(data?.meta.categories ?? []),
                    ...(params.category ? [params.category] : []),
                  ]),
                ].map((value) => ({ value, label: value })),
              ]}
            />
            <SelectField
              label="Location"
              value={params.location_id}
              groupLabel="Warehouse locations"
              icon={<MapPin size={15} />}
              onChange={(value) =>
                setSearchParams((previous) => {
                  const next = new URLSearchParams(previous);
                  value
                    ? next.set("location_id", value)
                    : next.delete("location_id");
                  const q = filters.search.replace(/^ +| +$/g, "");
                  q ? next.set("q", q) : next.delete("q");
                  next.delete("page");
                  return next;
                })
              }
              options={[
                { value: "", label: "All locations" },
                ...(locations.data?.data ?? []).map((location) => ({
                  value: String(location.id),
                  label: location.code,
                })),
              ]}
            />
            <TableDisplayMenu value={density} onChange={setDensity} />
            <Button
              variant="outline"
              className="toolbar-button"
              onClick={() => void query.refetch()}
              disabled={query.isFetching}
              aria-label="Refresh inventory"
            >
              <RotateCw className={query.isFetching ? "animate-spin" : ""} />
              <span>Refresh</span>
            </Button>
            {(params.q || params.category || params.location_id) && (
              <Button
                variant="ghost"
                size="icon"
                aria-label="Clear inventory filters"
                onClick={() => {
                  filters.setSearch("");
                  setSearchParams({});
                }}
              >
                <X size={16} />
              </Button>
            )}
          </div>
        </div>
        {locations.isError && (
          <p role="alert" className="field-error px-6">
            Location filter unavailable.{" "}
            <Button
              variant="link"
              size="sm"
              className="h-auto p-0"
              onClick={() => void locations.refetch()}
            >
              Retry
            </Button>
          </p>
        )}
        {query.isPending ? (
          <LoadingState />
        ) : query.isError ? (
          <ErrorState
            message={query.error.message}
            retry={() => void query.refetch()}
          />
        ) : data && data.data.length ? (
          <Table className="inventory-table">
            <TableHeader>
              <TableRow>
                {[
                  "SKU / Item",
                  "Category",
                  "Location",
                  "Zone",
                  "Quantity",
                  "Updated",
                  "Actions",
                ].map((c) => (
                  <TableHead key={c}>{c}</TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.data.map((v) => (
                <TableRow key={v.id}>
                  <TableCell>
                    <Link className="inventory-item" to={`/items/${v.item.id}`}>
                      <span className="sku">{v.item.sku}</span>
                      <strong>{v.item.name}</strong>
                    </Link>
                  </TableCell>
                  <TableCell>{v.item.category}</TableCell>
                  <TableCell>{v.location.code}</TableCell>
                  <TableCell>{v.location.zone}</TableCell>
                  <TableCell className="font-semibold tabular-nums">
                    {v.qty.toLocaleString()}{" "}
                    <span className="muted">{v.item.unit}</span>
                  </TableCell>
                  <TableCell>
                    {new Date(v.updated_at).toLocaleString()}
                  </TableCell>
                  <TableCell>
                    <div className="flex gap-1">
                      <Button size="icon-sm" variant="ghost" asChild>
                        <Link
                          aria-label={`Receive ${v.item.name}`}
                          to={`/receiving?item_id=${v.item.id}&location_id=${v.location.id}`}
                        >
                          <ArrowDownToLine size={15} />
                        </Link>
                      </Button>
                      <Button size="icon-sm" variant="ghost" asChild>
                        <Link
                          aria-label={`Transfer ${v.item.name}`}
                          to={`/transfers?item_id=${v.item.id}&from_location_id=${v.location.id}`}
                        >
                          <ArrowRightLeft size={15} />
                        </Link>
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <div className="empty-note">
            No matching stock balances. Try another filter or receive stock.
          </div>
        )}
        {data && !query.isError && (
          <Pagination
            page={page}
            limit={limit}
            total={data.meta.total}
            disabled={query.isFetching}
            onPage={filters.setPage}
            onLimit={filters.setLimit}
          />
        )}
      </Card>
    </>
  );
}
