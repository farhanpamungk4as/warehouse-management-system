import { useEffect, useState } from "react";
import { useLocation, useSearchParams } from "react-router-dom";
import { useDebouncedValue } from "../../../hooks/useDebouncedValue";
import type { ItemListParams } from "../types/item";
import { isUnicodeWhitespaceOnly } from "../types/validation";

const trim = (value: string) => value.replace(/^ +| +$/g, "");
export function readItemFilters(query: URLSearchParams): ItemListParams {
  const integer = (name: string, fallback: number, max: number) => {
    const raw = query.get(name) ?? "";
    const value = Number(raw);
    return /^\d+$/.test(raw) &&
      Number.isSafeInteger(value) &&
      value >= 1 &&
      value <= max
      ? value
      : fallback;
  };
  const limit = integer("limit", 10, 100);
  const text = (key: string, max: number) => {
    const value = trim(query.get(key) ?? "");
    return [...value].length <= max &&
      !/[\u0000-\u001f\u007f]/.test(value) &&
      (!value || !isUnicodeWhitespaceOnly(value))
      ? value
      : "";
  };
  return {
    page: integer("page", 1, Math.floor(2147483647 / limit) + 1),
    limit,
    q: text("q", 200),
    category: text("category", 64),
  };
}
export function useItemFilters(preserveKeys: string[] = []) {
  const [searchParams, setSearchParams] = useSearchParams();
  const location = useLocation();
  const params = readItemFilters(searchParams);
  const [draft, setDraft] = useState({
    value: params.q,
    key: location.key,
    dirty: false,
  });
  const debounced = useDebouncedValue(draft, 300);
  const search = draft.key === location.key ? draft.value : params.q;
  function publish(next: ItemListParams) {
    const query = new URLSearchParams();
    preserveKeys.forEach((key) => {
      const value = searchParams.get(key);
      if (value) query.set(key, value);
    });
    if (next.q) query.set("q", next.q);
    if (next.category) query.set("category", next.category);
    if (next.page !== 1) query.set("page", String(next.page));
    if (next.limit !== 10) query.set("limit", String(next.limit));
    setSearchParams(query);
  }
  useEffect(() => {
    // History/navigation invalidates a pending draft by key, even if q is equal.
    if (debounced !== draft || !debounced.dirty || draft.key !== location.key)
      return;
    const q = trim(debounced.value);
    setDraft({ value: q, key: location.key, dirty: false });
    if (q !== params.q) publish({ ...params, q, page: 1 });
  }, [
    debounced,
    draft,
    location.key,
    params.q,
    params.category,
    params.page,
    params.limit,
  ]);
  const change = (patch: Partial<ItemListParams>) => {
    const q = trim(search);
    setDraft({ value: q, key: location.key, dirty: false });
    publish({ ...params, q, ...patch });
  };
  return {
    params,
    search,
    setSearch: (value: string) =>
      setDraft({ value, key: location.key, dirty: true }),
    setCategory: (category: string) => change({ category, page: 1 }),
    setLimit: (limit: number) => change({ limit, page: 1 }),
    setPage: (page: number) => change({ page }),
    clear: () => {
      setDraft({ value: "", key: location.key, dirty: false });
      publish({ ...params, page: 1, q: "", category: "" });
    },
  };
}
