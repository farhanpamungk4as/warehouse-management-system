import { useEffect } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Card } from "../components/ui/card";
import { PageHeading } from "../components/PageHeading";
import { Button } from "../components/ui/button";
import { Icon } from "../components/ui/Icon";
import { LoadingState } from "../components/ui/LoadingState";
import { ErrorState } from "../components/ui/ErrorState";
import { ItemForm } from "../features/items/components/ItemForm";
import { useItem } from "../features/items/hooks/useItem";
import { useItemMutations } from "../features/items/hooks/useItemMutations";
import { ApiError } from "../services/apiClient";

export function ItemEditPage() {
  const { id: rawID = "" } = useParams();
  const id = /^\d+$/.test(rawID) ? Number(rawID) : 0;
  const valid = Number.isInteger(id) && id >= 1 && id <= 2147483647;
  const query = useItem(valid ? id : 0);
  const { update } = useItemMutations();
  const navigate = useNavigate();
  useEffect(() => {
    document.title = "Edit item · Warehouse";
  }, []);
  if (!valid)
    return (
      <ErrorState
        title="Invalid item link"
        message="The item ID in this link is not valid. Return to the directory to choose an item."
      />
    );
  const missing =
    (query.error instanceof ApiError && query.error.status === 404) ||
    (update.error instanceof ApiError && update.error.status === 404);
  return (
    <>
      <Button variant="link" className="back-link h-auto px-0" asChild>
        <Link to="/items">
          <Icon name="back" size={17} />
          Back to items
        </Link>
      </Button>
      <PageHeading
        title="Edit item"
        description="Update the product details used across your warehouse."
      />

      {missing ? (
        <ErrorState
          title="This item is unavailable"
          message="It may have been removed. Return to the directory to choose another item."
        />
      ) : query.isPending ? (
        <Card className="form-card">
          <LoadingState form />
        </Card>
      ) : query.isError ? (
        <ErrorState
          message={query.error.message}
          retry={() => void query.refetch()}
        />
      ) : (
        <ItemForm
          key={id}
          initial={query.data.data}
          editing
          pending={update.isPending}
          onSubmit={async (input) => {
            await update.mutateAsync({ id, input });
            navigate("/items");
          }}
        />
      )}
    </>
  );
}
