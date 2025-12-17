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

type Function struct {
	ID          string `bson:"_id" json:"id"`
	Type        string `bson:"type" json:"type"` // "function"
	Description string `bson:"description,omitempty" json:"description,omitempty"`
}
