package core

import "gorm.io/gorm"

type Global struct {
	DB        *gorm.DB
	DBPath    string
	Configure *Configure
}

var GLOBAL = new(Global)
