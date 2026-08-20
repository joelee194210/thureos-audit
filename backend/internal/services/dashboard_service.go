package services

import (
	"context"
	"fmt"

	"github.com/joelee/datawatch/internal/models"
	"github.com/joelee/datawatch/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type DashboardService struct {
	dashboardRepo *repository.DashboardRepository
	monitorRepo   *repository.MonitorRepository
	ruleRepo      *repository.RuleRepository
}

func NewDashboardService(dashboardRepo *repository.DashboardRepository, monitorRepo *repository.MonitorRepository, ruleRepo *repository.RuleRepository) *DashboardService {
	return &DashboardService{dashboardRepo: dashboardRepo, monitorRepo: monitorRepo, ruleRepo: ruleRepo}
}

type WidgetData struct {
	WidgetID string        `json:"widgetId"`
	Title    string        `json:"title"`
	Type     string        `json:"type"`
	Data     []interface{} `json:"data"`
}

// GetWidgetData fetches aggregated data for a single widget
func (s *DashboardService) GetWidgetData(ctx context.Context, widget models.Widget) (*WidgetData, error) {
	pipeline := buildAggregationPipeline(widget)

	// Resolve monitorId → collectionId
	monitorOID, err := parseObjectID(widget.MonitorID)
	if err != nil {
		return nil, fmt.Errorf("invalid monitor ID: %w", err)
	}
	monitor, err := s.monitorRepo.FindByID(ctx, monitorOID)
	if err != nil {
		return nil, fmt.Errorf("finding monitor: %w", err)
	}

	results, err := s.monitorRepo.AggregateData(ctx, monitor.CollectionID, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregating data: %w", err)
	}

	data := make([]interface{}, len(results))
	for i, r := range results {
		data[i] = r
	}

	return &WidgetData{
		WidgetID: widget.ID,
		Title:    widget.Title,
		Type:     string(widget.Type),
		Data:     data,
	}, nil
}

// GetDashboardData fetches data for all widgets in a dashboard
func (s *DashboardService) GetDashboardData(ctx context.Context, dashboardID string) ([]WidgetData, error) {
	id, err := parseObjectID(dashboardID)
	if err != nil {
		return nil, err
	}

	dashboard, err := s.dashboardRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("finding dashboard: %w", err)
	}

	var widgetData []WidgetData
	for _, widget := range dashboard.Widgets {
		wd, err := s.GetWidgetData(ctx, widget)
		if err != nil {
			continue
		}
		widgetData = append(widgetData, *wd)
	}

	return widgetData, nil
}

func buildAggregationPipeline(widget models.Widget) mongo.Pipeline {
	pipeline := mongo.Pipeline{}

	// Add filters
	if len(widget.Filters) > 0 {
		matchConditions := bson.D{}
		for _, f := range widget.Filters {
			matchConditions = append(matchConditions, bson.E{
				Key:   f.Field,
				Value: conditionToMongo(models.Condition(f)),
			})
		}
		if len(matchConditions) > 0 {
			pipeline = append(pipeline, bson.D{{Key: "$match", Value: matchConditions}})
		}
	}

	// Build aggregation based on type
	if widget.GroupBy != "" {
		groupStage := bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$" + widget.GroupBy},
				{Key: "value", Value: buildAggExpression(widget.Aggregation, widget.Field)},
			}},
		}
		pipeline = append(pipeline, groupStage)

		pipeline = append(pipeline, bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "name", Value: "$_id"},
				{Key: "value", Value: 1},
				{Key: "_id", Value: 0},
			}},
		})

		pipeline = append(pipeline, bson.D{
			{Key: "$sort", Value: bson.D{{Key: "value", Value: -1}}},
		})

		pipeline = append(pipeline, bson.D{
			{Key: "$limit", Value: 20},
		})
	} else {
		groupStage := bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: nil},
				{Key: "value", Value: buildAggExpression(widget.Aggregation, widget.Field)},
			}},
		}
		pipeline = append(pipeline, groupStage)
	}

	return pipeline
}

// DrillDown returns paginated records matching a widget's groupBy value
type DrillDownResult struct {
	Records []bson.M `json:"records"`
	Total   int64    `json:"total"`
	Page    int      `json:"page"`
}

