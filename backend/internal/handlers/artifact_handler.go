package handlers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/thureos/compliance/internal/services"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
		// Una instantánea no se re-ejecuta, pero sí se puede exportar con
		// los datos que ya tiene guardados. ErrArtifactSourceUnavailable
		// NO entra en este respaldo a propósito: significa que el
		// artefacto SÍ tiene fuentes pero una ya no resuelve a un
		// monitor, y exportar en silencio una caché que puede estar
		// desactualizada por eso sería peor que devolver el error —el
		// usuario pide un archivo, no una pantalla con un aviso al lado.
		if errors.Is(err, services.ErrArtifactNotRerunnable) && len(art.CachedData) > 0 {
			run = services.ArtifactRun{Data: art.CachedData, RanAt: art.RanAt}
		} else {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	data, err := services.BuildArtifactXLSX(art, run)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	// El título lo escribió el LLM: fuera comillas y saltos de línea antes
	// de meterlo en una cabecera HTTP, para que no rompa ni inyecte en
	// Content-Disposition.
	safeTitle := strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(art.Title)
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, safeTitle))
	return c.Send(data)
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
