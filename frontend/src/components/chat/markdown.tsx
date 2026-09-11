"use client";

import { Fragment, type ReactNode } from "react";
import { lexMarkdown, safeHref, type Token } from "@/lib/markdown-tokens";

/**
 * Renderiza markdown como elementos de React.
 *
 * INVARIANTE: no hay `dangerouslySetInnerHTML` en este archivo ni debe
 * agregarse. Todo lo que no se sabe representar cae en el branch
 * `default` de `renderToken`, que lo muestra como texto plano. Un token
 * que no se reconoce se ve feo, nunca se ejecuta.
 */
export function Markdown({ children }: { children: string }) {
  return <div className="space-y-2">{renderTokens(lexMarkdown(children))}</div>;
}

function renderTokens(tokens: Token[]): ReactNode {
  return tokens.map((token, i) => (
    <Fragment key={i}>{renderToken(token)}</Fragment>
  ));
}

/** Los tokens inline traen `tokens` hijos; si no, se cae al texto crudo. */
function renderInline(token: { tokens?: Token[]; text?: string }): ReactNode {
  if (token.tokens && token.tokens.length > 0) return renderTokens(token.tokens);
  return token.text ?? "";
}

function renderToken(token: Token): ReactNode {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const t = token as any;

  switch (token.type) {
    case "paragraph":
      return <p className="leading-relaxed">{renderInline(t)}</p>;

    case "text":
      // Un token de texto puede traer hijos inline (dentro de un listitem
      // o una celda de tabla) o ser una hoja.
      return renderInline(t);

    case "strong":
      return <strong className="font-semibold">{renderInline(t)}</strong>;

    case "em":
      return <em className="italic">{renderInline(t)}</em>;

    case "del":
      return <del className="line-through">{renderInline(t)}</del>;

    case "escape":
      // "\*", "\_", etc: marked ya resolvió el caracter escapado en
      // `text` (sin la barra invertida). Mostrar `t.raw` en su lugar
      // dejaría la barra invertida visible.
      return t.text ?? "";

    case "codespan":
      return (
        <code className="rounded bg-muted px-1 py-0.5 font-mono text-[0.85em]">
          {t.text}
        </code>
      );

    case "code":
      return (
        <pre className="overflow-x-auto rounded-md bg-muted p-3">
          <code className="font-mono text-xs">{t.text}</code>
        </pre>
      );

    case "heading": {
      const level = Math.min(Math.max(t.depth ?? 1, 1), 6);
      const Tag = `h${level}` as "h1";
      return (
        <Tag className="font-semibold" style={{ fontSize: `${1.3 - level * 0.06}em` }}>
          {renderInline(t)}
        </Tag>
      );
    }

    case "list": {
      const rawItems = (t.items ?? []) as Array<{
        tokens?: Token[];
        text?: string;
        task?: boolean;
        checked?: boolean;
      }>;
      // Una checklist ("- [ ] x" / "- [x] x") es una lista cuyos items
      // traen `task: true`. Si se ignora `checked`, pendiente y hecho se
      // ven idénticos, que es justo lo que una checklist no puede hacer.
      const hasTasks = rawItems.some((item) => item.task);
      const items = rawItems.map((item, i) => (
        <li key={i} className="ml-4 list-outside">
          {item.task ? (
            <input
              type="checkbox"
              checked={!!item.checked}
              disabled
              readOnly
              className="mr-2 align-middle"
            />
          ) : null}
          {renderInline(item)}
        </li>
      ));
      return t.ordered ? (
        <ol className="list-decimal space-y-1">{items}</ol>
      ) : (
        <ul className={hasTasks ? "list-none space-y-1" : "list-disc space-y-1"}>
          {items}
        </ul>
      );
    }

    case "blockquote":
      return (
        <blockquote className="border-l-2 pl-3 text-muted-foreground">
          {renderTokens(t.tokens ?? [])}
        </blockquote>
      );

    case "table":
      return (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b">
                {(t.header ?? []).map((cell: Token, i: number) => (
                  <th key={i} className="p-2 text-left font-medium">
                    {renderInline(cell as { tokens?: Token[]; text?: string })}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {(t.rows ?? []).map((row: Token[], r: number) => (
                <tr key={r} className="border-b last:border-0">
                  {row.map((cell, c) => (
                    <td key={c} className="p-2">
                      {renderInline(cell as { tokens?: Token[]; text?: string })}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );

    case "link": {
      const href = safeHref(t.href ?? "");
      // Sin href seguro se muestra el texto, nunca el enlace. El
      // contenido no se pierde: lo que se pierde es la navegación.
      if (!href) return <>{renderInline(t)}</>;
      return (
        <a
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          className="underline underline-offset-2"
        >
          {renderInline(t)}
        </a>
      );
    }

    case "hr":
      return <hr className="border-border" />;

    case "br":
      return <br />;

    case "space":
      return null;

    case "image":
      // Decisión deliberada, no un olvido: NO se renderiza <img>. Un
      // <img src="..."> dispara una petición GET a esa URL apenas se
      // pinta el DOM, sin que el usuario haga click en nada. El texto del
      // asistente está influido por archivos que suben los usuarios, así
      // que esa URL puede venir de una inyección de prompt -y convertirse
      // en un vector de rastreo o de filtración de datos (por query
      // string) hacia un servidor de terceros-. Se muestra el markdown
      // crudo como texto, igual que cualquier tipo no reconocido.
      return <span className="whitespace-pre-wrap">{t.raw ?? t.text ?? ""}</span>;

    case "def":
      // Definición de enlace/imagen por referencia ("[foo]: url \"title\"").
      // marked ya la usa para resolver el link o la imagen que la
      // referencia; la línea de la declaración en sí no produce salida
      // visible en markdown estándar, así que tampoco acá.
      return null;

    // "html" y cualquier tipo que no conozcamos: SIEMPRE como texto.
    // Este default es la red de seguridad del componente.
    default:
      return <span className="whitespace-pre-wrap">{t.raw ?? t.text ?? ""}</span>;
  }
}
