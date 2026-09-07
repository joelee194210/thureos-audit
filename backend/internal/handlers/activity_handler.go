package handlers

import (
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ActivityHandler struct {
	activityRepo *repository.ActivityLogRepository
}

func NewActivityHandler(activityRepo *repository.ActivityLogRepository) *ActivityHandler {
	return &ActivityHandler{activityRepo: activityRepo}
}

// validActions is the allowlist of known activity types for query filtering.
var validActions = map[string]bool{
	string(models.ActivityLogin):         true,
	string(models.ActivityRedFlagAction): true,
	string(models.ActivityRuleCreate):    true,
	string(models.ActivityRuleUpdate):    true,
	string(models.ActivityRuleDelete):    true,
	string(models.ActivityRuleExecute):   true,
	string(models.ActivityUpload):        true,
	string(models.ActivityUserManage):    true,
	string(models.ActivityMCCUpdate):     true,
}

func (h *ActivityHandler) List(c *fiber.Ctx) error {
	limitStr := c.Query("limit", "100")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit < 1 || limit > 500 {
		limit = 100
	}

	filter := bson.M{}

	if userID := c.Query("userId"); userID != "" {
		oid, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid userId"})
		}
		filter["user_id"] = oid
	}

	if action := c.Query("action"); action != "" {
		if !validActions[action] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid action type"})
		}
		filter["action"] = action
	}

	logs, err := h.activityRepo.FindFiltered(c.Context(), filter, limit)
	if err != nil {
		log.Printf("ERROR listing activity logs: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error listing activity logs"})
	}

	return c.JSON(logs)
}
