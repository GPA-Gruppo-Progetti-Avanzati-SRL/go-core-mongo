package mongo

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/mongo"
	"regexp"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
)

// Query rappresenta i componenti di una query SQL analizzata
type Query struct {
	Collection string                 // Equivalente alla tabella SQL
	Fields     []string               // Campi selezionati (SELECT)
	Conditions map[string]interface{} // Condizioni WHERE
	Limit      int                    // Limite di risultati (LIMIT)
	Offset     int                    // Offset dei risultati (OFFSET)
	Sort       map[string]int         // Ordinamento (ORDER BY)
}

// MongoExecutor contiene il client MongoDB e il database per eseguire le query

// ParseSQL analizza una query SQL e la converte in un oggetto Query
func ParseSQL(sql string) (*Query, error) {
	sql = strings.TrimSpace(sql)
	if !strings.HasPrefix(strings.ToUpper(sql), "SELECT") {
		return nil, errors.New("la query deve iniziare con SELECT")
	}

	// Espressione regolare per parsing di SELECT, FROM, WHERE, ORDER BY, LIMIT e OFFSET
	re := regexp.MustCompile(`(?i)SELECT\s+(.*?)\s+FROM\s+(\w+)(?:\s+WHERE\s+(.*?))?(?:\s+ORDER\s+BY\s+(.*?))?(?:\s+LIMIT\s+(\d+))?(?:\s+OFFSET\s+(\d+))?`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return nil, errors.New("formato SQL non valido")
	}

	query := &Query{
		Fields:     parseFields(matches[1]),
		Collection: matches[2],
		Conditions: make(map[string]interface{}),
		Sort:       make(map[string]int),
	}

	// Parsing delle condizioni WHERE
	if len(matches) > 3 && matches[3] != "" {
		err := parseConditions(matches[3], query)
		if err != nil {
			return nil, err
		}
	}

	// Parsing di ORDER BY
	if len(matches) > 4 && matches[4] != "" {
		err := parseOrderBy(matches[4], query)
		if err != nil {
			return nil, err
		}
	}

	// Parsing di LIMIT
	if len(matches) > 5 && matches[5] != "" {
		limit, err := strconv.Atoi(matches[5])
		if err != nil {
			return nil, errors.New("valore LIMIT non valido")
		}
		query.Limit = limit
	}

	// Parsing di OFFSET
	if len(matches) > 6 && matches[6] != "" {
		offset, err := strconv.Atoi(matches[6])
		if err != nil {
			return nil, errors.New("valore OFFSET non valido")
		}
		query.Offset = offset
	}

	return query, nil
}

// parseFields estrae i campi dal SELECT
func parseFields(fieldsStr string) []string {
	fields := strings.Split(fieldsStr, ",")
	for i, field := range fields {
		fields[i] = strings.TrimSpace(field)
	}
	return fields
}

func parseConditions(whereStr string, query *Query) error {
	conditions := splitConditions(whereStr)
	if len(conditions) > 1 && strings.Contains(strings.ToUpper(whereStr), " OR ") {
		var orConditions []map[string]interface{}
		for _, cond := range conditions {
			condition, err := parseSingleCondition(cond)
			if err != nil {
				return err
			}
			orConditions = append(orConditions, condition)
		}
		query.Conditions["$or"] = orConditions
	} else {
		for _, cond := range conditions {
			condition, err := parseSingleCondition(cond)
			if err != nil {
				return err
			}
			for k, v := range condition {
				query.Conditions[k] = v
			}
		}
	}
	return nil
}

// splitConditions divide la stringa WHERE in base a AND e OR
func splitConditions(whereStr string) []string {
	return regexp.MustCompile(`(?i)\s+(AND|OR)\s+`).Split(whereStr, -1)
}

// OperatorHandler definisce la firma per le funzioni di parsing degli operatori
type OperatorHandler func(cond string) (map[string]interface{}, error)

