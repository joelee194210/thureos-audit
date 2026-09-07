package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type MCCNetwork string

const (
	NetworkVisa       MCCNetwork = "visa"
	NetworkMastercard MCCNetwork = "mastercard"
	NetworkUnionPay   MCCNetwork = "unionpay"
	NetworkAmex       MCCNetwork = "amex"
	NetworkDiscover   MCCNetwork = "discover"
	NetworkDiners     MCCNetwork = "diners"
	NetworkJCB        MCCNetwork = "jcb"
	NetworkAll        MCCNetwork = "all"
)

type MCC struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Code        string             `bson:"code" json:"code"`
	Description string             `bson:"description" json:"description"`
	Category    string             `bson:"category" json:"category"`
	Networks    []MCCNetwork       `bson:"networks" json:"networks"`
	RiskLevel   string             `bson:"risk_level" json:"riskLevel"` // low, medium, high
}
