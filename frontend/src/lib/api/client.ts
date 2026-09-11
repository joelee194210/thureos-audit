const API_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

/**
 * Un error del backend que conserva el código de estado. Antes se lanzaba un
 * Error pelado con el mensaje, así que quien lo atrapaba no podía distinguir
 * "esto no existe" de "el servidor falló", y solo sabía hacer una cosa:
 * mostrar un toast. Una pestaña guardada apuntando a una entidad borrada
 * repetía ese toast en cada carga, para siempre.
 *
 * Extiende Error a propósito: los `err instanceof Error ? err.message` que ya
 * existen en el proyecto siguen funcionando sin cambios.
 */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/** Si el error viene del backend y es un 404. */
export function esNoEncontrado(err: unknown): boolean {
  return err instanceof ApiError && err.status === 404;
}

class ApiClient {
  private token: string | null = null;

  setToken(token: string) {
    this.token = token;
    if (typeof window !== "undefined") {
      localStorage.setItem("token", token);
    }
  }

  getToken(): string | null {
    if (this.token) return this.token;
    if (typeof window !== "undefined") {
      this.token = localStorage.getItem("token");
    }
    return this.token;
  }

  clearToken() {
    this.token = null;
    if (typeof window !== "undefined") {
      localStorage.removeItem("token");
    }
  }

  private async request<T>(
    path: string,
    options: RequestInit = {},
  ): Promise<T> {
    const token = this.getToken();
    const headers: Record<string, string> = {
      ...(options.headers as Record<string, string>),
    };

    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    if (!(options.body instanceof FormData)) {
      headers["Content-Type"] = "application/json";
    }

    const response = await fetch(`${API_URL}${path}`, {
      ...options,
      headers,
    });

    if (response.status === 401) {
      this.clearToken();
      if (typeof window !== "undefined") {
        window.location.href = "/login";
      }
      throw new Error("Unauthorized");
    }

    if (!response.ok) {
      const error = await response
        .json()
        .catch(() => ({ error: "Request failed" }));
      throw new ApiError(error.error || "Request failed", response.status);
    }

    return response.json();
  }

  get<T>(path: string): Promise<T> {
    return this.request<T>(path);
  }

  post<T>(path: string, data?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: "POST",
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  put<T>(path: string, data: unknown): Promise<T> {
    return this.request<T>(path, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  patch<T>(path: string, data: unknown): Promise<T> {
    return this.request<T>(path, {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  }

  delete<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: "DELETE" });
  }

  /** Descarga binaria (ej. un PDF ya generado). Devuelve null en 404. */
  async getBlob(path: string): Promise<Blob | null> {
    const token = this.getToken();
    const headers: Record<string, string> = {};
    if (token) headers["Authorization"] = `Bearer ${token}`;

    const response = await fetch(`${API_URL}${path}`, { headers });

    if (response.status === 404) return null;
    if (response.status === 401) {
      this.clearToken();
      if (typeof window !== "undefined") window.location.href = "/login";
      throw new Error("Unauthorized");
    }
    if (!response.ok) {
      throw new Error("Request failed");
    }
    return response.blob();
  }

  upload<T>(path: string, file: File): Promise<T> {
    const formData = new FormData();
    formData.append("file", file);
    return this.request<T>(path, {
      method: "POST",
      body: formData,
    });
  }
}

export const api = new ApiClient();
