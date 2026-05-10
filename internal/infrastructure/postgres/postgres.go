package postgres

import (
	"log"

	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	conn *gorm.DB
}

func NewDB(dsn string) (*DB, error) {
	conn, err := gorm.Open(gormPostgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	
	sqlDB, err := conn.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)

	log.Println("POSTGRES: connected successfully")

	db := &DB{conn: conn}

	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) GetConn() *gorm.DB {
	return db.conn
}

func (db *DB) Close() error {
	log.Println("POSTGRES: closing connection")
	sqlDB, err := db.conn.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (db *DB) migrate() error {
	err := db.conn.AutoMigrate(
		&OrderModel{},
		&TradeModel{},
		&UserModel{},
	)
	if err != nil {
		return err
	}
	log.Println("POSTGRES: auto-migration completed successfully")
	return nil
}
