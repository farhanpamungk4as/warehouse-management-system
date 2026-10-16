import type { ReactNode } from "react";
import { toast } from "sonner";
import { Toaster } from "./sonner";
export function ToastProvider({ children }: { children: ReactNode }) {
  return (
    <>
      {children}
      <Toaster
        theme="light"
        richColors
        closeButton
        position="bottom-right"
        duration={6000}
      />
    </>
  );
}
export function useToast() {
  return (message: string, tone: "success" | "error") => {
    toast[tone](message);
  };
}
