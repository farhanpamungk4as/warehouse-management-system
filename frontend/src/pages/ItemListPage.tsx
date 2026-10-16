import { useEffect, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { Plus } from "lucide-react";
import type { TableDensity } from "../components/ui/TableDisplayMenu";
import { Card } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { LoadingState } from "../components/ui/LoadingState";
import { ErrorState } from "../components/ui/ErrorState";
import { Pagination } from "../components/ui/Pagination";
import { ItemFilters } from "../features/items/components/ItemFilters";
import { ItemTable } from "../features/items/components/ItemTable";
import { EmptyItems } from "../features/items/components/EmptyItems";
import { useItems } from "../features/items/hooks/useItems";
import { useItemFilters } from "../features/items/hooks/useItemFilters";

export function ItemListPage() {
  const [density, setDensity] = useState<TableDensity>("comfortable");
  const filters = useItemFilters();
  const query = useItems(filters.params);
  const { data } = query;
  const filtered = Boolean(filters.params.q || filters.params.category);
  const lastPage = data
    ? Math.max(1, Math.ceil(data.meta.total / filters.params.limit))
    : filters.params.page;
  useEffect(() => {
    document.title = "Items · Warehouse";
  }, []);
  if (data && filters.params.page > lastPage) {
    const query = new URLSearchParams({
      page: String(lastPage),
      limit: String(filters.params.limit),
    });
    if (filters.params.q) query.set("q", filters.params.q);
    if (filters.params.category) query.set("category", filters.params.category);
    return <Navigate replace to={`/items?${query}`} />;
  }
  return (
    <>
      <Card
        className="directory-card list-panel"
        data-density={density}
        aria-label="Item directory"
        aria-busy={query.isFetching}
      >
        <div className="directory-heading">
          <div className="flex items-center gap-3">
            <h1>Item list</h1>
            {data && (
              <span className="count-badge">
                {data.meta.total.toLocaleString()} records
              </span>
            )}
          </div>
          <div className="directory-actions">
            <span className="directory-status" role="status">
              {query.isFetching && !query.isPending ? (
                <>
                  <span className="spinner" />
                  Updating…
                </>
              ) : (
                <>
                  <span className="live-dot" />
                  Active directory
                </>
              )}
            </span>
            <Button asChild>
              <Link to="/items/new">
                <Plus />
                Create item
              </Link>
            </Button>
          </div>
        </div>
        <div className="category-tabs" aria-label="Quick category filters">
          <Button
            variant="outline"
            type="button"
            className={
              !filters.params.category
                ? "category-tab selected"
                : "category-tab"
            }
            onClick={() => filters.setCategory("")}
          >
            <span className="tab-dot" />
            All items
          </Button>
          {(data?.meta.categories ?? []).slice(0, 4).map((category) => (
            <Button
              variant="outline"
              type="button"
              key={category}
              title={category}
              className={
                filters.params.category === category
                  ? "category-tab selected"
                  : "category-tab"
              }
              onClick={() => filters.setCategory(category)}
            >
              <span className="truncate">{category}</span>
            </Button>
          ))}
        </div>
        <ItemFilters
          search={filters.search}
          category={filters.params.category}
          categories={data?.meta.categories ?? []}
          filtered={filtered || Boolean(filters.search)}
          onSearch={filters.setSearch}
          onCategory={filters.setCategory}
          onClear={filters.clear}
          refresh={() => void query.refetch()}
          fetching={query.isFetching}
          density={density}
          onDensity={setDensity}
        />
        {query.isPending ? (
          <LoadingState />
        ) : query.isError ? (
          <ErrorState
            message={query.error.message}
            retry={() => void query.refetch()}
          />
        ) : data && data.data.length === 0 ? (
          <EmptyItems filtered={filtered} clear={filters.clear} />
        ) : data ? (
          <ItemTable items={data.data} />
        ) : null}
        {data && !query.isError && (
          <Pagination
            page={filters.params.page}
            limit={filters.params.limit}
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
