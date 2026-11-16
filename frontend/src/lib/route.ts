export type Route = "overview" | "workflows" | "runners";
export const pages: { route: Route; path: string; label: string }[] = [
  { route: "overview", path: "/", label: "Overview" },
  { route: "workflows", path: "/workflows", label: "Workflows" },
  { route: "runners", path: "/runners", label: "Runners" },
];
export function routeFor(pathname: string): Route {
  const path = pathname.replace(/\/+$/, "");
  return pages.find((page) => page.path === path && path !== "")?.route ?? "overview";
}
// The dev-only ?mock preview flag survives page changes.
export function withMock(href: string, mock: boolean): string {
  if (!mock) return href;
  const url = new URL(href, "http://local");
  url.searchParams.set("mock", "");
  return url.pathname + url.search;
}
// Follow an in-app link without a page load, leaving modified and new-tab clicks alone.
export function followLink(event: MouseEvent, go: (href: string) => void) {
  const anchor = event.currentTarget as HTMLAnchorElement;
  if (
    event.defaultPrevented ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey ||
    (anchor.target && anchor.target !== "_self")
  )
    return;
  event.preventDefault();
  go(anchor.getAttribute("href") ?? "/");
}
