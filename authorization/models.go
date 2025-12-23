package authorization

// Modelli di mapping per la collection unica "acl"

type Role struct {
	ID             string   `bson:"_id" json:"id"`
	Type           string   `bson:"type" json:"type"` // "role"
	Description    string   `bson:"description,omitempty" json:"description,omitempty"`
	FunctionGroups []string `bson:"functiongroups,omitempty" json:"functiongroups,omitempty"`
}

type FunctionGroup struct {
	ID          string   `bson:"_id" json:"id"`
	Type        string   `bson:"type" json:"type"` // "functiongroup"
	Description string   `bson:"description,omitempty" json:"description,omitempty"`
	Functions   []string `bson:"functions,omitempty" json:"functions,omitempty"`
}

// Nodi dedicati per ciascun tipo di function
type EndpointNode struct {
	OperationID string `bson:"operationid,omitempty" json:"operationid,omitempty"`
}

type MenuNode struct {
	Icon             string `bson:"icon,omitempty" json:"icon,omitempty"`
	Order            int    `bson:"order,omitempty" json:"order,omitempty"`
	IsLeaf           bool   `bson:"isleaf,omitempty" json:"isleaf,omitempty"`
	Endpoint         string `bson:"endpoint,omitempty" json:"endpoint,omitempty"`
	FunctionParentID string `bson:"functionparentid,omitempty" json:"functionparentid,omitempty"`
	AppID            string `bson:"appid,omitempty" json:"appid,omitempty"`
}

type CapabilityNode struct {
	CapType string `bson:"captype,omitempty" json:"captype,omitempty"` // "client" | "server"
	AppID   string `bson:"appid,omitempty" json:"appid,omitempty"`
}

type Function struct {
	ID          string `bson:"_id" json:"id"`
	Type        string `bson:"type" json:"type"` // "function"
	Description string `bson:"description,omitempty" json:"description,omitempty"`

	Endpoint   *EndpointNode   `bson:"endpoint,omitempty" json:"endpoint,omitempty"`
	Menu       *MenuNode       `bson:"menu,omitempty" json:"menu,omitempty"`
	Capability *CapabilityNode `bson:"capability,omitempty" json:"capability,omitempty"`
}