// operatorMap mappa gli operatori SQL alle loro funzioni di parsing
var operatorMap = map[string]OperatorHandler{
	"IS NULL":     handleIsNull,
	"IS NOT NULL": handleIsNotNull,
	"EXISTS":      handleExists,
	"NOT EXISTS":  handleNotExists,
	"NOT BETWEEN": handleNotBetween,
	"BETWEEN":     handleBetween,
	"NOT IN":      handleNotIn,
	"IN":          handleIn,
	"NOT LIKE":    handleNotLike,
	">=":          handleComparison("$gte"),
	"<=":          handleComparison("$lte"),
	">":           handleComparison("$gt"),
	"<":           handleComparison("$lt"),
	"!=":          handleComparison("$ne"),
	"=":           handleComparison("$eq"),
	"LIKE":        handleLike,
}

// parseSingleCondition analizza una singola condizione e restituisce una mappa
func parseSingleCondition(cond string) (map[string]interface{}, error) {
	cond = strings.TrimSpace(cond)
	for op, handler := range operatorMap {
		if strings.Contains(strings.ToUpper(cond), " "+op) || strings.Contains(cond, op) {
			return handler(cond)
		}
	}
	return nil, errors.New("operatore non supportato nella condizione: " + cond)
}

// Funzioni di parsing per ogni operatore

func handleIsNull(cond string) (map[string]interface{}, error) {
	parts := strings.SplitN(strings.ToUpper(cond), " IS NULL", 2)
	key := strings.TrimSpace(parts[0])
	return map[string]interface{}{key: nil}, nil
}

func handleIsNotNull(cond string) (map[string]interface{}, error) {
	parts := strings.SplitN(strings.ToUpper(cond), " IS NOT NULL", 2)
	key := strings.TrimSpace(parts[0])
	return map[string]interface{}{key: map[string]interface{}{"$ne": nil}}, nil
}

func handleExists(cond string) (map[string]interface{}, error) {
	parts := strings.SplitN(strings.ToUpper(cond), " EXISTS", 2)
	key := strings.TrimSpace(parts[0])
	return map[string]interface{}{key: map[string]interface{}{"$exists": true}}, nil
}

func handleNotExists(cond string) (map[string]interface{}, error) {
	parts := strings.SplitN(strings.ToUpper(cond), " NOT EXISTS", 2)
	key := strings.TrimSpace(parts[0])
	return map[string]interface{}{key: map[string]interface{}{"$exists": false}}, nil
}

func handleNotBetween(cond string) (map[string]interface{}, error) {
	parts := strings.SplitN(strings.ToUpper(cond), " NOT BETWEEN ", 2)
	key := strings.TrimSpace(parts[0])
	betweenParts := strings.SplitN(parts[1], " AND ", 2)
	if len(betweenParts) != 2 {
		return nil, errors.New("formato NOT BETWEEN non valido")
	}
	return map[string]interface{}{
		key: map[string]interface{}{
			"$not": map[string]interface{}{
				"$gte": strings.Trim(betweenParts[0], "'\""),
				"$lte": strings.Trim(betweenParts[1], "'\""),
			},
		},
	}, nil
}

func handleBetween(cond string) (map[string]interface{}, error) {
	parts := strings.SplitN(strings.ToUpper(cond), " BETWEEN ", 2)
	key := strings.TrimSpace(parts[0])
	betweenParts := strings.SplitN(parts[1], " AND ", 2)
	if len(betweenParts) != 2 {
		return nil, errors.New("formato BETWEEN non valido")
	}
	return map[string]interface{}{
		key: map[string]interface{}{
			"$gte": strings.Trim(betweenParts[0], "'\""),
			"$lte": strings.Trim(betweenParts[1], "'\""),
		},
	}, nil
}

func handleNotIn(cond string) (map[string]interface{}, error) {
	parts := strings.SplitN(strings.ToUpper(cond), " NOT IN ", 2)
	key := strings.TrimSpace(parts[0])
	valuesStr := strings.Trim(parts[1], "()")
	values := strings.Split(valuesStr, ",")
	for i, v := range values {
		values[i] = strings.Trim(strings.TrimSpace(v), "'\"")
	}
	return map[string]interface{}{
		key: map[string]interface{}{"$nin": values},
	}, nil
}

