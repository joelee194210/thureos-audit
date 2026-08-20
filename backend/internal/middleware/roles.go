package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/joelee/datawatch/internal/models"
)

func RequireRole(roles ...models.Role) fiber.Handler {
	return func(c *fiber.Ctx) error {
		roleStr, ok := c.Locals("role").(string)
		if !ok || roleStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}
		userRole := models.Role(roleStr)

		for _, r := range roles {
			if userRole == r {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient permissions",
		})
	}
}

func RequireAdmin() fiber.Handler {
	return RequireRole(models.RoleAdmin)
}

func RequireComplianceOrAbove() fiber.Handler {
	return RequireRole(models.RoleAdmin, models.RoleCompliance)
}