func (s *DashboardService) DrillDown(ctx context.Context, dashboardID, widgetID, groupValue string, page, pageSize int, search string, export bool) (*DrillDownResult, error) {
	dashOID, err := parseObjectID(dashboardID)
	if err != nil {
		return nil, fmt.Errorf("invalid dashboard ID: %w", err)
	}

	dashboard, err := s.dashboardRepo.FindByID(ctx, dashOID)
	if err != nil {
		return nil, fmt.Errorf("finding dashboard: %w", err)
	}

	// Find the widget
	var widget *models.Widget
	for i := range dashboard.Widgets {
		if dashboard.Widgets[i].ID == widgetID {
			widget = &dashboard.Widgets[i]
			break
		}
	}
	if widget == nil {
		return nil, fmt.Errorf("widget not found")
	}

	// Resolve monitor → collection
	monitorOID, err := parseObjectID(widget.MonitorID)
	if err != nil {
		return nil, fmt.Errorf("invalid monitor ID: %w", err)
	}
	monitor, err := s.monitorRepo.FindByID(ctx, monitorOID)
	if err != nil {
		return nil, fmt.Errorf("finding monitor: %w", err)
	}

	// Build filter: widget filters + groupBy match
	filter := bson.M{}

	// Apply widget's existing filters
	for _, f := range widget.Filters {
		filter[f.Field] = conditionToMongo(models.Condition(f))
	}

	// Apply groupBy = groupValue filter (empty string matches records with no value)
	if widget.GroupBy != "" {
		filter[widget.GroupBy] = groupValue
	}

	// Apply search across string fields
	if search != "" {
		var orConditions []bson.M
		for _, field := range monitor.Schema {
			if field.Type == "string" {
				orConditions = append(orConditions, bson.M{
					field.Name: bson.M{"$regex": search, "$options": "i"},
				})
			}
		}
		if len(orConditions) > 0 {
			filter["$or"] = orConditions
		}
	}

	if page < 1 {
		page = 1
	}
	if export {
		if pageSize < 1 || pageSize > 10000 {
			pageSize = 10000
		}
	} else {
		if pageSize < 1 || pageSize > 100 {
			pageSize = 50
		}
	}

	records, total, err := s.monitorRepo.QueryDataPaginated(ctx, monitor.CollectionID, filter, int64(page), int64(pageSize))
	if err != nil {
		return nil, fmt.Errorf("querying data: %w", err)
	}

	return &DrillDownResult{
		Records: records,
		Total:   total,
		Page:    page,
	}, nil
}

// ConfigureWidgetFromRule auto-configures a widget based on a rule's conditions
func (s *DashboardService) ConfigureWidgetFromRule(ctx context.Context, ruleID string, widget *models.Widget) error {
	ruleOID, err := primitive.ObjectIDFromHex(ruleID)
	if err != nil {
		return fmt.Errorf("invalid rule ID: %w", err)
	}

	rule, err := s.ruleRepo.FindByID(ctx, ruleOID)
	if err != nil {
		return fmt.Errorf("finding rule: %w", err)
	}

	// Set monitor from rule
	widget.MonitorID = rule.MonitorID.Hex()
	widget.RuleID = ruleID

	// If rule has aggregate conditions, use the first one to configure the widget
	if len(rule.AggregateConditions) > 0 {
		agg := rule.AggregateConditions[0]
		widget.Field = agg.Field
		widget.GroupBy = agg.GroupBy

		// Map rule's AggFunction to widget's AggregationType
		switch agg.Function {
		case models.AggFuncSum:
			widget.Aggregation = models.AggSum
		case models.AggFuncCount:
			widget.Aggregation = models.AggCount
		case models.AggFuncAvg:
			widget.Aggregation = models.AggAvg
		case models.AggFuncMin:
			widget.Aggregation = models.AggMin
		case models.AggFuncMax:
			widget.Aggregation = models.AggMax
		default:
			widget.Aggregation = models.AggCount
		}
	} else {
		// Use condition group to create filters
		widget.Aggregation = models.AggCount
		if len(rule.ConditionGroup.Conditions) > 0 {
			widget.Filters = rule.ConditionGroup.Conditions
			// Use the first condition's field for display
			widget.Field = rule.ConditionGroup.Conditions[0].Field
		}
	}

	// Default title from rule name if empty
	if widget.Title == "" {
		widget.Title = "Regla: " + rule.Name
	}

	return nil
}

func buildAggExpression(agg models.AggregationType, field string) bson.D {
	switch agg {
	case models.AggCount:
		return bson.D{{Key: "$sum", Value: 1}}
	case models.AggSum:
		return bson.D{{Key: "$sum", Value: "$" + field}}
	case models.AggAvg:
		return bson.D{{Key: "$avg", Value: "$" + field}}
	case models.AggMin:
		return bson.D{{Key: "$min", Value: "$" + field}}
	case models.AggMax:
		return bson.D{{Key: "$max", Value: "$" + field}}
	default:
		return bson.D{{Key: "$sum", Value: 1}}
	}
}
