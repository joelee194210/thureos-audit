"use client";

import { useEffect, useState } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Users, UserX, UserCheck, Trash2, Plus } from "lucide-react";
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

const emptyNewUser = {
  email: "",
  name: "",
  password: "",
  role: "viewer" as Role,
};

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [newUser, setNewUser] = useState(emptyNewUser);
  const currentUser = useAuthStore((s) => s.user);
  const { toastError, toastSuccess } = useToast();

  useEffect(() => {
    loadUsers();
  }, []);

  async function loadUsers() {
    try {
      setUsers(await usersApi.list());
    } catch {
      toastError("Error al cargar usuarios");
    }
  }

  async function createUser(e: React.FormEvent) {
    e.preventDefault();
    try {
      await usersApi.create(newUser);
      toastSuccess("Usuario creado");
      setIsCreateOpen(false);
      setNewUser(emptyNewUser);
      loadUsers();
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error al crear usuario");
    }
  }

  async function updateRole(id: string, role: Role) {
    try {
      await usersApi.updateRole(id, role);
      loadUsers();
    } catch {
      toastError("Error al actualizar rol");
    }
  }

  async function toggleActive(id: string) {
    try {
      await usersApi.toggleActive(id);
      loadUsers();
    } catch {
      toastError("Error al cambiar estado");
    }
  }

  async function deleteUser(id: string) {
    try {
      await usersApi.delete(id);
      loadUsers();
    } catch {
      toastError("Error al eliminar usuario");
    }
  }

  return (
    <>
      <Header title="Usuarios" />
      <div className="p-6">
        <div className="mb-2 flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Gestiona los usuarios y sus roles
          </p>
          <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="h-4 w-4" /> Nuevo usuario
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Crear usuario</DialogTitle>
              </DialogHeader>
              <form onSubmit={createUser} className="space-y-4">
                <div className="space-y-2">
                  <Label>Nombre</Label>
                  <Input
                    value={newUser.name}
                    onChange={(e) =>
                      setNewUser({ ...newUser, name: e.target.value })
                    }
                    placeholder="Ej: Jorge Neyra"
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label>Correo electrónico</Label>
                  <Input
                    type="email"
                    value={newUser.email}
                    onChange={(e) =>
                      setNewUser({ ...newUser, email: e.target.value })
                    }
                    placeholder="usuario@dominio.com"
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label>Contraseña</Label>
                  <Input
                    type="password"
                    value={newUser.password}
                    onChange={(e) =>
                      setNewUser({ ...newUser, password: e.target.value })
                    }
                    placeholder="Mínimo 8 caracteres, mayúscula, minúscula, número y símbolo"
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label>Rol</Label>
                  <Select
                    value={newUser.role}
                    onValueChange={(v) =>
                      setNewUser({ ...newUser, role: v as Role })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="admin">{roleLabels.admin}</SelectItem>
                      <SelectItem value="compliance">
                        {roleLabels.compliance}
                      </SelectItem>
                      <SelectItem value="viewer">
                        {roleLabels.viewer}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <Button type="submit" className="w-full">
                  Crear
                </Button>
              </form>
            </DialogContent>
          </Dialog>
        </div>

        <p className="mb-6 text-xs text-muted-foreground">
          <strong className="text-foreground">Desactivar</strong> bloquea el
          acceso de forma temporal — se puede reactivar cuando quieras.{" "}
          <strong className="text-foreground">Eliminar</strong> es una baja
          permanente: el usuario no vuelve a aparecer en esta lista ni puede
          iniciar sesión, y no se puede deshacer desde la interfaz.
        </p>

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
                      <p className="text-sm text-muted-foreground">
                        {user.email}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        Registrado: {formatDate(user.createdAt)}
                      </p>
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
                      <SelectTrigger className="w-36">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="admin">
                          {roleLabels.admin}
                        </SelectItem>
                        <SelectItem value="compliance">
                          {roleLabels.compliance}
                        </SelectItem>
                        <SelectItem value="viewer">
                          {roleLabels.viewer}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    {user.id !== currentUser?.id && (
                      <div className="flex gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          title={
                            user.active
                              ? "Desactivar: bloquea el acceso temporalmente, se puede reactivar"
                              : "Activar: restaura el acceso"
                          }
                          onClick={() => toggleActive(user.id)}
                        >
                          {user.active ? (
                            <>
                              <UserX className="h-4 w-4" /> Desactivar
                            </>
                          ) : (
                            <>
                              <UserCheck className="h-4 w-4" /> Activar
                            </>
                          )}
                        </Button>
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button
                              variant="ghost"
                              size="sm"
                              title="Eliminar: baja permanente, no se puede deshacer desde la interfaz"
                              className="text-destructive"
                            >
                              <Trash2 className="h-4 w-4" /> Eliminar
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>
                                ¿Eliminar a {user.name}?
                              </AlertDialogTitle>
                              <AlertDialogDescription>
                                A diferencia de desactivar, esta acción es una
                                baja permanente: el usuario deja de poder
                                iniciar sesión y desaparece de esta lista. El
                                registro se conserva internamente por retención
                                regulatoria, pero no hay forma de revertirlo
                                desde la interfaz.
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>Cancelar</AlertDialogCancel>
                              <AlertDialogAction
                                onClick={() => deleteUser(user.id)}
                              >
                                Eliminar definitivamente
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
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
