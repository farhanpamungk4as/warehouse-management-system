import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import {
  ArrowDownToLine,
  ArrowRight,
  ArrowRightLeft,
  Boxes,
  MapPin,
  Package,
  Layers,
} from "lucide-react";
import { getSummary } from "../../services/warehouse";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "../../components/ui/card";
import { Button } from "../../components/ui/button";
import { PageHeading } from "../../components/PageHeading";
import { LoadingState } from "../../components/ui/LoadingState";
import { ErrorState } from "../../components/ui/ErrorState";
export function DashboardPage() {
  const query = useQuery({
    queryKey: ["dashboard"],
    queryFn: ({ signal }) => getSummary(signal),
  });
  if (query.isPending) return <LoadingState />;
  if (query.isError)
    return (
      <ErrorState
        message={query.error.message}
        retry={() => void query.refetch()}
      />
    );
  const s = query.data.data;
  const metrics = [
    ["Active items", s.active_items, Package, "Products in your directory"],
    [
      "Available quantity",
      s.available_quantity,
      Boxes,
      "Units across active items",
    ],
    [
      "Warehouse locations",
      s.locations,
      MapPin,
      "Configured storage locations",
    ],
    [
      "Occupied stock pairs",
      s.occupied_stock_pairs,
      Layers,
      "Item / location pairs with stock",
    ],
  ] as const;
  const max = Math.max(1, ...s.distribution.map((d) => d.qty));
  return (
    <>
      <PageHeading
        title="Warehouse overview"
        description="A clear view of your inventory, right now."
      >
        <Button asChild>
          <Link to="/receiving">
            <ArrowDownToLine />
            Receive stock
          </Link>
        </Button>
      </PageHeading>
      <div className="metrics-grid">
        {metrics.map(([label, value, Icon, note]) => (
          <Card key={label} className="metric-card">
            <CardContent>
              <div className="metric-top">
                <span>{label}</span>
                <Icon size={18} />
              </div>
              <div className="metric-value" data-testid={label}>
                {value.toLocaleString()}
              </div>
              <p>{note}</p>
            </CardContent>
          </Card>
        ))}
      </div>
      <div className="dashboard-grid">
        <Card>
          <CardHeader>
            <CardTitle>Stock by location</CardTitle>
            <CardDescription>
              Current quantity for active items. Updated from live inventory.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {s.distribution.length === 0 ? (
              <p className="muted">No warehouse locations configured.</p>
            ) : (
              <div className="distribution">
                {s.distribution.map((d) => (
                  <Link
                    key={d.location_id}
                    to={`/inventory?location_id=${d.location_id}`}
                    className="distribution-row"
                  >
                    <div>
                      <span>{d.code}</span>
                      <strong>{d.qty.toLocaleString()}</strong>
                    </div>
                    <div className="bar-track">
                      <div style={{ width: `${(d.qty / max) * 100}%` }} />
                    </div>
                  </Link>
                ))}
              </div>
            )}
            {s.available_quantity === 0 && (
              <p className="muted mt-5">
                No available stock yet. Receive your first delivery to start.
              </p>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Keep things moving</CardTitle>
            <CardDescription>
              Your everyday warehouse workflows.
            </CardDescription>
          </CardHeader>
          <CardContent className="quick-actions">
            {[
              [
                Package,
                "Manage items",
                "Maintain your product directory",
                "/items",
              ],
              [
                ArrowDownToLine,
                "Receive stock",
                "Record an incoming delivery",
                "/receiving",
              ],
              [
                ArrowRightLeft,
                "Transfer stock",
                "Move units between locations",
                "/transfers",
              ],
            ].map(([I, title, note, to]) => {
              const Icon = I as typeof Package;
              return (
                <Button
                  asChild
                  variant="ghost"
                  className="h-auto justify-start whitespace-normal"
                  key={String(to)}
                >
                  <Link to={String(to)}>
                    <span className="quick-icon">
                      <Icon size={20} />
                    </span>
                    <div>
                      <strong>{String(title)}</strong>
                      <p>{String(note)}</p>
                    </div>
                    <ArrowRight size={16} />
                  </Link>
                </Button>
              );
            })}
          </CardContent>
        </Card>
      </div>
      <Card className="snapshot-card">
        <CardContent>
          <div className="snapshot-icon">
            <Boxes size={29} />
          </div>
          <div>
            <h2>Every unit. Every location.</h2>
            <p>
              Explore quantities and storage locations in a single inventory
              view.
            </p>
          </div>
          <Button variant="outline" asChild>
            <Link to="/inventory">
              View inventory
              <ArrowRight />
            </Link>
          </Button>
        </CardContent>
      </Card>
    </>
  );
}
