import type { ApiFailure, ApiSuccess, FieldError } from "../types/api";

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status = 0,
    public readonly fields: FieldError[] = [],
  ) {
    super(message);
    this.name = "ApiError";
  }
}

const baseURL = (import.meta.env.VITE_API_BASE_URL || "/api/v1").replace(
  /\/$/,
  "",
);
const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null;
function isFailure(value: unknown): value is ApiFailure {
  return (
    isObject(value) &&
    value.success === false &&
    typeof value.message === "string" &&
    Array.isArray(value.errors) &&
    value.errors.every(
      (field) =>
        isObject(field) &&
        typeof field.field === "string" &&
        typeof field.reason === "string",
    )
  );
}

export async function apiRequest<T, R extends ApiSuccess<T> = ApiSuccess<T>>(
  path: string,
  options: RequestInit = {},
): Promise<R> {
  const controller = new AbortController();
  const abort = () => controller.abort();
  options.signal?.addEventListener("abort", abort, { once: true });
  if (options.signal?.aborted) controller.abort();
  let timedOut = false;
  const timeout = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, 10_000);
  try {
    const response = await fetch(`${baseURL}${path}`, {
      ...options,
      headers: {
        Accept: "application/json",
        ...(options.body ? { "Content-Type": "application/json" } : {}),
        ...options.headers,
      },
      signal: controller.signal,
    });
    let value: unknown;
    try {
      value = await response.json();
    } catch {
      if (controller.signal.aborted)
        throw new DOMException("Request canceled", "AbortError");
      throw new ApiError(
        "The server returned an unexpected response. Please try again.",
        response.status,
      );
    }
    if (isFailure(value))
      throw new ApiError(value.message, response.status, value.errors);
    if (!response.ok)
      throw new ApiError(
        "Your request could not be completed. Please try again.",
        response.status,
      );
    if (
      !isObject(value) ||
      value.success !== true ||
      typeof value.message !== "string" ||
      !("data" in value)
    ) {
      throw new ApiError(
        "The server returned an unexpected response. Please try again.",
        response.status,
      );
    }
    return value as unknown as R;
  } catch (error) {
    if (error instanceof ApiError) throw error;
    if (options.signal?.aborted)
      throw new DOMException("Request canceled", "AbortError");
    if (timedOut)
      throw new ApiError(
        "The request timed out. Check your connection and try again.",
      );
    throw new ApiError(
      "Unable to reach the server. Check your connection and try again.",
    );
  } finally {
    clearTimeout(timeout);
    options.signal?.removeEventListener("abort", abort);
  }
}
