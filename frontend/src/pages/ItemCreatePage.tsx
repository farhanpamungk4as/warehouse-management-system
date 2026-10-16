import { useEffect } from "react";
import { Link, useNavigate } from "react-router-dom";
import { PageHeading } from "../components/PageHeading";
import { Button } from "../components/ui/button";
import { Icon } from "../components/ui/Icon";
import { ItemForm } from "../features/items/components/ItemForm";
import { useItemMutations } from "../features/items/hooks/useItemMutations";

export function ItemCreatePage() {
  const { create } = useItemMutations();
  const navigate = useNavigate();
  useEffect(() => {
    document.title = "Create item · Warehouse";
  }, []);
  return (
    <>
      <Button variant="link" className="back-link h-auto px-0" asChild>
        <Link to="/items">
          <Icon name="back" size={17} />
          Back to items
        </Link>
      </Button>
      <PageHeading
        title="Create item"
        description="Add a product to your warehouse directory."
      />

      <ItemForm
        pending={create.isPending}
        onSubmit={async (input) => {
          await create.mutateAsync(input);
          navigate("/items");
        }}
      />
    </>
  );
}
