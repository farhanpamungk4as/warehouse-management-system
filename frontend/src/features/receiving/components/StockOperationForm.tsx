import { useStockOperation } from "../hooks/useStockOperation";
import { Link } from "react-router-dom";
import {
  ArrowDownToLine,
  ArrowRightLeft,
  ShieldCheck,
  CheckCircle2,
} from "lucide-react";
import { ItemCombobox } from "../../items/components/ItemCombobox";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "../../../components/ui/card";
import { Input } from "../../../components/ui/input";
import { Button } from "../../../components/ui/button";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
} from "../../../components/ui/alert-dialog";
import { SelectField } from "../../../components/ui/SelectField";
import { MapPin } from "lucide-react";
import { PageHeading } from "../../../components/PageHeading";
export function StockOperationForm({
  transfer = false,
}: {
  transfer?: boolean;
}) {
  const {
    item,
    setItem,
    from,
    setFrom,
    to,
    setTo,
    qty,
    setQty,
    fields,
    setFields,
    failure,
    setFailure,
    uncertain,
    setUncertain,
    confirm,
    setConfirm,
    result,
    setResult,
    form,
    locations,
    stock,
    balance,
    pending,
    execute,
    submit,
  } = useStockOperation(transfer);
  const label = (id: string) =>
    locations.data?.data.find((l) => l.id === Number(id))?.code ?? "—";
  const locationField = (destination: boolean) => {
    const key = destination
      ? "to_location_id"
      : transfer
        ? "from_location_id"
        : "location_id";
    return (
      <div className="operation-field">
        <label htmlFor={key}>
          {destination
            ? "Destination location"
            : transfer
              ? "Source location"
              : "Location"}
        </label>
        <SelectField
          id={key}
          label={
            destination
              ? "Destination location"
              : transfer
                ? "Source location"
                : "Location"
          }
          value={destination ? to : from}
          onChange={(value) => {
            destination ? setTo(value) : setFrom(value);
            setResult(null);
          }}
          disabled={pending || uncertain || locations.isPending}
          invalid={!!fields[key]}
          describedBy={fields[key] ? `${key}-error` : undefined}
          placeholder="Choose a location"
          groupLabel={
            destination ? "Transfer destination" : "Warehouse locations"
          }
          icon={<MapPin size={16} />}
          options={(locations.data?.data ?? []).map((location) => ({
            value: String(location.id),
            label: `${location.code} · Zone ${location.zone}`,
            disabled:
              transfer && String(location.id) === (destination ? from : to),
          }))}
        />

        {fields[key] && (
          <p className="field-error" id={`${key}-error`}>
            {fields[key]}
          </p>
        )}
      </div>
    );
  };
  return (
    <>
      <PageHeading
        title={transfer ? "Stock transfer" : "Receive stock"}
        description={
          transfer
            ? "Move stock between locations with balances kept in sync."
            : "Record incoming stock against an item and storage location."
        }
      />
      <div className="operation-layout">
        <Card>
          <CardHeader>
            <CardTitle>
              {transfer ? "Transfer details" : "New receipt"}
            </CardTitle>
            <CardDescription>
              All fields are required. Balances are checked again when you
              submit.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form
              ref={form}
              onSubmit={submit}
              noValidate
              aria-busy={pending}
              className="operation-form"
            >
              <div className="operation-field">
                <label htmlFor="item_id">Item</label>
                <ItemCombobox
                  value={item}
                  disabled={pending || uncertain}
                  onChange={(id) => {
                    setItem(id);
                    setResult(null);
                    setFields({});
                  }}
                />
                {fields.item_id && (
                  <p className="field-error">{fields.item_id}</p>
                )}
              </div>
              {locationField(false)}
              {transfer && locationField(true)}
              <div className="operation-field">
                <label htmlFor="qty">Quantity</label>
                <Input
                  id="qty"
                  inputMode="numeric"
                  value={qty}
                  onChange={(e) => {
                    setQty(e.target.value);
                    setResult(null);
                  }}
                  placeholder="Enter quantity"
                  disabled={pending || uncertain}
                  aria-invalid={!!fields.qty}
                  aria-describedby={fields.qty ? "qty-error" : "quantity-hint"}
                />
                <p id="quantity-hint" className="field-hint">
                  Whole units only. Maximum 2,147,483,647 per location.
                </p>
                {fields.qty && (
                  <p id="qty-error" className="field-error">
                    {fields.qty}
                  </p>
                )}
              </div>
              {failure && (
                <div className="form-alert" role="alert">
                  <div>
                    {failure}
                    {Object.entries(fields)
                      .filter(
                        ([k]) =>
                          ![
                            "item_id",
                            "qty",
                            "location_id",
                            "from_location_id",
                            "to_location_id",
                          ].includes(k),
                      )
                      .map(([k, v]) => (
                        <p key={k}>{v}</p>
                      ))}
                  </div>
                </div>
              )}
              {uncertain && (
                <div className="uncertain-warning" role="alert">
                  <strong>The result may already be committed.</strong>
                  <p>
                    Verify the actual balances before manually trying again.
                    This operation is never retried automatically.
                  </p>
                  <Button variant="link" className="h-auto p-0" asChild>
                    <Link to={`/items/${item}`}>View item balances</Link>
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    className="mt-3"
                    onClick={async () => {
                      const fresh = await stock.refetch();
                      if (fresh.isSuccess) {
                        setUncertain(false);
                        setFailure("");
                        setQty("");
                      }
                    }}
                  >
                    Refresh balances before retry
                  </Button>
                </div>
              )}
              {locations.isError && (
                <div role="alert" className="field-error">
                  Locations could not be loaded.{" "}
                  <Button
                    variant="link"
                    size="sm"
                    className="h-auto p-0"
                    type="button"
                    onClick={() => void locations.refetch()}
                  >
                    Retry locations
                  </Button>
                </div>
              )}
              <Button
                type="submit"
                disabled={
                  pending ||
                  uncertain ||
                  locations.isPending ||
                  locations.isError ||
                  !stock.isSuccess
                }
                className="w-full"
              >
                {pending ? (
                  <>
                    <span className="spinner" />
                    Saving…
                  </>
                ) : transfer ? (
                  <>
                    <ArrowRightLeft />
                    Review transfer
                  </>
                ) : (
                  <>
                    <ArrowDownToLine />
                    Receive stock
                  </>
                )}
              </Button>
            </form>
          </CardContent>
        </Card>
        <div className="operation-aside">
          <Card>
            <CardHeader>
              <CardTitle>Balance preview</CardTitle>
              <CardDescription>
                Current stored balances for the selected item.
              </CardDescription>
            </CardHeader>
            <CardContent>
              {!item ? (
                <p className="muted">Choose an item to see its stock.</p>
              ) : stock.isPending ? (
                <p className="muted">Loading balances…</p>
              ) : stock.isError ? (
                <div role="alert">
                  <p className="field-error">{stock.error.message}</p>
                  <Button
                    variant="outline"
                    onClick={() => void stock.refetch()}
                  >
                    Retry balances
                  </Button>
                </div>
              ) : (
                <>
                  <dl className="details">
                    <div>
                      <dt>
                        {transfer ? "Source" : "Location"} · {label(from)}
                      </dt>
                      <dd data-testid="source-balance">
                        {from ? balance(from).toLocaleString() : "—"}
                      </dd>
                    </div>
                    {transfer && (
                      <div>
                        <dt>Destination · {label(to)}</dt>
                        <dd data-testid="destination-balance">
                          {to ? balance(to).toLocaleString() : "—"}
                        </dd>
                      </div>
                    )}
                  </dl>
                  <Button asChild variant="link" className="px-0 mt-4">
                    <Link to={`/items/${item}`}>
                      View item and all locations
                    </Link>
                  </Button>
                </>
              )}
            </CardContent>
          </Card>
          {result && (
            <Card className="result-card" role="status">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <CheckCircle2 size={19} />
                  Stock {transfer ? "transferred" : "received"}
                </CardTitle>
              </CardHeader>
              <CardContent>
                {"source" in result ? (
                  <p>
                    Committed balances: source{" "}
                    <strong>{result.source.qty.toLocaleString()}</strong>,
                    destination{" "}
                    <strong>{result.destination.qty.toLocaleString()}</strong>.
                  </p>
                ) : (
                  <p>
                    Committed balance:{" "}
                    <strong>{result.qty.toLocaleString()}</strong> at{" "}
                    {label(String(result.location_id))}.
                  </p>
                )}
                <Button asChild variant="outline" className="mt-4">
                  <Link to="/inventory">View updated inventory</Link>
                </Button>
              </CardContent>
            </Card>
          )}
          <div className="operation-note">
            <ShieldCheck size={19} />
            <p>
              {transfer
                ? "Both balances are updated in one transaction. Total item quantity stays unchanged."
                : "Stock is added to the existing balance. Each successful submission adds the entered quantity."}
            </p>
          </div>
        </div>
      </div>
      <AlertDialog open={confirm} onOpenChange={setConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Confirm stock transfer</AlertDialogTitle>
            <AlertDialogDescription>
              Move {qty} units from {label(from)} to {label(to)}? The server
              will verify available stock before committing both balances.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <Button
              onClick={() => void execute()}
              disabled={pending || uncertain}
            >
              Confirm transfer
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
