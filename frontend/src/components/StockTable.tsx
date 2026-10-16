import type { Balance } from "../services/warehouse";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "./ui/table";
export function StockTable({ stocks }: { stocks: Balance[] }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Location</TableHead>
          <TableHead>Zone</TableHead>
          <TableHead>Type</TableHead>
          <TableHead>Quantity</TableHead>
          <TableHead>Last updated</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {stocks.map((s) => (
          <TableRow key={s.id}>
            <TableCell className="font-medium">{s.location.code}</TableCell>
            <TableCell>{s.location.zone}</TableCell>
            <TableCell>{s.location.type}</TableCell>
            <TableCell className="tabular-nums font-semibold">
              {s.qty.toLocaleString()}
            </TableCell>
            <TableCell>{new Date(s.updated_at).toLocaleString()}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
