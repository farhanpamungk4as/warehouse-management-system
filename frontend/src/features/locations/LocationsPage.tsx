import { useState } from "react";
import { Link } from "react-router-dom";
import { MapPin, ArrowUpRight, Plus, Pencil, Trash2 } from "lucide-react";
import type { Location } from "../../services/warehouse";
import { LocationFormDialog } from "./LocationFormDialog";
import { DeleteLocationDialog } from "./DeleteLocationDialog";
import { useLocations } from "./hooks/useLocations";
import { SelectField } from "../../components/ui/SelectField";
import { Card, CardContent } from "../../components/ui/card";
import { Input } from "../../components/ui/input";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { LoadingState } from "../../components/ui/LoadingState";
import { ErrorState } from "../../components/ui/ErrorState";
export function LocationsPage() {
  const query = useLocations();
  const [q, setQ] = useState("");
  const [zone, setZone] = useState("");
  const [editing, setEditing] = useState<Location | null | undefined>(
    undefined,
  );
  const [deleting, setDeleting] = useState<Location | null>(null);
  const locations = query.data?.data ?? [];
  const rows = locations.filter(
    (l) =>
      (!zone || l.zone === zone) &&
      [l.code, l.zone, l.type].some((v) =>
        v.toLowerCase().includes(q.toLowerCase()),
      ),
  );
  return (
    <>
      <Card
        className="directory-card list-panel"
        aria-label="Location directory"
      >
        <div className="directory-heading">
          <div className="flex items-center gap-3">
            <h1>Warehouse locations</h1>
            <span className="count-badge">{locations.length} locations</span>
          </div>
          <div className="directory-actions">
            <Button onClick={() => setEditing(null)}>
              <Plus />
              Create location
            </Button>
          </div>
        </div>
        <div className="panel-intro">
          Manage storage locations and explore their inventory.
        </div>
        <div className="location-filters">
          <Input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            aria-label="Search locations"
            placeholder="Search code, zone or type…"
            className="max-w-sm"
          />
          <SelectField
            label="Zone"
            value={zone}
            onChange={setZone}
            groupLabel="Storage zones"
            icon={<MapPin size={15} />}
            options={[
              { value: "", label: "All zones" },
              ...[...new Set(locations.map((location) => location.zone))]
                .sort()
                .map((value) => ({ value, label: `Zone ${value}` })),
            ]}
          />

          <span className="muted">{rows.length} locations</span>
        </div>
        {query.isPending ? (
          <LoadingState />
        ) : query.isError ? (
          <ErrorState
            message={query.error.message}
            retry={() => void query.refetch()}
          />
        ) : rows.length ? (
          <div className="locations-grid">
            {rows.map((l) => (
              <Card key={l.id}>
                <CardContent>
                  <div className="location-icon">
                    <MapPin size={24} />
                    <Badge variant="secondary">{l.type}</Badge>
                  </div>
                  <h2 className="location-code">{l.code}</h2>
                  <p className="muted">Zone {l.zone}</p>
                  <div className="mt-4 flex flex-wrap gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => setEditing(l)}
                      aria-label={`Edit ${l.code}`}
                    >
                      <Pencil />
                      Edit
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => setDeleting(l)}
                      aria-label={`Delete ${l.code}`}
                    >
                      <Trash2 />
                      Delete
                    </Button>
                  </div>
                  <Button
                    variant="outline"
                    className="mt-6 w-full justify-between"
                    asChild
                  >
                    <Link to={`/inventory?location_id=${l.id}`}>
                      View inventory
                      <ArrowUpRight />
                    </Link>
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        ) : (
          <Card>
            <CardContent className="empty-note">
              No locations match your filters.
            </CardContent>
          </Card>
        )}
      </Card>
      {editing !== undefined && (
        <LocationFormDialog
          location={editing}
          onClose={() => setEditing(undefined)}
        />
      )}
      {deleting && (
        <DeleteLocationDialog
          location={deleting}
          onClose={() => setDeleting(null)}
        />
      )}
    </>
  );
}
