import { Link } from "react-router-dom";
import {
  Package,
  Pencil,
  Eye,
  MoreHorizontal,
  ArrowDownToLine,
  ArrowRightLeft,
} from "lucide-react";
import type { Item } from "../types/item";
import { Button } from "../../../components/ui/button";
import { Badge } from "../../../components/ui/badge";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "../../../components/ui/table";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../../../components/ui/dropdown-menu";
import { DeleteItem } from "./DeleteItem";
export function ItemTable({ items }: { items: Item[] }) {
  return (
    <div
      className="table-scroll"
      tabIndex={0}
      role="region"
      aria-label="Item directory table"
    >
      <Table className="item-directory-table">
        <TableHeader>
          <TableRow>
            <TableHead>SKU</TableHead>
            <TableHead>Item name</TableHead>
            <TableHead>Category</TableHead>
            <TableHead>Unit</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((item) => (
            <TableRow key={item.id}>
              <TableCell>
                <Link className="sku" to={`/items/${item.id}`}>
                  {item.sku}
                </Link>
              </TableCell>
              <TableCell>
                <Link className="item-name" to={`/items/${item.id}`}>
                  <span className="item-glyph">
                    <Package size={18} />
                  </span>
                  {item.name}
                </Link>
              </TableCell>
              <TableCell>
                <Badge variant="secondary">{item.category}</Badge>
              </TableCell>
              <TableCell>{item.unit}</TableCell>
              <TableCell>
                <div className="flex items-center justify-end gap-1">
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label={`Actions for ${item.name}`}
                      >
                        <MoreHorizontal size={18} />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-52">
                      <DropdownMenuLabel>Item actions</DropdownMenuLabel>
                      <DropdownMenuItem asChild>
                        <Link to={`/items/${item.id}`}>
                          <Eye />
                          View details
                        </Link>
                      </DropdownMenuItem>
                      <DropdownMenuItem asChild>
                        <Link to={`/items/${item.id}/edit`}>
                          <Pencil />
                          Edit item
                        </Link>
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem asChild>
                        <Link to={`/receiving?item_id=${item.id}`}>
                          <ArrowDownToLine />
                          Receive stock
                        </Link>
                      </DropdownMenuItem>
                      <DropdownMenuItem asChild>
                        <Link to={`/transfers?item_id=${item.id}`}>
                          <ArrowRightLeft />
                          Transfer stock
                        </Link>
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                  <DeleteItem item={item} />
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
