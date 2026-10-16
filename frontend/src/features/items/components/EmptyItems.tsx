import { Link } from "react-router-dom";
import { Card } from "../../../components/ui/card";
import { Button } from "../../../components/ui/button";
import { Icon } from "../../../components/ui/Icon";

export function EmptyItems({
  filtered,
  clear,
}: {
  filtered: boolean;
  clear: () => void;
}) {
  return (
    <Card className="state-panel">
      <div className="empty-box">
        <Icon name={filtered ? "search" : "box"} size={32} />
      </div>
      <h2>
        {filtered ? "No matching items" : "Your item directory starts here"}
      </h2>
      <p>
        {filtered
          ? "Try a different SKU, item name, or category to find what you need."
          : "Add your first item to keep warehouse details organized in one place."}
      </p>
      {filtered ? (
        <Button variant="outline" onClick={clear}>
          Clear filters
        </Button>
      ) : (
        <Button asChild>
          <Link to="/items/new">
            <Icon name="plus" size={17} />
            Create your first item
          </Link>
        </Button>
      )}
    </Card>
  );
}
