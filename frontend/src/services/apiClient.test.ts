import { afterEach, expect, it, vi } from "vitest";
import { ApiError, apiRequest } from "./apiClient";
afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});
it("preserves status, message and field errors for duplicate SKU", async () => {
  const payload = {
    success: false,
    message: "SKU already exists",
    errors: [{ field: "sku", reason: "Duplicate entry" }],
  };
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify(payload), { status: 409 }),
      ),
  );
  await expect(
    apiRequest("/items", { method: "POST", body: "{}" }),
  ).rejects.toMatchObject({
    status: 409,
    message: payload.message,
    fields: payload.errors,
  });
});
it("rejects non-JSON and malformed success envelopes safely", async () => {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValueOnce(
        new Response("<html>private trace</html>", { status: 502 }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ success: true, message: "OK" })),
      ),
  );
  for (let i = 0; i < 2; i++)
    await expect(apiRequest("/items")).rejects.toThrow("unexpected response");
});
it("handles network failure without leaking technical details", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockRejectedValue(new Error("private host configuration")),
  );
  await expect(apiRequest("/items")).rejects.toMatchObject({
    status: 0,
    message: "Unable to reach the server. Check your connection and try again.",
  });
});
it("propagates caller cancellation and clears timeout", async () => {
  vi.useFakeTimers();
  vi.stubGlobal(
    "fetch",
    vi.fn(
      (_url, options: RequestInit) =>
        new Promise((_resolve, reject) =>
          options.signal?.addEventListener("abort", () =>
            reject(new DOMException("Aborted", "AbortError")),
          ),
        ),
    ),
  );
  const controller = new AbortController();
  const request = apiRequest("/items", { signal: controller.signal });
  controller.abort();
  await expect(request).rejects.toMatchObject({ name: "AbortError" });
  expect(vi.getTimerCount()).toBe(0);
});
it("bounds requests and never retries a write", async () => {
  vi.useFakeTimers();
  const fetchMock = vi.fn(
    (_url, options: RequestInit) =>
      new Promise((_resolve, reject) =>
        options.signal?.addEventListener("abort", () =>
          reject(new DOMException("Aborted", "AbortError")),
        ),
      ),
  );
  vi.stubGlobal("fetch", fetchMock);
  const request = apiRequest("/items", { method: "POST", body: "{}" });
  const assertion = expect(request).rejects.toBeInstanceOf(ApiError);
  await vi.advanceTimersByTimeAsync(10_000);
  await assertion;
  expect(fetchMock).toHaveBeenCalledTimes(1);
  expect(vi.getTimerCount()).toBe(0);
});
