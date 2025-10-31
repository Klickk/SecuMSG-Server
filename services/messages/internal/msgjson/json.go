package msgjson

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSON is a lightweight replacement for gorm.io/datatypes.JSON that avoids external
// dependencies while still satisfying the sql.Scanner and driver.Valuer interfaces.
type JSON []byte

// MarshalJSON returns the stored JSON document or null when empty.
func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	if !json.Valid(j) {
		return nil, fmt.Errorf("msgjson.JSON: invalid JSON value")
	}
	return append([]byte(nil), j...), nil
}

// UnmarshalJSON stores the provided JSON payload.
func (j *JSON) UnmarshalJSON(data []byte) error {
	if !json.Valid(data) {
		return fmt.Errorf("msgjson.JSON: invalid JSON payload")
	}
	*j = append((*j)[:0], data...)
	return nil
}

// Value implements driver.Valuer.
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	if !json.Valid(j) {
		return nil, fmt.Errorf("msgjson.JSON: invalid JSON value")
	}
	// Return a copy to avoid exposing internal memory.
	return append([]byte(nil), j...), nil
}

// Scan implements sql.Scanner.
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		if !json.Valid(v) {
			return fmt.Errorf("msgjson.JSON: invalid JSON payload")
		}
		*j = append((*j)[:0], v...)
	case string:
		if !json.Valid([]byte(v)) {
			return fmt.Errorf("msgjson.JSON: invalid JSON payload")
		}
		*j = append((*j)[:0], v...)
	default:
		return fmt.Errorf("msgjson.JSON: unsupported scan type %T", value)
	}
	return nil
}
