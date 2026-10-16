import { Link } from "react-router-dom";
import { Card } from "./card";
import { Button } from "./button";
import { Icon } from "./Icon";

export function ErrorState({
  title = "We couldn’t load your items",
  message,
  retry,
}: {
  title?: string;
  message: string;
  retry?: () => void;
}) {
  return (
    <Card className="state-panel" role="alert">
      <div className="state-icon">
        <Icon name="info" size={25} />
      </div>
      <h2>{title}</h2>
      <p>{message}</p>
      {retry ? (
        <Button variant="outline" onClick={retry}>
          <Icon name="refresh" size={17} />
          Try again
        </Button>
      ) : (
        <Button variant="outline" asChild>
          <Link to="/items">
            <Icon name="back" size={17} />
            Back to items
          </Link>
        </Button>
      )}
    </Card>
  );
}
