import { Link, useParams, useNavigate } from "react-router-dom";
import { Pencil, ArrowDownToLine, ArrowRightLeft } from "lucide-react";
import { useItem } from "./hooks/useItem";
import { useStock } from "../inventory/hooks/useStock";
import { DeleteItem } from "./components/DeleteItem";
import { PageHeading } from "../../components/PageHeading";
import { StockTable } from "../../components/StockTable";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "../../components/ui/card";
import { Button } from "../../components/ui/button";
import { LoadingState } from "../../components/ui/LoadingState";
import { ErrorState } from "../../components/ui/ErrorState";
export function ItemDetailPage() {
  const raw = useParams().id ?? "";
  const id = /^\d+$/.test(raw) && Number(raw) <= 2147483647 ? Number(raw) : 0;
  const query = useItem(id);
  const stocks = useStock(id);
  const navigate = useNavigate();
  if (!id)
    return (
      <ErrorState
        title="Invalid item link"
        message="Choose an item from the directory."
      />
    );
  if (query.isPending) return <LoadingState />;
  if (query.isError)
    return (
      <ErrorState
        title="This item is unavailable"
        message={query.error.message}
        retry={() => void query.refetch()}
      />
    );
  const item = query.data.data;
  const total = stocks.data?.data.stocks.reduce(
    (sum, s) => sum + BigInt(s.qty),
    0n,
  );
  return (
    <>
      <PageHeading
        title={item.name}
        description={`${item.sku} · ${item.category}`}
      >
        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <Link to={`/items/${id}/edit`}>
              <Pencil />
              Edit item
            </Link>
          </Button>
          <DeleteItem item={item} onDeleted={() => navigate("/items")} />
        </div>
      </PageHeading>
      <div className="detail-grid">
        <Card>
          <CardHeader>
            <CardTitle>Item information</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="details">
              {[
                ["SKU", item.sku],
                ["Category", item.category],
                ["Unit", item.unit],
                ["Created", new Date(item.created_at).toLocaleString()],
              ].map(([k, v]) => (
                <div key={k}>
                  <dt>{k}</dt>
                  <dd>{v}</dd>
                </div>
              ))}
            </dl>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Total available quantity</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="metric-value">
              {total !== undefined ? total.toLocaleString() : "—"}{" "}
              <small>{item.unit}</small>
            </div>
            <p className="muted">Across all stored location balances</p>
            <div className="flex flex-wrap gap-2 mt-6">
              <Button asChild>
                <Link to={`/receiving?item_id=${id}`}>
                  <ArrowDownToLine />
                  Receive stock
                </Link>
              </Button>
              <Button variant="outline" asChild>
                <Link to={`/transfers?item_id=${id}`}>
                  <ArrowRightLeft />
                  Transfer stock
                </Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
      <Card className="mt-6">
        <CardHeader>
          <CardTitle>Stock by location</CardTitle>
        </CardHeader>
        <CardContent>
          {stocks.isPending ? (
            <LoadingState />
          ) : stocks.isError ? (
            <ErrorState
              message={stocks.error.message}
              retry={() => void stocks.refetch()}
            />
          ) : stocks.data.data.stocks.length ? (
            <StockTable stocks={stocks.data.data.stocks} />
          ) : (
            <p className="empty-note">
              This item has no stock yet. Receive a delivery to add a location
              balance.
            </p>
          )}
        </CardContent>
      </Card>
    </>
  );
}
