import { Skeleton } from "./skeleton";
export function LoadingState({ form = false }: { form?: boolean }) {
  return (
    <div
      role="status"
      aria-label={form ? "Loading item" : "Loading items"}
      className="loading-state"
    >
      <span className="sr-only">
        {form ? "Loading item…" : "Loading items…"}
      </span>
      {Array.from({ length: form ? 4 : 6 }, (_, index) => (
        <Skeleton
          key={index}
          className={`skeleton ${form ? "skeleton-field" : "skeleton-row"}`}
        />
      ))}
    </div>
  );
}
