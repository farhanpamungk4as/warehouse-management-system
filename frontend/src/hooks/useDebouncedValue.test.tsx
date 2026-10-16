import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useDebouncedValue } from "./useDebouncedValue";
beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());
it("waits 300ms, publishes only the latest value and cleans up", () => {
  const { result, rerender, unmount } = renderHook(
    ({ value }) => useDebouncedValue(value, 300),
    { initialProps: { value: "" } },
  );
  rerender({ value: "bo" });
  act(() => vi.advanceTimersByTime(200));
  rerender({ value: "box" });
  act(() => vi.advanceTimersByTime(299));
  expect(result.current).toBe("");
  act(() => vi.advanceTimersByTime(1));
  expect(result.current).toBe("box");
  rerender({ value: "boxes" });
  unmount();
  expect(vi.getTimerCount()).toBe(0);
});
