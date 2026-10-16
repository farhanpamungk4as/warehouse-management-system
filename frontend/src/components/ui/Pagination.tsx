import { SelectField } from "./SelectField";
import { Button } from "./button";
import { Icon } from "./Icon";

export function Pagination({
  page,
  limit,
  total,
  disabled,
  onPage,
  onLimit,
}: {
  page: number;
  limit: number;
  total: number;
  disabled?: boolean;
  onPage: (page: number) => void;
  onLimit: (limit: number) => void;
}) {
  const pages = Math.max(1, Math.ceil(total / limit));
  const start = total === 0 ? 0 : (page - 1) * limit + 1;
  return (
    <div className="pagination">
      <p className="results-range">
        Showing{" "}
        <strong>
          {start}–{Math.min(page * limit, total)}
        </strong>{" "}
        of <strong>{total.toLocaleString()}</strong> items
      </p>
      <div className="pagination-controls">
        <label className="page-size">
          Rows per page
          <SelectField
            label="Rows per page"
            value={String(limit)}
            onChange={(value) => onLimit(Number(value))}
            className="page-size-select"
            options={[...new Set([10, 25, 50, 100, limit])]
              .sort((a, b) => a - b)
              .map((value) => ({ value: String(value), label: String(value) }))}
          />
        </label>
        <nav aria-label="Pagination" className="page-buttons">
          <Button
            variant="outline"
            size="icon-sm"
            className="icon-button"
            aria-label="Previous page"
            disabled={disabled || page <= 1}
            onClick={() => onPage(page - 1)}
          >
            <Icon name="chevron" className="rotate-180" size={17} />
          </Button>
          <span aria-live="polite">
            Page <strong>{page}</strong> of {pages}
          </span>
          <Button
            variant="outline"
            size="icon-sm"
            className="icon-button"
            aria-label="Next page"
            disabled={disabled || page >= pages}
            onClick={() => onPage(page + 1)}
          >
            <Icon name="chevron" size={17} />
          </Button>
        </nav>
      </div>
    </div>
  );
}
