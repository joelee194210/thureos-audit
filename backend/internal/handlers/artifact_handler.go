package handlers

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/text/unicode/norm"
)

type ArtifactHandler struct {
	artifactRepo    *repository.ChatArtifactRepository
	artifactService *services.ArtifactService
}

func NewArtifactHandler(artifactRepo *repository.ChatArtifactRepository, artifactService *services.ArtifactService) *ArtifactHandler {
	return &ArtifactHandler{artifactRepo: artifactRepo, artifactService: artifactService}
}

// load resuelve el artefacto del path y verifica propiedad. Devuelve
// siempre 404 cuando no es del usuario: nunca se distingue "no existe" de
// "no es tuyo" (ver Global Constraints).
//
// El tercer valor es un OK, no un error, y es importante que así sea:
// fiber.Ctx.JSON() devuelve nil cuando serializa bien — escribe la
// respuesta, no produce un error. Devolver su resultado como "el error"
// daría nil en todos los caminos de fallo, el llamador seguiría de largo
// y desreferenciaría un artefacto nil. Cuando ok es false la respuesta de
// error YA fue escrita: el handler solo tiene que devolver nil.
func (h *ArtifactHandler) load(c *fiber.Ctx) (*models.ChatArtifact, primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
		return nil, primitive.NilObjectID, false
	}
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		_ = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
		return nil, primitive.NilObjectID, false
	}
	art, err := h.artifactRepo.GetByID(c.Context(), id)
	if err != nil || art.UserID != userID {
		_ = c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "artefacto no encontrado"})
		return nil, primitive.NilObjectID, false
	}
	return art, userID, true
}

func (h *ArtifactHandler) Save(c *fiber.Ctx) error {
	art, _, ok := h.load(c)
	if !ok {
		return nil // load ya escribió la respuesta de error
	}
	// Se parsea a un struct propio con el ÚNICO campo que este endpoint
	// legitima — el nombre — y nunca a models.ChatArtifact: ese tipo trae
	// json tags para id, userId, monitorIds, saved, savedName, cachedData
	// y ranAt, y un body parseado directo ahí dejaría que cualquier
	// llamador se autoasignara "saved": true o su propia allowlist de
	// monitorIds. El mismo agujero que ParseArtifact cerró del lado del
	// modelo, pero abierto del lado del usuario. No lo "simplifiques" a
	// c.BodyParser(art).
	var body struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&body); err != nil || body.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name es obligatorio"})
	}
	if err := h.artifactRepo.SetSaved(c.Context(), art.ID, true, body.Name); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	art.Saved, art.SavedName = true, body.Name
	return c.JSON(art)
}

func (h *ArtifactHandler) Unsave(c *fiber.Ctx) error {
	art, _, ok := h.load(c)
	if !ok {
		return nil // load ya escribió la respuesta de error
	}
	if err := h.artifactRepo.SetSaved(c.Context(), art.ID, false, ""); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "artefacto quitado de la biblioteca"})
}

func (h *ArtifactHandler) List(c *fiber.Ctx) error {
	userID, err := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "usuario inválido"})
	}
	arts, err := h.artifactRepo.ListSavedByUser(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(arts)
}

// Run re-ejecuta el artefacto y persiste la caché. Es el ÚNICO endpoint
// que escribe cached_data/ran_at: exportar re-ejecuta pero no muta.
func (h *ArtifactHandler) Run(c *fiber.Ctx) error {
	art, userID, ok := h.load(c)
	if !ok {
		return nil // load ya escribió la respuesta de error
	}
	run, err := h.artifactService.RunArtifact(c.Context(), art, userID)
	if err != nil {
		if errors.Is(err, services.ErrArtifactNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "artefacto no encontrado"})
		}
		if errors.Is(err, services.ErrArtifactNotRerunnable) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if errors.Is(err, services.ErrArtifactSourceUnavailable) {
			// Modo degradado del spec: alguna fuente ya no resuelve a un
			// monitor. No es un fallo transitorio del servidor —es un
			// estado permanente del artefacto—, así que se responde con
			// un código propio (409, no 500) y el texto del centinela
			// para que el cliente distinga "tu caché quedó vieja porque
			// la fuente desapareció" de un error de conexión y pinte la
			// caché que ya tiene con el motivo a la vista. No se llega a
			// SetCache: no hay dato fresco que guardar.
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	// La corrida ya tuvo éxito y los datos frescos viajan en la respuesta
	// de todos modos: si cachear falla (blip de Mongo) no hay que tirar
	// la petición por eso, solo se pierde la persistencia de ESTA corrida.
	_ = h.artifactRepo.SetCache(c.Context(), art.ID, run.Data, run.RanAt)
	// series viaja al frontend porque el pivote renombra las columnas:
	// sin ella, un gráfico multi-fuente no sabría qué claves dibujar.
	return c.JSON(fiber.Map{"data": run.Data, "series": run.Series, "ranAt": run.RanAt})
}

