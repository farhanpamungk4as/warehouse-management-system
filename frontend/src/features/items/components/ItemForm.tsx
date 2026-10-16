import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import { Input } from "../../../components/ui/input";
import {
  Card,
  CardHeader,
  CardContent,
  CardFooter,
  CardTitle,
  CardDescription,
} from "../../../components/ui/card";
import { Button } from "../../../components/ui/button";
import { Icon } from "../../../components/ui/Icon";
import { ApiError } from "../../../services/apiClient";
import type { ItemErrors, ItemField, ItemWrite } from "../types/item";
import { fieldDefinitions, validateItem } from "../types/validation";

const empty: ItemWrite = { sku: "", name: "", category: "", unit: "" };
export function ItemForm({
  initial = empty,
  editing = false,
  pending,
  onSubmit,
}: {
  initial?: ItemWrite;
  editing?: boolean;
  pending: boolean;
  onSubmit: (input: ItemWrite) => Promise<unknown>;
}) {
  const [values, setValues] = useState<ItemWrite>({
    sku: initial.sku,
    name: initial.name,
    category: initial.category,
    unit: initial.unit,
  });
  const [errors, setErrors] = useState<ItemErrors>({});
  const [failure, setFailure] = useState("");
  const locked = useRef(false);
  const focusAfterSave = useRef(false);
  const form = useRef<HTMLFormElement>(null);
  function focusError(fields: ItemErrors) {
    const first = fieldDefinitions.find((field) => fields[field.name]);
    if (first)
      form.current
        ?.querySelector<HTMLInputElement>(`[name="${first.name}"]`)
        ?.focus();
  }
  useEffect(() => {
    // API field errors can arrive before the pending inputs are enabled again.
    if (!pending && focusAfterSave.current) {
      focusAfterSave.current = false;
      focusError(errors);
    }
  }, [errors, pending]);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending || locked.current) return;
    const checked = validateItem(values);
    setErrors(checked.errors);
    setFailure("");
    if (Object.keys(checked.errors).length) {
      focusError(checked.errors);
      return;
    }
    locked.current = true;
    try {
      await onSubmit(checked.values);
    } catch (error) {
      const fields: ItemErrors = {};
      if (error instanceof ApiError) {
        for (const entry of error.fields)
          if (fieldDefinitions.some((field) => field.name === entry.field))
            fields[entry.field as ItemField] = entry.reason;
      }
      focusAfterSave.current = true;
      setErrors(fields);
      setFailure(
        error instanceof ApiError
          ? error.message
          : "The item could not be saved. Please try again.",
      );
    } finally {
      locked.current = false;
    }
  }
  return (
    <Card className="item-form">
      <form
        ref={form}
        noValidate
        onSubmit={(event) => void submit(event)}
        aria-busy={pending}
      >
        <CardHeader className="form-card-heading">
          <div>
            <CardTitle>
              <h2>Item details</h2>
            </CardTitle>
            <CardDescription>
              <p>The essentials your team needs to identify this item.</p>
            </CardDescription>
          </div>
          <span className="required-note">All fields required</span>
        </CardHeader>
        <CardContent className="form-fields">
          {failure && (
            <div className="form-alert" role="alert">
              <Icon name="info" size={19} />
              <span>{failure}</span>
            </div>
          )}
          {fieldDefinitions.map((field) => (
            <div className={`form-field field-${field.name}`} key={field.name}>
              <div className="field-label">
                <label htmlFor={field.name}>
                  {field.label}
                  <span aria-hidden="true"> *</span>
                </label>
                <span className="character-count">
                  {[...values[field.name]].length}/{field.max}
                </span>
              </div>
              <Input
                id={field.name}
                name={field.name}
                value={values[field.name]}
                disabled={pending}
                required
                placeholder={field.placeholder}
                autoComplete="off"
                aria-invalid={Boolean(errors[field.name])}
                aria-describedby={`${field.name}-hint${errors[field.name] ? ` ${field.name}-error` : ""}`}
                onChange={(event) => {
                  setValues((current) => ({
                    ...current,
                    [field.name]: event.target.value,
                  }));
                  setErrors((current) => ({
                    ...current,
                    [field.name]: undefined,
                  }));
                  setFailure("");
                }}
              />
              <p id={`${field.name}-hint`} className="field-hint">
                {field.hint}
              </p>
              {errors[field.name] && (
                <p className="field-error" id={`${field.name}-error`}>
                  {errors[field.name]}
                </p>
              )}
            </div>
          ))}
        </CardContent>
        <CardFooter className="form-footer">
          <span className="save-note">
            <Icon name="info" size={16} />
            You can update these details later.
          </span>
          <div>
            <Button variant="outline" asChild>
              <Link
                className={
                  pending ? "pointer-events-none opacity-50" : undefined
                }
                aria-disabled={pending}
                tabIndex={pending ? -1 : 0}
                onClick={(event) => {
                  if (pending) event.preventDefault();
                }}
                to="/items"
              >
                Cancel
              </Link>
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? (
                <>
                  <span className="spinner" />
                  Saving…
                </>
              ) : (
                <>
                  <Icon name={editing ? "check" : "plus"} size={17} />
                  {editing ? "Save changes" : "Create item"}
                </>
              )}
            </Button>
          </div>
        </CardFooter>
      </form>
    </Card>
  );
}
