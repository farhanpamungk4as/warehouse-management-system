import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Search, ArrowUpRight } from "lucide-react";
import {
  CommandDialog,
  CommandInput,
  CommandList,
  CommandGroup,
  CommandItem,
  CommandEmpty,
} from "./ui/command";
import { Button } from "./ui/button";
import { navigation } from "../app/navigation";
export function NavigationSearch() {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();
  useEffect(() => {
    const key = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setOpen((value) => !value);
      }
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  }, []);
  return (
    <>
      <Button
        variant="outline"
        type="button"
        className="module-search"
        aria-label="Search menus or modules"
        onClick={() => setOpen(true)}
      >
        <Search size={17} />
        <span>Search menus or modules…</span>
        <kbd>⌘ K</kbd>
      </Button>
      <CommandDialog
        open={open}
        onOpenChange={setOpen}
        title="Find a page"
        description="Search warehouse menus and navigate to a page."
      >
        <CommandInput placeholder="Search menus or modules…" />
        <CommandList>
          <CommandEmpty>No matching pages.</CommandEmpty>
          <CommandGroup heading="Warehouse menus">
            {navigation.map((item) => (
              <CommandItem
                key={item.to}
                value={`${item.label} ${item.description}`}
                onSelect={() => {
                  setOpen(false);
                  navigate(item.to);
                }}
              >
                <item.icon />
                <span className="flex-1">
                  <strong className="block font-medium">{item.label}</strong>
                  <span className="text-xs text-muted-foreground">
                    {item.description}
                  </span>
                </span>
                <ArrowUpRight className="opacity-40" />
              </CommandItem>
            ))}
          </CommandGroup>
        </CommandList>
      </CommandDialog>
    </>
  );
}
