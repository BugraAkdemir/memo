// The installed video workspace has no @types/react-dom package.
// Keep the small createRoot boundary typed without changing that shared workspace.
declare module "react-dom/client" {
  import type { ReactNode } from "react";
  export function createRoot(container: Element | DocumentFragment): {
    render(children: ReactNode): void;
    unmount(): void;
  };
}