func handleIn(cond string) (map[string]interface{}, error) {
	parts := strings.SplitN(strings.ToUpper(cond), " IN ", 2)
	key := strings.TrimSpace(parts[0])
	valuesStr := strings.Trim(parts[1], "()")
	values := strings.Split(valuesStr, ",")
	for i, v := range values {
		values[i] = strings.Trim(strings.TrimSpace(v), "'\"")
	}
	return map[string]interface{}{
		key: values,
	}, nil
}

func handleNotLike(cond string) (map[string]interface{}, error) {
	parts := strings.SplitN(strings.ToUpper(cond), " NOT LIKE", 2)
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, "'\"")
	value = strings.ReplaceAll(value, "%", ".*")
	value = strings.ReplaceAll(value, "_", ".")
	return map[string]interface{}{
		key: map[string]interface{}{
			"$not": map[string]interface{}{
				"$regex": value,
			},
		},
	}, nil
}

func handleLike(cond string) (map[string]interface{}, error) {
	parts := strings.SplitN(strings.ToUpper(cond), "LIKE", 2)
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, "'\"")
	value = strings.ReplaceAll(value, "%", ".*")
	value = strings.ReplaceAll(value, "_", ".")
	return map[string]interface{}{
		key: map[string]interface{}{"$regex": value},
	}, nil
}

func handleComparison(operator string) OperatorHandler {
	return func(cond string) (map[string]interface{}, error) {
		parts := strings.SplitN(cond, operator, 2)
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
		if operator == "$eq" {
			return map[string]interface{}{key: value}, nil
		}
		return map[string]interface{}{
			key: map[string]interface{}{operator: value},
		}, nil
	}
}

// parseOrderBy analizza la clausola ORDER BY
func parseOrderBy(orderByStr string, query *Query) error {
	fields := strings.Split(orderByStr, ",")
	for _, field := range fields {
		field = strings.TrimSpace(field)
		parts := strings.Fields(field) // Divide in campo e direzione
		if len(parts) == 0 {
			return errors.New("formato ORDER BY non valido")
		}
		key := parts[0]
		direction := 1 // Default: ASC
		if len(parts) > 1 {
			switch strings.ToUpper(parts[1]) {
			case "ASC":
				direction = 1
			case "DESC":
				direction = -1
			default:
				return errors.New("direzione ORDER BY non valida: " + parts[1])
			}
		}
		query.Sort[key] = direction
	}
	return nil
}

// ToMongoQuery converte la Query in un formato compatibile con MongoDB
func (q *Query) ToMongoQuery() bson.M {
	mongoQuery := bson.M{}
	if len(q.Conditions) > 0 {
		mongoQuery["$match"] = q.Conditions
	}
	return mongoQuery
}

// ExecuteQuery esegue la query SQL convertita su MongoDB
func (ms *Service) ExecuteQuery(ctx context.Context, sql string) (*mongo.Cursor, error) {
	query, err := ParseSQL(sql)
	if err != nil {
		return nil, err
	}

	// Imposta la collezione corretta
	collection := ms.Database.Collection(query.Collection)

	// Costruisci la pipeline di aggregazione con $match
	pipeline := []bson.M{}

	if len(query.Conditions) > 0 {
		pipeline = append(pipeline, bson.M{"$match": query.Conditions})
	}

	if len(query.Sort) > 0 {
		pipeline = append(pipeline, bson.M{"$sort": query.Sort})
	}

	if query.Offset > 0 {
		pipeline = append(pipeline, bson.M{"$skip": query.Offset})
	}

	if query.Limit > 0 {
		pipeline = append(pipeline, bson.M{"$limit": query.Limit})
	}

	if len(query.Fields) > 0 && query.Fields[0] != "*" {
		project := bson.M{"$project": bson.M{}}
		for _, field := range query.Fields {
			project["$project"].(bson.M)[field] = 1
		}
		pipeline = append(pipeline, project)
	}

	// Esegui la query

	cursor, err := collection.Aggregate(ctx, pipeline)
	return cursor, err
}
