# Thureos · Design Tokens v1.0

Fuente única de verdad de color, tipografía, espaciado, forma y movimiento para
**Thureos Compliance** y **Thureos Fraud & Behavior Analytics**.

## Archivos

| Archivo | Para qué sirve |
|---|---|
| `thureos-tokens.css` | Variables CSS. Temas (`data-theme`) y marcas (`data-brand`). Es el archivo obligatorio. |
| `thureos-tokens.json` | Fuente neutra para Figma, Style Dictionary o cualquier stack no-web. |
| `tailwind.preset.js` | Preset de Tailwind: colores, tipografía, radios, sombras, motion. |
| `shadcn-theme.css` | Variables HSL de shadcn/ui, por tema y por marca. |

## Instalación (Next.js + Tailwind + shadcn/ui)

```tsx
// app/layout.tsx
<html lang="es" data-theme="dark" data-brand="compliance">
```

```js
// tailwind.config.js
module.exports = { presets: [require('./tokens/tailwind.preset.js')] }
```

```css
/* app/globals.css — en este orden */
@import '../tokens/thureos-tokens.css';
@import '../tokens/shadcn-theme.css';
@tailwind base; @tailwind components; @tailwind utilities;
```

## Uso

Ningún componente escribe un color literal. Siempre el token semántico:

```tsx
<button className="bg-accent text-accent-on hover:bg-accent-hover rounded-md">
<div className="bg-surface border border-line text-ink">
<span className="text-ink-muted text-body-sm">
```

Cambiar de producto es cambiar un atributo:

```tsx
data-brand="compliance"   →   data-brand="fraud"
```

## Reglas

1. El acento de producto **no** comunica severidad. El riesgo usa `--risk-*`.
2. Las series de gráfico usan `--chart-1..8`, nunca el acento.
3. Todo par texto/fondo del sistema está verificado en WCAG 2.1 AA. Un cambio que
   rompa un umbral no se aprueba.
4. El arte del logotipo no se modifica, recolorea ni redibuja.
