"use client";

import { createContext, useContext, useState, useCallback, type ReactNode } from "react";

interface Toast {
  id: number;
  message: string;
  type: "error" | "success";
}

interface ToastContextType {
  toastError: (message: string) => void;
  toastSuccess: (message: string) => void;
}

const ToastContext = createContext<ToastContextType>({
  toastError: () => {},
  toastSuccess: () => {},
});

export function useToast() {
  return useContext(ToastContext);
}

let nextId = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const addToast = useCallback((message: string, type: "error" | "success") => {
    const id = ++nextId;
    setToasts((prev) => [...prev, { id, message, type }]);
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 4000);
  }, []);

  const toastError = useCallback((message: string) => addToast(message, "error"), [addToast]);
  const toastSuccess = useCallback((message: string) => addToast(message, "success"), [addToast]);

  return (
    <ToastContext.Provider value={{ toastError, toastSuccess }}>
      {children}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
        {toasts.map((toast) => (
          <div
            key={toast.id}
            className={`animate-in slide-in-from-right rounded-md px-4 py-3 text-sm font-medium shadow-lg ${
              toast.type === "error"
                ? "bg-danger-bg text-danger-fg border border-danger-border"
                : "bg-success-bg text-success-fg border border-success-border"
            }`}
          >
            {toast.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
