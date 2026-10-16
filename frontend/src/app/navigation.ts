import {
  LayoutDashboard,
  Package,
  Boxes,
  ArrowDownToLine,
  ArrowRightLeft,
  MapPin,
} from "lucide-react";
export const navigation = [
  {
    to: "/dashboard",
    label: "Dashboard",
    icon: LayoutDashboard,
    group: "Workspace",
    description: "Live warehouse overview",
  },
  {
    to: "/items",
    label: "Items",
    icon: Package,
    group: "Inventory",
    description: "Product directory and item details",
  },
  {
    to: "/inventory",
    label: "Stock inventory",
    icon: Boxes,
    group: "Inventory",
    description: "Quantities across warehouse locations",
  },
  {
    to: "/receiving",
    label: "Receive stock",
    icon: ArrowDownToLine,
    group: "Operations",
    description: "Record an incoming delivery",
  },
  {
    to: "/transfers",
    label: "Stock transfer",
    icon: ArrowRightLeft,
    group: "Operations",
    description: "Move stock between locations",
  },
  {
    to: "/locations",
    label: "Locations",
    icon: MapPin,
    group: "Warehouse",
    description: "Browse storage locations and zones",
  },
];
