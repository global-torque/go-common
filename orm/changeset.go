package orm

import "maps"

// ChangeSet tracks changed database column values.
type ChangeSet struct {
	values map[string]any
}

// Set stores or overwrites a changed column value.
func (c *ChangeSet) Set(column string, value any) {
	if c.values == nil {
		c.values = map[string]any{}
	}

	c.values[column] = value
}

// Changes returns a defensive copy of tracked changes.
func (c *ChangeSet) Changes() map[string]any {
	if len(c.values) == 0 {
		return map[string]any{}
	}

	res := make(map[string]any, len(c.values))
	maps.Copy(res, c.values)

	return res
}

// ResetChanges clears tracked changes.
func (c *ChangeSet) ResetChanges() {
	c.values = map[string]any{}
}
