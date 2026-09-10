#!/usr/bin/env bash
#
# Despliegue de Thureos Compliance al VPS de producción.
#
# Existe porque este rsync y este build tienen dos trampas que ya se pisaron,
# y ninguna avisa:
#
#   1. `--delete` sin excluir `.env.production` borra el archivo de secretos
#      del servidor. Está en .gitignore, así que no existe en el origen y
#      rsync lo considera sobrante. La app sigue corriendo con los
#      contenedores viejos, así que no se nota hasta el siguiente build, que
#      falla con "couldn't find env file".
#
#   2. `docker compose build` sin `--env-file .env.production` no resuelve
#      ${DOMAIN}, y el frontend queda compilado contra `https:///api/v1`.
#      El build termina bien; la app queda muerta en el navegador.
#
# Uso:  deploy/desplegar.sh [servicio ...]      (por defecto: backend frontend)

set -euo pipefail

VPS="${THUREOS_VPS:-root@169.58.242.38}"
REMOTO="${THUREOS_VPS_PATH:-/opt/thureos-audit/app}"
RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

SERVICIOS=("$@")
if [ ${#SERVICIOS[@]} -eq 0 ]; then
  SERVICIOS=(backend frontend)
fi

COMPOSE=(docker compose --env-file .env.production -f docker-compose.prod.yml -p thureos-compliance)

echo "==> Sincronizando $RAIZ -> $VPS:$REMOTO"
rsync -az --delete \
  --exclude '.git' \
  --exclude '.env.production' \
  --exclude 'node_modules' \
  --exclude '.next' \
  --exclude 'backend/bin' \
  --exclude '.worktrees' \
  --exclude '.claude' \
  --exclude '.superpowers' \
  "$RAIZ/" "$VPS:$REMOTO/"

echo "==> Verificando que el archivo de entorno siga en su lugar"
ssh "$VPS" "test -s $REMOTO/.env.production" \
  || { echo "FALTA $REMOTO/.env.production — no se puede construir"; exit 1; }

echo "==> Construyendo: ${SERVICIOS[*]}"
ssh "$VPS" "cd $REMOTO && ${COMPOSE[*]} build ${SERVICIOS[*]}"

echo "==> Levantando: ${SERVICIOS[*]}"
ssh "$VPS" "cd $REMOTO && ${COMPOSE[*]} up -d ${SERVICIOS[*]}"

echo "==> Estado"
ssh "$VPS" "cd $REMOTO && docker compose -p thureos-compliance ps --format '{{.Name}}\t{{.Status}}'"

DOMINIO="$(ssh "$VPS" "grep '^DOMAIN=' $REMOTO/.env.production | cut -d= -f2")"
echo "==> Humo sobre https://$DOMINIO"
curl -s -m 15 -o /dev/null -w "   portada: %{http_code}\n" "https://$DOMINIO/"
curl -s -m 15 -o /dev/null -w "   api:     %{http_code}  (401 es lo esperado: pide auth)\n" \
  "https://$DOMINIO/api/v1/health"
