import { act, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, useLocation, useNavigate } from "react-router-dom";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { readItemFilters, useItemFilters } from "./useItemFilters";
beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());
function Probe() {
  const filters = useItemFilters();
  const location = useLocation();
  const navigate = useNavigate();
  return (
    <>
      <input
        aria-label="Search"
        value={filters.search}
        onChange={(event) => filters.setSearch(event.target.value)}
      />
      <button onClick={() => filters.setCategory("Tools")}>Category</button>
      <button onClick={() => navigate(-1)}>Back</button>
      <output data-testid="url">{location.search}</output>
    </>
  );
}
function show(entries = ["/items?page=3"]) {
  render(
    <MemoryRouter
      initialEntries={entries}
      future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
    >
      <Probe />
    </MemoryRouter>,
  );
}
it("publishes debounced q and page reset together", () => {
  show();
  fireEvent.change(screen.getByRole("textbox"), { target: { value: "Box" } });
  act(() => vi.advanceTimersByTime(299));
  expect(screen.getByTestId("url")).toHaveTextContent("page=3");
  act(() => vi.advanceTimersByTime(1));
  expect(screen.getByTestId("url")).toHaveTextContent("?q=Box");
  expect(screen.getByTestId("url")).not.toHaveTextContent("page=");
});
it("history navigation cancels pending search even if q is unchanged", () => {
  show(["/items?page=1", "/items?page=3"]);
  fireEvent.change(screen.getByRole("textbox"), {
    target: { value: "stale input" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Back" }));
  act(() => vi.advanceTimersByTime(1000));
  expect(screen.getByRole("textbox")).toHaveValue("");
  expect(screen.getByTestId("url")).toHaveTextContent("?page=1");
});
it("category change preserves pending search and resets page atomically", () => {
  show();
  fireEvent.change(screen.getByRole("textbox"), { target: { value: " box " } });
  fireEvent.click(screen.getByRole("button", { name: "Category" }));
  act(() => vi.advanceTimersByTime(1000));
  expect(screen.getByTestId("url")).toHaveTextContent("?q=box&category=Tools");
  expect(screen.getByRole("textbox")).toHaveValue("box");
});
it("uses safe defaults for invalid browser query values", () => {
  expect(
    readItemFilters(
      new URLSearchParams("page=-1&limit=101&q=%09&category=%E2%80%83"),
    ),
  ).toEqual({ page: 1, limit: 10, q: "", category: "" });
  expect(
    readItemFilters(new URLSearchParams("page=2147483648&limit=1")).page,
  ).toBe(2147483648);
});
