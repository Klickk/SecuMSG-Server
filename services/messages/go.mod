module messages

go 1.25

require (
        cryptocore v0.0.0
        github.com/google/uuid v1.6.0
        gorm.io/driver/postgres v1.6.0
        gorm.io/gorm v1.31.0
)

replace cryptocore => ../crypto-core

