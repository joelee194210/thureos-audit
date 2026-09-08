"use client";

import { Component, type ReactNode } from "react";

interface TabErrorBoundaryProps {
  tabLabel: string;
  children: ReactNode;
}

interface TabErrorBoundaryState {
  error: Error | null;
}

/** Contiene el throw de render de UN tab-content a su propio panel.
 * TabContentHost mantiene montadas todas las tabs abiertas simultáneamente
 * (para no perder su estado) — sin este boundary, un error de render en
 * cualquiera de ellas tumba el árbol completo, incluida la tab visible. */
export class TabErrorBoundary extends Component<
  TabErrorBoundaryProps,
  TabErrorBoundaryState
> {
  state: TabErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): TabErrorBoundaryState {
    return { error };
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex flex-col items-center justify-center gap-2 p-8 text-center">
          <p className="text-sm font-medium text-destructive">
            No se pudo cargar &quot;{this.props.tabLabel}&quot;
          </p>
          <p className="text-xs text-muted-foreground">
            Cerrá esta pestaña y volvé a abrirla para reintentar.
          </p>
        </div>
      );
    }
    return this.props.children;
  }
}
