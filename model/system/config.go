package system

// Config 配置
type Config struct {
	ID    uint64 `gorm:"primary_key;auto_increment"`
	Group string `gorm:"type:varchar(255);" json:"group"`
	Key   string `gorm:"type:varchar(255);" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}
