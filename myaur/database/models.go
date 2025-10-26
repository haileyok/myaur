package database

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	return json.Marshal(s)
}

func (s *StringSlice) Scan(value any) error {
	if value == nil {
		*s = []string{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to unmarshal StringSlice value: %v (type: %T)", value, value)
	}

	return json.Unmarshal(bytes, s)
}

type PackageInfo struct {
	Id             int64       `gorm:"primaryKey;autoIncrement" json:"ID"`
	Name           string      `gorm:"uniqueIndex;not null" json:"Name"`
	PackageBaseID  int64       `json:"PackageBaseID"`
	PackageBase    string      `gorm:"index" json:"PackageBase"`
	Version        string      `json:"Version"`
	Description    string      `gorm:"index:idx_description" json:"Description"`
	Url            string      `json:"URL"`
	NumVotes       int64       `json:"NumVotes"`
	Popularity     float64     `json:"Popularity"`
	OutOfDate      *int64      `json:"OutOfDate"`
	Maintainer     string      `gorm:"index" json:"Maintainer"`
	FirstSubmitted int64       `json:"FirstSubmitted"`
	LastModified   int64       `json:"LastModified"`
	UrlPath        string      `json:"URLPath"`
	Depends        StringSlice `gorm:"type:text" json:"Depends"`
	MakeDepends    StringSlice `gorm:"type:text" json:"MakeDepends"`
	License        StringSlice `gorm:"type:text" json:"License"`
	Keywords       StringSlice `gorm:"type:text" json:"Keywords"`
}

// set the tablename so gorm doesn't mess it up
func (PackageInfo) TableName() string {
	return "package_info"
}
