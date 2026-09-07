package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CountryHandler struct {
	countryRepo *repository.CountryRepository
}

func NewCountryHandler(repo *repository.CountryRepository) *CountryHandler {
	return &CountryHandler{countryRepo: repo}
}

func (h *CountryHandler) List(c *fiber.Ctx) error {
	countries, err := h.countryRepo.FindAll(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(countries)
}

func (h *CountryHandler) Active(c *fiber.Ctx) error {
	countries, err := h.countryRepo.FindActive(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(countries)
}

func (h *CountryHandler) Search(c *fiber.Ctx) error {
	q := c.Query("q")
	if q == "" {
		return h.List(c)
	}
	countries, err := h.countryRepo.Search(c.Context(), q)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(countries)
}

func (h *CountryHandler) ByRiskLevel(c *fiber.Ctx) error {
	level := c.Params("level")
	countries, err := h.countryRepo.FindByRiskLevel(c.Context(), level)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(countries)
}

func (h *CountryHandler) Regions(c *fiber.Ctx) error {
	regions, err := h.countryRepo.GetRegions(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(regions)
}

func (h *CountryHandler) ByRegion(c *fiber.Ctx) error {
	region := c.Params("region")
	countries, err := h.countryRepo.FindByRegion(c.Context(), region)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(countries)
}

func (h *CountryHandler) Get(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID invalido"})
	}
	country, err := h.countryRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Pais no encontrado"})
	}
	return c.JSON(country)
}

type createCountryRequest struct {
	Code        string                  `json:"code"`
	Code3       string                  `json:"code3"`
	Name        string                  `json:"name"`
	NameEN      string                  `json:"nameEn"`
	Region      string                  `json:"region"`
	RiskLevel   models.CountryRiskLevel `json:"riskLevel"`
	RiskSources []string                `json:"riskSources"`
	Active      bool                    `json:"active"`
	Notes       string                  `json:"notes"`
}

func (h *CountryHandler) Create(c *fiber.Ctx) error {
	var req createCountryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Datos invalidos"})
	}
	if req.Code == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Codigo y nombre son requeridos"})
	}

	updatedBy := "system"
	if email, ok := c.Locals("email").(string); ok {
		updatedBy = email
	}

	country := &models.Country{
		Code:        req.Code,
		Code3:       req.Code3,
		Name:        req.Name,
		NameEN:      req.NameEN,
		Region:      req.Region,
		RiskLevel:   req.RiskLevel,
		RiskSources: req.RiskSources,
		Active:      req.Active,
		Notes:       req.Notes,
		UpdatedBy:   updatedBy,
	}

	if err := h.countryRepo.Create(c.Context(), country); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(country)
}

type updateCountryRequest struct {
	Name        *string                  `json:"name,omitempty"`
	NameEN      *string                  `json:"nameEn,omitempty"`
	Region      *string                  `json:"region,omitempty"`
	RiskLevel   *models.CountryRiskLevel `json:"riskLevel,omitempty"`
	RiskSources []string                 `json:"riskSources,omitempty"`
	Active      *bool                    `json:"active,omitempty"`
	Notes       *string                  `json:"notes,omitempty"`
}

func (h *CountryHandler) Update(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID invalido"})
	}

	var req updateCountryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Datos invalidos"})
	}

	updatedBy := "system"
	if email, ok := c.Locals("email").(string); ok {
		updatedBy = email
	}

	update := bson.M{"updated_by": updatedBy, "updated_at": time.Now()}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.NameEN != nil {
		update["name_en"] = *req.NameEN
	}
	if req.Region != nil {
		update["region"] = *req.Region
	}
	if req.RiskLevel != nil {
		update["risk_level"] = *req.RiskLevel
	}
	if req.RiskSources != nil {
		update["risk_sources"] = req.RiskSources
	}
	if req.Active != nil {
		update["active"] = *req.Active
	}
	if req.Notes != nil {
		update["notes"] = *req.Notes
	}

	if err := h.countryRepo.Update(c.Context(), id, update); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	country, _ := h.countryRepo.FindByID(c.Context(), id)
	return c.JSON(country)
}

func (h *CountryHandler) Delete(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID invalido"})
	}
	if err := h.countryRepo.Delete(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Pais eliminado"})
}
