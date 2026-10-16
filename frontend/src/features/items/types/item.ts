import type { ApiSuccess, PaginationMeta } from "../../../types/api";

export interface ItemWrite {
  sku: string;
  name: string;
  category: string;
  unit: string;
}
export interface Item extends ItemWrite {
  id: number;
  created_at: string;
}
export interface ItemListParams {
  page: number;
  limit: number;
  category: string;
  q: string;
}
export interface ItemListMeta extends PaginationMeta {
  categories: string[];
}
export interface ItemListResponse extends ApiSuccess<Item[]> {
  meta: ItemListMeta;
}
export type ItemField = keyof ItemWrite;
export type ItemErrors = Partial<Record<ItemField, string>>;
