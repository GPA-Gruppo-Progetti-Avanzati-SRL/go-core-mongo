package authorization

// Modelli di mapping per la collection unica "acl"

type App struct {
	ID          string `bson:"_id" json:"id"`
	EntityType  string `bson:"_et" json:"et"` // "APP"
	Description string `bson:"description,omitempty" json:"description,omitempty"`
	Path        string `bson:"path,omitempty" json:"path,omitempty"`
	Icon        string `bson:"icon,omitempty" json:"icon,omitempty"`
	Order       int    `bson:"order,omitempty" json:"order,omitempty"`
}

type Role struct {
	ID               string   `bson:"_id" json:"id"`
	EntityType       string   `bson:"_et" json:"et"` // "ROLE"
	Description      string   `bson:"description,omitempty" json:"description,omitempty"`
	CapabilityGroups []string `bson:"capability_groups,omitempty" json:"capability_groups,omitempty"`
	Capabilities     []string `bson:"capabilities,omitempty" json:"capabilities,omitempty"`
}

type CapabilityGroup struct {
	ID           string   `bson:"_id" json:"id"`
	EntityType   string   `bson:"_et" json:"et"` // "CAPABILITYGROUP"
	Description  string   `bson:"description,omitempty" json:"description,omitempty"`
	Capabilities []string `bson:"capabilities,omitempty" json:"capabilities,omitempty"`
}

type ApiNode struct {
	ID          string `bson:"_id" json:"id"`
	EntityType  string `bson:"_et" json:"et"` // "CAPABILITY"
	Description string `bson:"description,omitempty" json:"description,omitempty"`
	Category    string `bson:"category,omitempty" json:"category,omitempty"` // "api"
	OperationID string `bson:"operationid,omitempty" json:"operationid,omitempty"`
}

type UINode struct {
	ID          string `bson:"_id" json:"id"`
	EntityType  string `bson:"_et" json:"et"` // "CAPABILITY"
	Icon        string `bson:"icon,omitempty" json:"icon,omitempty"`
	Description string `bson:"description,omitempty" json:"description,omitempty"`
	Order       int    `bson:"order,omitempty" json:"order,omitempty"`
	Endpoint    string `bson:"endpoint,omitempty" json:"endpoint,omitempty"`
	Category    string `bson:"category,omitempty" json:"category,omitempty"` // "ui"
	IsMenu      bool   `bson:"menu" json:"menu"`
	AppID       string `bson:"appId,omitempty" json:"appId,omitempty"`
}

type ActUi struct {
	ID          string `bson:"_id" json:"id"`
	EntityType  string `bson:"_et" json:"et"` // "CAPABILITY"
	Description string `bson:"description,omitempty" json:"description,omitempty"`
	Category    string `bson:"category,omitempty" json:"category,omitempty"` // "action_ui"
	AppID       string `bson:"appId,omitempty" json:"appId,omitempty"`
}

type ActApi struct {
	ID          string `bson:"_id" json:"id"`
	EntityType  string `bson:"_et" json:"et"` // "CAPABILITY"
	Description string `bson:"description,omitempty" json:"description,omitempty"`
	Category    string `bson:"category,omitempty" json:"category,omitempty"` // "action_api"
}