func (h *ArtifactHandler) ExportXLSX(c *fiber.Ctx) error {
	art, userID, ok := h.load(c)
	if !ok {
		return nil // load ya escribió la respuesta de error
	}
	run, err := h.artifactService.RunArtifact(c.Context(), art, userID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrArtifactNotRerunnable) && len(art.CachedData) > 0:
			// Una instantánea no se re-ejecuta, pero sí se puede exportar
			// con los datos que ya tiene guardados.
			run = services.ArtifactRun{Data: art.CachedData, RanAt: art.RanAt}
		case errors.Is(err, services.ErrArtifactSourceUnavailable):
			// Mismo negocio que en Run, mismo código (409, no 500): una
			// fuente que ya no resuelve a un monitor es un estado
			// permanente del artefacto, no un fallo transitorio del
			// servidor, y el cliente debe poder distinguirlos igual acá
			// que en /run. NO cae al respaldo de caché de la rama de
			// arriba a propósito: ese respaldo es solo para instantáneas
			// sin sources, y acá el artefacto SÍ tiene fuentes y una
			// desapareció — exportar en silencio una caché que puede
			// estar desactualizada sería peor que devolver el error, ya
			// que una descarga no tiene dónde mostrar el aviso.
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	data, err := services.BuildArtifactXLSX(art, run)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", xlsxContentDisposition(art.Title))
	return c.Send(data)
}

// sanitizeXLSXFilename saca las comillas dobles del título antes de
// interpolarlo en Content-Disposition: fasthttp NO las toca, y una sin
// escapar rompe el atributo filename="...". El CR/LF también se
// descarta acá, aunque es cinturón y tirantes — fasthttp (c.Set) ya los
// elimina de cualquier valor de cabecera antes de escribirlo.
func sanitizeXLSXFilename(title string) string {
	return strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(title)
}

// asciiXLSXFilename arma el respaldo ASCII de filename= para clientes
// viejos que no leen filename*= (RFC 5987/6266). El título es texto
// libre en español —tildes, eñes y ¿¡ son lo esperable en casi
// cualquier artefacto real, no el caso raro— así que no alcanza con
// descartar los bytes no ASCII sin más: primero se separan los acentos
// de la letra (NFD) y se descartan solo las marcas combinantes, mismo
// criterio que stripDiacritics en services/chat_monitors.go, para que
// "Región" caiga en "Region" y no en "Regin". Si no queda ni un
// carácter reconocible (un título que sea solo símbolos o CJK), se usa
// un nombre genérico en vez de un archivo sin nombre.
func asciiXLSXFilename(title string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(title) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if r <= unicode.MaxASCII {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		out = "artefacto"
	}
	return out
}

// rfc5987AttrChar son los bytes que RFC 5987 permite sin percent-encode
// en un ext-value. Todo lo demás —incluido cualquier byte de una
// secuencia UTF-8 multibyte, que siempre cae fuera de este conjunto— se
// codifica byte a byte, que es exactamente cómo pide la RFC el charset
// UTF-8 con comillas simples vacías (ver xlsxContentDisposition).
func rfc5987AttrChar(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '!', '#', '$', '&', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}
	return false
}

// percentEncodeRFC5987 codifica s para el ext-value de un filename*=,
// byte a byte sobre su representación UTF-8 (no rune a rune: un acento
// es varios bytes y todos necesitan su propio %XX).
func percentEncodeRFC5987(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if rfc5987AttrChar(c) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// xlsxContentDisposition arma el header completo de la descarga. Manda
// las dos formas que define RFC 6266: filename= en ASCII de respaldo
// (para el cliente que no entienda filename*=) y un filename*= con el
// prefijo UTF-8 de comillas simples vacías y el título completo, tildes
// y ñ incluidas, percent-encoded — así un título como
// `Análisis "Región" Caribe` no degrada a mojibake en el navegador que
// sí lo lee, que es el caso común de un producto en español, no el borde.
func xlsxContentDisposition(title string) string {
	safe := sanitizeXLSXFilename(title)
	ascii := asciiXLSXFilename(safe)
	encoded := percentEncodeRFC5987(safe + ".xlsx")
	return fmt.Sprintf(`attachment; filename="%s.xlsx"; filename*=UTF-8''%s`, ascii, encoded)
}

func (h *ArtifactHandler) Delete(c *fiber.Ctx) error {
	art, _, ok := h.load(c)
	if !ok {
		return nil // load ya escribió la respuesta de error
	}
	if err := h.artifactRepo.Delete(c.Context(), art.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "artefacto borrado"})
}
