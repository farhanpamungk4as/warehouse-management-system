export interface FieldError {
  field: string;
  reason: string;
}
export interface ApiFailure {
  success: false;
  message: string;
  errors: FieldError[];
}
export interface ApiSuccess<T> {
  success: true;
  message: string;
  data: T;
}
export interface PaginationMeta {
  page: number;
  limit: number;
  total: number;
}
