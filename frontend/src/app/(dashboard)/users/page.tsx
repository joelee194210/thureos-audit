"use client";

import { useEffect, useState } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Users, UserX, UserCheck, Trash2 } from "lucide-react";
import { usersApi } from "@/lib/api/users";
import { useToast } from "@/lib/use-toast";
import { formatDate } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth-store";
import type { User, Role } from "@/lib/types";

const roleLabels: Record<Role, string> = {
  admin: "Administrador",
  compliance: "Cumplimiento",
  viewer: "Visualizador",
};

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const currentUser = useAuthStore((s) => s.user);
  const { toastError } = useToast();

  useEffect(() => {
    loadUsers();
  }, []);

  async function loadUsers() {
    try { setUsers(await usersApi.list()); } catch { toastError("Error al cargar usuarios"); }
  }

  async function updateRole(id: string, role: Role) {
    try { await usersApi.updateRole(id, role); loadUsers(); } catch { toastError("Error al actualizar rol"); }
  }

  async function toggleActive(id: string) {
    try { await usersApi.toggleActive(id); loadUsers(); } catch { toastError("Error al cambiar estado"); }
  }

  async function deleteUser(id: string) {
    try { await usersApi.delete(id); loadUsers(); } catch { toastError("Error al eliminar usuario"); }
  }

  return (
    <>
      <Header title="Usuarios" />
      <div className="p-6">
        <div className="mb-6">
          <p className="text-sm text-muted-foreground">
            Gestiona los usuarios y sus roles
          </p>
        </div>

        {users.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Users className="mb-4 h-12 w-12 text-muted-foreground" />
              <p className="font-medium">No hay usuarios</p>
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-3">
            {users.map((user) => (
              <Card key={user.id}>
                <CardContent className="flex items-center justify-between p-4">
                  <div className="flex items-center gap-4">
                    <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted text-sm font-medium">
                      {user.name.charAt(0).toUpperCase()}
                    </div>
                    <div>
                      <p className="font-medium">{user.name}</p>
                      <p className="text-sm text-muted-foreground">{user.email}</p>
                      <p className="text-xs text-muted-foreground">Registrado: {formatDate(user.createdAt)}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <Badge variant={user.active ? "success" : "destructive"}>
                      {user.active ? "Activo" : "Inactivo"}
                    </Badge>
                    <Select
                      value={user.role}
                      onValueChange={(v) => updateRole(user.id, v as Role)}
                      disabled={user.id === currentUser?.id}
                    >
                      <SelectTrigger className="w-36"><SelectValue /></SelectTrigger>
                      <SelectContent>
                        <SelectItem value="admin">{roleLabels.admin}</SelectItem>
                        <SelectItem value="compliance">{roleLabels.compliance}</SelectItem>
                        <SelectItem value="viewer">{roleLabels.viewer}</SelectItem>
                      </SelectContent>
                    </Select>
                    {user.id !== currentUser?.id && (
                      <div className="flex gap-1">
                        <Button variant="ghost" size="icon" onClick={() => toggleActive(user.id)}>
                          {user.active ? <UserX className="h-4 w-4" /> : <UserCheck className="h-4 w-4" />}
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => deleteUser(user.id)}>
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>
    </>
  );
}
