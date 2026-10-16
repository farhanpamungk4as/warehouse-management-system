import { render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { ErrorBoundary } from "./ErrorBoundary";
afterEach(() => vi.restoreAllMocks());
it("catches render failures and offers safe recovery navigation", () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  function Broken(): never {
    throw new Error("private implementation detail");
  }
  render(
    <ErrorBoundary>
      <Broken />
    </ErrorBoundary>,
  );
  expect(
    screen.getByRole("heading", { name: "Something went wrong" }),
  ).toBeVisible();
  expect(screen.getByRole("button", { name: "Reload page" })).toBeVisible();
  expect(
    screen.getByRole("link", { name: "Return to dashboard" }),
  ).toHaveAttribute("href", "/dashboard");
  expect(
    screen.queryByText("private implementation detail"),
  ).not.toBeInTheDocument();
});
