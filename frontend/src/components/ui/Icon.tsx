import type { SVGProps } from "react";
import {
  Package,
  LayoutGrid,
  Search,
  Plus,
  ArrowRight,
  ArrowLeft,
  ChevronRight,
  Pencil,
  Check,
  X,
  Info,
  RefreshCw,
  Warehouse,
  ListFilter,
} from "lucide-react";
const icons = {
  box: Package,
  grid: LayoutGrid,
  search: Search,
  plus: Plus,
  arrow: ArrowRight,
  back: ArrowLeft,
  chevron: ChevronRight,
  edit: Pencil,
  check: Check,
  close: X,
  info: Info,
  refresh: RefreshCw,
  warehouse: Warehouse,
  filter: ListFilter,
};
export function Icon({
  name,
  size = 20,
  ...props
}: SVGProps<SVGSVGElement> & { name: keyof typeof icons; size?: number }) {
  const Component = icons[name];
  return <Component size={size} aria-hidden="true" {...props} />;
}
