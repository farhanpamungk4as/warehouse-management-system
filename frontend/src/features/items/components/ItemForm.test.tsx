import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { expect, it, vi } from "vitest";
import { ItemForm } from "./ItemForm";
import { ApiError } from "../../../services/apiClient";
import { validateItem } from "../types/validation";
import type { ItemWrite } from "../types/item";
const input: ItemWrite = {
  sku: " abc ",
  name: " Box ",
  category: " Tools ",
  unit: " pcs ",
};
function show(props: Parameters<typeof ItemForm>[0]) {
  return render(
    <MemoryRouter
      future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
    >
      <ItemForm {...props} />
    </MemoryRouter>,
  );
}
it("rejects required fields before mutation and focuses first error", () => {
  const submit = vi.fn();
  show({ pending: false, onSubmit: submit });
  fireEvent.click(screen.getByRole("button", { name: "Create item" }));
  expect(submit).not.toHaveBeenCalled();
  expect(screen.getByLabelText(/SKU/)).toHaveFocus();
  expect(screen.getByText("SKU is required.")).toBeVisible();
  expect(screen.getByText("Item name is required.")).toBeVisible();
  const name = screen.getByLabelText(/Item name/);
  name.focus();
  fireEvent.change(name, { target: { value: "Box" } });
  expect(name).toHaveFocus();
});
it("trims spaces, prevents concurrent submit and preserves duplicate SKU fields", async () => {
  let reject!: (error: unknown) => void;
  const submit = vi.fn(
    () =>
      new Promise((_resolve, fail) => {
        reject = fail;
      }),
  );
  show({ initial: input, pending: false, onSubmit: submit });
  const button = screen.getByRole("button", { name: "Create item" });
  fireEvent.click(button);
  fireEvent.click(button);
  expect(submit).toHaveBeenCalledTimes(1);
  expect(submit).toHaveBeenCalledWith({
    sku: "abc",
    name: "Box",
    category: "Tools",
    unit: "pcs",
  });
  await act(async () =>
    reject(
      new ApiError("SKU already exists", 409, [
        { field: "sku", reason: "Duplicate entry" },
      ]),
    ),
  );
  await waitFor(() =>
    expect(screen.getByText("Duplicate entry")).toBeVisible(),
  );
  expect(screen.getByRole("alert")).toHaveTextContent("SKU already exists");
  expect(screen.getByLabelText(/SKU/)).toHaveValue(" abc ");
});
it("disables fields and submit while pending", () => {
  show({ initial: input, pending: true, onSubmit: vi.fn() });
  expect(screen.getByRole("button", { name: "Saving…" })).toBeDisabled();
  expect(screen.getByLabelText(/Item name/)).toBeDisabled();
});
it("counts code points and matches blank/control/length rules", () => {
  const valid = {
    sku: "界".repeat(64),
    name: "😀".repeat(200),
    category: "界".repeat(64),
    unit: "😀".repeat(32),
  };
  expect(validateItem(valid).errors).toEqual({});
  for (const sku of [
    "\u2003\u00a0",
    "\u0085",
    "bad\tSKU",
    "bad\u007f",
    "界".repeat(65),
    "\ud800",
  ])
    expect(validateItem({ ...valid, sku }).errors.sku).toBeTruthy();
});
