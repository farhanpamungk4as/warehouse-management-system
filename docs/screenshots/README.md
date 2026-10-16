# Actual browser captures

Captured by the 28-test Chrome acceptance run against the disposable Docker project, using real PostgreSQL fixture data. These are intentional review artifacts; traces and temporary test files remain ignored. Counts reflect the capture moment, before the final test soft-deleted its stock fixture.

| Viewport | Dashboard | Inventory | Transfer confirmation |
|---|---|---|---|
| 1440 × 960 | [Overview](dashboard-1440.png) | [Inventory](inventory-1440.png) | [Confirmation](transfer-confirm-1440.png) |
| 1280 × 960 | [Overview](dashboard-1280.png) | [Inventory](inventory-1280.png) | [Confirmation](transfer-confirm-1280.png) |
| 768 × 960 | [Overview](dashboard-768.png) | [Inventory](inventory-768.png) | [Confirmation](transfer-confirm-768.png) |
| 390 × 960 | [Overview](dashboard-390.png) | [Inventory](inventory-390.png) | [Confirmation](transfer-confirm-390.png) |

The actual browser checks assert no horizontal document overflow, tablet/mobile Sheet navigation, desktop collapse, table-contained scrolling, keyboard combobox focus/Escape, visible input focus and dialogs within the viewport. Inventory intentionally scrolls horizontally on narrow screens to retain readable columns.

## Revised admin layout and dropdowns

The supplied visual reference guided the lighter, denser UI. These images use actual PostgreSQL browser-test fixtures; long identifiers/categories intentionally exercise truncation and table scrolling. Open-menu images capture the viewport so fixed portal menus remain visible.

| Viewport | Item list | Open shadcn category menu |
|---|---|---|
| 1440 | [Items](items-1440.png) | [Menu](items-dropdown-1440.png) |
| 1280 | [Items](items-1280.png) | [Menu](items-dropdown-1280.png) |
| 768 | [Items](items-768.png) | [Menu](items-dropdown-768.png) |
| 390 | [Items](items-390.png) | [Menu](items-dropdown-390.png) |

For a clearer visual review with ordinary product names: [desktop](admin-review-desktop.png), [open dropdown](admin-review-dropdown.png), [mobile](admin-review-mobile.png). These three captures use 10 sample products created through the actual API in the disposable UI database after the regression suite. They do not represent retained developer inventory. The long-identifier captures above remain the responsive stress checks.

## Location CRUD and seeded dashboard

[Locations desktop](locations-1440.png) · [Locations mobile](locations-390.png) · [Create location desktop](location-create-1440.png) · [Create location mobile](location-create-390.png). Location captures use real browser fixtures. [Seeded overview desktop](demo-dashboard-1440.png) and [mobile](demo-dashboard-390.png) use the ten demo items and 900 units created by scripts/seed_demo.py in the isolated database.
